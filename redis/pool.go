// Package redis 提供Redis连接池管理功能
package redis

import (
	"context"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
	"app/pkg/storage/pool"
)

// PoolConfig Redis连接池配置
type PoolConfig struct {
	// 连接池大小
	PoolSize int `yaml:"pool_size" json:"pool_size"`

	// 最小空闲连接数
	MinIdleConns int `yaml:"min_idle_conns" json:"min_idle_conns"`

	// 连接最大生命周期(秒)
	MaxConnAge int `yaml:"max_conn_age" json:"max_conn_age"`

	// 从池获取连接超时时间(秒)
	PoolTimeout int `yaml:"pool_timeout" json:"pool_timeout"`

	// 空闲连接超时时间(秒)
	IdleTimeout int `yaml:"idle_timeout" json:"idle_timeout"`

	// 空闲连接检查频率(秒)
	IdleCheckFrequency int `yaml:"idle_check_frequency" json:"idle_check_frequency"`

	// 是否启用连接池统计
	EnableStats bool `yaml:"enable_stats" json:"enable_stats"`
}

// Validate 验证配置
func (c *PoolConfig) Validate() error {
	if c.PoolSize <= 0 {
		return errors.New("pool_size必须大于0")
	}

	if c.MinIdleConns < 0 {
		return errors.New("min_idle_conns不能为负数")
	}

	if c.MinIdleConns > c.PoolSize {
		return errors.New("min_idle_conns不能大于pool_size")
	}

	if c.PoolTimeout < 0 {
		return errors.New("pool_timeout不能为负数")
	}

	return nil
}

// IsStatsEnabled 是否启用统计
func (c *PoolConfig) IsStatsEnabled() bool {
	return c.EnableStats
}

// DefaultPoolConfig 返回默认Redis连接池配置
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		PoolSize:           50,
		MinIdleConns:       10,
		MaxConnAge:         3600, // 1小时
		PoolTimeout:        30,   // 30秒
		IdleTimeout:        300,  // 5分钟
		IdleCheckFrequency: 60,   // 1分钟
		EnableStats:        true,
	}
}

// Pool Redis连接池管理器
type Pool struct {
	client    *redis.Client
	config    *PoolConfig
	stats     *pool.BasePoolStats
	ticker    *time.Ticker
	stopCh    chan struct{}
	mu        sync.RWMutex
	isRunning bool
}

// NewPool 创建新的Redis连接池管理器
func NewPool(client *redis.Client, config *PoolConfig) (*Pool, error) {
	if client == nil {
		return nil, errors.New("client不能为nil")
	}

	if config == nil {
		config = DefaultPoolConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "Redis连接池配置无效")
	}

	// 应用配置到Redis客户端
	client.Options().PoolSize = config.PoolSize
	client.Options().MinIdleConns = config.MinIdleConns
	client.Options().MaxConnAge = time.Duration(config.MaxConnAge) * time.Second
	client.Options().PoolTimeout = time.Duration(config.PoolTimeout) * time.Second
	client.Options().IdleTimeout = time.Duration(config.IdleTimeout) * time.Second
	client.Options().IdleCheckFrequency = time.Duration(config.IdleCheckFrequency) * time.Second

	p := &Pool{
		client:    client,
		config:    config,
		stats:     pool.NewBasePoolStats(),
		stopCh:    make(chan struct{}),
		isRunning: false,
	}

	// 如果启用了统计功能，启动定时统计任务
	if config.IsStatsEnabled() {
		p.startStatsCollector()
	}

	return p, nil
}

// Stats 获取连接池统计信息
func (p *Pool) Stats() *pool.PoolStats {
	if p.stats == nil || !p.config.IsStatsEnabled() {
		return &pool.PoolStats{}
	}

	// 更新最新的连接信息
	p.updateConnectionStats()

	return p.stats.GetStats()
}

// Close 关闭连接池
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isRunning {
		close(p.stopCh)
		p.isRunning = false
	}

	if p.ticker != nil {
		p.ticker.Stop()
		p.ticker = nil
	}

	return p.client.Close()
}

