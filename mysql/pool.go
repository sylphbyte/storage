// Package mysql 提供MySQL数据库连接池管理功能
package mysql

import (
	"database/sql"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
	"app/pkg/storage/pool"
	"gorm.io/gorm"
)

// PoolConfig MySQL连接池配置
type PoolConfig struct {
	// 最大空闲连接数
	MaxIdleConns int `yaml:"max_idle_conns" json:"max_idle_conns"`

	// 最大打开连接数
	MaxOpenConns int `yaml:"max_open_conns" json:"max_open_conns"`

	// 连接最大生命周期(秒)
	ConnMaxLifetime int `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`

	// 连接最大空闲时间(秒)
	ConnMaxIdleTime int `yaml:"conn_max_idle_time" json:"conn_max_idle_time"`

	// 是否启用连接池统计
	EnableStats bool `yaml:"enable_stats" json:"enable_stats"`
}

// Validate 验证配置
func (c *PoolConfig) Validate() error {
	if c.MaxIdleConns < 0 {
		return errors.New("max_idle_conns不能为负数")
	}

	if c.MaxOpenConns <= 0 {
		return errors.New("max_open_conns必须大于0")
	}

	if c.MaxIdleConns > c.MaxOpenConns {
		return errors.New("max_idle_conns不能大于max_open_conns")
	}

	return nil
}

// IsStatsEnabled 是否启用统计
func (c *PoolConfig) IsStatsEnabled() bool {
	return c.EnableStats
}

// DefaultPoolConfig 返回默认MySQL连接池配置
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MaxIdleConns:    10,
		MaxOpenConns:    100,
		ConnMaxLifetime: 3600, // 1小时
		ConnMaxIdleTime: 600,  // 10分钟
		EnableStats:     true,
	}
}

// Pool MySQL连接池管理器
type Pool struct {
	db        *gorm.DB
	sqlDB     *sql.DB
	config    *PoolConfig
	stats     *pool.BasePoolStats
	ticker    *time.Ticker
	stopCh    chan struct{}
	mu        sync.RWMutex
	isRunning bool
}

// NewPool 创建新的MySQL连接池管理器
func NewPool(db *gorm.DB, config *PoolConfig) (*Pool, error) {
	if db == nil {
		return nil, errors.New("db不能为nil")
	}

	if config == nil {
		config = DefaultPoolConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "MySQL连接池配置无效")
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, errors.Wrap(err, "获取SQL DB失败")
	}

	// 配置连接池
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(config.ConnMaxLifetime) * time.Second)
	sqlDB.SetConnMaxIdleTime(time.Duration(config.ConnMaxIdleTime) * time.Second)

	p := &Pool{
		db:        db,
		sqlDB:     sqlDB,
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

	return p.sqlDB.Close()
}

// Resize 调整连接池大小
func (p *Pool) Resize(maxIdle, maxActive int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 验证参数
	if maxIdle < 0 {
		return errors.New("maxIdle不能为负数")
	}

	if maxActive <= 0 {
		return errors.New("maxActive必须大于0")
	}

	if maxIdle > maxActive {
		return errors.New("maxIdle不能大于maxActive")
	}

	// 更新配置
	p.config.MaxIdleConns = maxIdle
	p.config.MaxOpenConns = maxActive

	// 调整连接池
	p.sqlDB.SetMaxIdleConns(maxIdle)
	p.sqlDB.SetMaxOpenConns(maxActive)

	pr.Info("MySQL连接池大小已调整: maxIdle=%d, maxActive=%d", maxIdle, maxActive)
	return nil
}

// ClearIdle 清除空闲连接
func (p *Pool) ClearIdle() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 获取当前空闲连接数
	stats := p.sqlDB.Stats()
	idleCount := stats.Idle

	// 通过将MaxIdleConns临时设置为0来清除所有空闲连接
	originalMaxIdle := p.config.MaxIdleConns
	p.sqlDB.SetMaxIdleConns(0)

	// 然后恢复原来的设置
	p.sqlDB.SetMaxIdleConns(originalMaxIdle)

	pr.Info("MySQL连接池空闲连接已清除: %d", idleCount)
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
	stats := p.sqlDB.Stats()

	p.stats.UpdateConnections(
		stats.InUse, // 活跃连接
		stats.Idle,  // 空闲连接
	)

	// 更新等待相关的统计
	p.stats.UpdateStats(func(ps *pool.PoolStats) {
		ps.WaitCount = stats.WaitCount
		ps.WaitDuration = stats.WaitDuration.Milliseconds()
	})
}

// SetMaxLifetime 设置连接最大生命周期
func (p *Pool) SetMaxLifetime(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config.ConnMaxLifetime = int(d.Seconds())
	p.sqlDB.SetConnMaxLifetime(d)
}

// SetMaxIdleTime 设置连接最大空闲时间
func (p *Pool) SetMaxIdleTime(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config.ConnMaxIdleTime = int(d.Seconds())
	p.sqlDB.SetConnMaxIdleTime(d)
}

// GetConfig 获取连接池配置的副本
func (p *Pool) GetConfig() *PoolConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return &PoolConfig{
		MaxIdleConns:    p.config.MaxIdleConns,
		MaxOpenConns:    p.config.MaxOpenConns,
		ConnMaxLifetime: p.config.ConnMaxLifetime,
		ConnMaxIdleTime: p.config.ConnMaxIdleTime,
		EnableStats:     p.config.EnableStats,
	}
}

// RecordQueryStats 记录查询统计信息
func (p *Pool) RecordQueryStats(duration time.Duration, err error) {
	if !p.config.IsStatsEnabled() {
		return
	}

	p.stats.RecordWait(duration, false, err)
}
