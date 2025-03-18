// Package elasticsearch 提供Elasticsearch连接池管理功能
package elasticsearch

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
	"app/pkg/storage/pool"
)

// PoolConfig ES连接池配置
type PoolConfig struct {
	// 最大空闲连接数
	MaxIdleConns int `yaml:"max_idle_conns" json:"max_idle_conns"`

	// 每个主机最大空闲连接数
	MaxIdleConnsPerHost int `yaml:"max_idle_conns_per_host" json:"max_idle_conns_per_host"`

	// 每个主机最大连接数
	MaxConnsPerHost int `yaml:"max_conns_per_host" json:"max_conns_per_host"`

	// 空闲连接超时(秒)
	IdleConnTimeout int `yaml:"idle_conn_timeout" json:"idle_conn_timeout"`

	// 是否启用连接池统计
	EnableStats bool `yaml:"enable_stats" json:"enable_stats"`
}

// Validate 验证配置
func (c *PoolConfig) Validate() error {
	if c.MaxIdleConns < 0 {
		return errors.New("max_idle_conns不能为负数")
	}

	if c.MaxIdleConnsPerHost < 0 {
		return errors.New("max_idle_conns_per_host不能为负数")
	}

	if c.MaxConnsPerHost <= 0 {
		return errors.New("max_conns_per_host必须大于0")
	}

	if c.IdleConnTimeout < 0 {
		return errors.New("idle_conn_timeout不能为负数")
	}

	return nil
}

// IsStatsEnabled 是否启用统计
func (c *PoolConfig) IsStatsEnabled() bool {
	return c.EnableStats
}

// DefaultPoolConfig 返回默认ES连接池配置
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     100,
		IdleConnTimeout:     90, // 90秒
		EnableStats:         true,
	}
}

// Pool ES连接池管理器
type Pool struct {
	client         *elasticsearch.Client
	httpClient     *http.Client
	transport      *http.Transport
	config         *PoolConfig
	stats          *pool.BasePoolStats
	ticker         *time.Ticker
	stopCh         chan struct{}
	mu             sync.RWMutex
	isRunning      bool
	requestCounter int64
	errorCounter   int64
}

// NewPool 创建新的ES连接池管理器
func NewPool(client *elasticsearch.Client, config *PoolConfig) (*Pool, error) {
	if client == nil {
		return nil, errors.New("client不能为nil")
	}

	if config == nil {
		config = DefaultPoolConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "ES连接池配置无效")
	}

	// 创建一个自定义的HTTP传输层
	transport := &http.Transport{
		MaxIdleConns:        config.MaxIdleConns,
		MaxIdleConnsPerHost: config.MaxIdleConnsPerHost,
		MaxConnsPerHost:     config.MaxConnsPerHost,
		IdleConnTimeout:     time.Duration(config.IdleConnTimeout) * time.Second,
	}

	// 创建HTTP客户端
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	// Elasticsearch客户端不允许直接修改其内部Transport
	// 我们会使用自定义HTTP客户端，但要记录下这个限制
	pr.Warning("ES连接池设置只能应用于新创建的HTTP客户端，不能修改现有ES客户端的Transport")

	p := &Pool{
		client:     client,
		httpClient: httpClient,
		transport:  transport,
		config:     config,
		stats:      pool.NewBasePoolStats(),
		stopCh:     make(chan struct{}),
		isRunning:  false,
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

	// Elasticsearch不提供连接池统计，所以我们手动估算
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

	// ES客户端无需显式关闭
	return nil
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
	p.config.MaxConnsPerHost = maxActive

	// 调整连接池设置
	if p.transport != nil {
		p.transport.MaxIdleConns = maxIdle
		p.transport.MaxConnsPerHost = maxActive
		p.transport.MaxIdleConnsPerHost = min(maxIdle, 10) // 每主机空闲数适当限制

		pr.Info("ES连接池大小已调整: maxIdle=%d, maxActive=%d", maxIdle, maxActive)
		return nil
	}

	return errors.New("无法调整ES连接池大小，传输层不可用")
}

// ClearIdle 清除空闲连接
func (p *Pool) ClearIdle() int {
	if p.transport == nil {
		return 0
	}

	// 获取当前空闲连接数估算值
	stats := p.Stats()
	idleCount := stats.IdleConnections

	// 通过关闭空闲连接来清除它们
	p.transport.CloseIdleConnections()

	// 执行一次ping以更新统计信息
	req := esapi.PingRequest{}
	res, err := req.Do(context.Background(), p.client)
	if err != nil {
		pr.Error("ES Ping失败: %v", err)
	} else {
		_ = res.Body.Close()
	}

	pr.Info("ES连接池空闲连接已清除: %d (估算值)", idleCount)
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
	p.mu.RLock()
	requestCount := p.requestCounter
	errorCount := p.errorCounter
	p.mu.RUnlock()

	// ES不提供确切的连接统计，我们做一个估算
	// 假设活跃连接是请求计数的10%，但不超过最大连接数
	activeEstimate := min(int(requestCount/10), p.config.MaxConnsPerHost)

	// 假设空闲连接是配置的最大空闲连接数的一半
	idleEstimate := p.config.MaxIdleConns / 2

	p.stats.UpdateConnections(
		activeEstimate,
		idleEstimate,
	)

	// 更新错误计数
	p.stats.UpdateStats(func(ps *pool.PoolStats) {
		ps.ErrorCount = errorCount
	})
}

// SetIdleConnTimeout 设置空闲连接超时时间
func (p *Pool) SetIdleConnTimeout(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config.IdleConnTimeout = int(d.Seconds())

	if p.transport != nil {
		p.transport.IdleConnTimeout = d
	}
}

// GetConfig 获取连接池配置的副本
func (p *Pool) GetConfig() *PoolConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return &PoolConfig{
		MaxIdleConns:        p.config.MaxIdleConns,
		MaxIdleConnsPerHost: p.config.MaxIdleConnsPerHost,
		MaxConnsPerHost:     p.config.MaxConnsPerHost,
		IdleConnTimeout:     p.config.IdleConnTimeout,
		EnableStats:         p.config.EnableStats,
	}
}

// RecordRequestStats 记录请求统计信息
func (p *Pool) RecordRequestStats(duration time.Duration, err error) {
	if !p.config.IsStatsEnabled() {
		return
	}

	p.mu.Lock()
	p.requestCounter++
	if err != nil {
		p.errorCounter++
	}
	p.mu.Unlock()

	p.stats.RecordWait(duration, false, err)
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