// Resize 调整连接池大小
func (p *Pool) Resize(minIdle, maxActive int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 验证参数
	if minIdle < 0 {
		return errors.New("minIdle不能为负数")
	}

	if maxActive <= 0 {
		return errors.New("maxActive必须大于0")
	}

	if minIdle > maxActive {
		return errors.New("minIdle不能大于maxActive")
	}

	// 需要创建新的client来应用更改
	oldOptions := p.client.Options()
	newClient := redis.NewClient(&redis.Options{
		Addr:               oldOptions.Addr,
		Username:           oldOptions.Username,
		Password:           oldOptions.Password,
		DB:                 oldOptions.DB,
		MaxRetries:         oldOptions.MaxRetries,
		MinRetryBackoff:    oldOptions.MinRetryBackoff,
		MaxRetryBackoff:    oldOptions.MaxRetryBackoff,
		DialTimeout:        oldOptions.DialTimeout,
		ReadTimeout:        oldOptions.ReadTimeout,
		WriteTimeout:       oldOptions.WriteTimeout,
		PoolSize:           maxActive,
		MinIdleConns:       minIdle,
		MaxConnAge:         time.Duration(p.config.MaxConnAge) * time.Second,
		PoolTimeout:        time.Duration(p.config.PoolTimeout) * time.Second,
		IdleTimeout:        time.Duration(p.config.IdleTimeout) * time.Second,
		IdleCheckFrequency: time.Duration(p.config.IdleCheckFrequency) * time.Second,
		TLSConfig:          oldOptions.TLSConfig,
	})

	// 关闭旧客户端
	err := p.client.Close()
	if err != nil {
		pr.Error("关闭旧Redis客户端失败: %v", err)
	}

	// 更新配置和客户端
	p.config.PoolSize = maxActive
	p.config.MinIdleConns = minIdle
	p.client = newClient

	pr.Info("Redis连接池大小已调整: minIdle=%d, poolSize=%d", minIdle, maxActive)
	return nil
}

// ClearIdle 清除空闲连接
func (p *Pool) ClearIdle() int {
	// 由于Redis客户端没有直接的方法来清除空闲连接，
	// 我们通过发出PING命令来检查连接，并通过Redis的IdleCheckFrequency机制让它自行清理空闲连接
	ctx := context.Background()
	_, err := p.client.Ping(ctx).Result()
	if err != nil {
		pr.Error("Redis Ping失败: %v", err)
	}

	// 无法获取确切的清除连接数，返回估算值
	stats := p.Stats()
	idleCount := stats.IdleConnections

	// 更新一下连接池统计数据
	p.updateConnectionStats()

	// 返回之前的空闲连接数作为估算的清除连接数
	return idleCount
}

// 启动统计信息收集器
func (p *Pool) startStatsCollector() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isRunning {
		return
	}

	p.ticker = time.NewTicker(30 * time.Second)
	p.isRunning = true

	go func() {
		for {
			select {
			case <-p.ticker.C:
				p.updateConnectionStats()
			case <-p.stopCh:
				return
			}
		}
	}()
}

// 更新连接统计信息
func (p *Pool) updateConnectionStats() {
	// Redis客户端不提供直接的连接池统计信息，使用PoolStats来获取
	poolStats := p.client.PoolStats()

	p.stats.UpdateConnections(
		int(poolStats.TotalConns-poolStats.IdleConns), // 活跃连接 = 总连接 - 空闲连接
		int(poolStats.IdleConns),                      // 空闲连接
	)

	// 更新其他统计数据
	p.stats.UpdateStats(func(ps *pool.PoolStats) {
		ps.TimeoutCount = int64(poolStats.Timeouts)
		// 没有等待计数，我们使用统计的请求数作为近似值
		ps.WaitCount = int64(poolStats.Hits + poolStats.Misses)
	})
}

// SetMaxConnAge 设置连接最大生命周期
func (p *Pool) SetMaxConnAge(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config.MaxConnAge = int(d.Seconds())
	p.client.Options().MaxConnAge = d
}

// SetIdleTimeout 设置连接最大空闲时间
func (p *Pool) SetIdleTimeout(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config.IdleTimeout = int(d.Seconds())
	p.client.Options().IdleTimeout = d
}

// SetPoolTimeout 设置从池获取连接的超时时间
func (p *Pool) SetPoolTimeout(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config.PoolTimeout = int(d.Seconds())
	p.client.Options().PoolTimeout = d
}

// GetConfig 获取连接池配置的副本
func (p *Pool) GetConfig() *PoolConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return &PoolConfig{
		PoolSize:           p.config.PoolSize,
		MinIdleConns:       p.config.MinIdleConns,
		MaxConnAge:         p.config.MaxConnAge,
		PoolTimeout:        p.config.PoolTimeout,
		IdleTimeout:        p.config.IdleTimeout,
		IdleCheckFrequency: p.config.IdleCheckFrequency,
		EnableStats:        p.config.EnableStats,
	}
}

// RecordCommandStats 记录命令执行统计信息
func (p *Pool) RecordCommandStats(duration time.Duration, err error) {
	if !p.config.IsStatsEnabled() {
		return
	}

	p.stats.RecordWait(duration, duration > time.Duration(p.config.PoolTimeout)*time.Second, err)
}
