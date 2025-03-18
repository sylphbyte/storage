// Package reconnect 提供自动重连功能的实现
package reconnect

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
)

// Reconnector 提供自动重连功能的接口
type Reconnector interface {
	// IsConnected 检查当前是否已连接
	IsConnected() bool

	// Connect 建立连接
	Connect(ctx context.Context) error

	// Reconnect 重新建立连接
	Reconnect(ctx context.Context) error

	// SetReconnectPolicy 设置重连策略
	SetReconnectPolicy(policy ReconnectPolicy) error

	// GetReconnectStats 获取重连统计信息
	GetReconnectStats() ReconnectStats
}

// ReconnectPolicy 重连策略
type ReconnectPolicy struct {
	// MaxRetries 最大重试次数，-1表示无限重试
	MaxRetries int `yaml:"max_retries" json:"max_retries"`

	// InitialInterval 初始重试间隔（毫秒）
	InitialInterval int `yaml:"initial_interval" json:"initial_interval"`

	// MaxInterval 最大重试间隔（毫秒）
	MaxInterval int `yaml:"max_interval" json:"max_interval"`

	// Multiplier 间隔递增倍数（指数退避）
	Multiplier float64 `yaml:"multiplier" json:"multiplier"`

	// RandomizationFactor 随机化因子（0-1之间）
	RandomizationFactor float64 `yaml:"randomization_factor" json:"randomization_factor"`
}

// DefaultReconnectPolicy 返回默认重连策略
func DefaultReconnectPolicy() ReconnectPolicy {
	return ReconnectPolicy{
		MaxRetries:          5,
		InitialInterval:     100,   // 100ms
		MaxInterval:         30000, // 30s
		Multiplier:          1.5,
		RandomizationFactor: 0.5,
	}
}

// Validate 验证重连策略
func (p *ReconnectPolicy) Validate() error {
	if p.InitialInterval <= 0 {
		return errors.New("初始间隔必须大于0")
	}

	if p.MaxInterval <= 0 {
		return errors.New("最大间隔必须大于0")
	}

	if p.Multiplier <= 0 {
		return errors.New("倍数必须大于0")
	}

	if p.RandomizationFactor < 0 || p.RandomizationFactor > 1 {
		return errors.New("随机化因子必须在0到1之间")
	}

	return nil
}

// ReconnectStats 重连统计信息
type ReconnectStats struct {
	// 重连尝试次数
	ReconnectAttempts int64 `json:"reconnect_attempts"`

	// 重连成功次数
	ReconnectSuccesses int64 `json:"reconnect_successes"`

	// 重连失败次数
	ReconnectFailures int64 `json:"reconnect_failures"`

	// 上次重连时间
	LastReconnectTime time.Time `json:"last_reconnect_time"`

	// 上次重连状态（成功/失败）
	LastReconnectStatus bool `json:"last_reconnect_status"`

	// 上次重连错误信息
	LastReconnectError string `json:"last_reconnect_error"`
}

// Copy 创建统计信息的副本
func (s *ReconnectStats) Copy() ReconnectStats {
	return ReconnectStats{
		ReconnectAttempts:   atomic.LoadInt64(&s.ReconnectAttempts),
		ReconnectSuccesses:  atomic.LoadInt64(&s.ReconnectSuccesses),
		ReconnectFailures:   atomic.LoadInt64(&s.ReconnectFailures),
		LastReconnectTime:   s.LastReconnectTime,
		LastReconnectStatus: s.LastReconnectStatus,
		LastReconnectError:  s.LastReconnectError,
	}
}

// BaseReconnector 基本重连器实现
type BaseReconnector struct {
	mu              sync.RWMutex
	connected       bool
	reconnecting    atomic.Bool
	policy          ReconnectPolicy
	stats           ReconnectStats
	connectFunc     func(ctx context.Context) error
	isConnectedFunc func() bool
	closeFunc       func() error
}

// NewBaseReconnector 创建基本重连器
func NewBaseReconnector(
	connectFunc func(ctx context.Context) error,
	isConnectedFunc func() bool,
	closeFunc func() error,
	policy *ReconnectPolicy,
) (*BaseReconnector, error) {
	if connectFunc == nil {
		return nil, errors.New("connectFunc不能为nil")
	}

	if isConnectedFunc == nil {
		return nil, errors.New("isConnectedFunc不能为nil")
	}

	if closeFunc == nil {
		return nil, errors.New("closeFunc不能为nil")
	}

	p := DefaultReconnectPolicy()
	if policy != nil {
		p = *policy
	}

	if err := p.Validate(); err != nil {
		return nil, errors.Wrap(err, "无效的重连策略")
	}

	return &BaseReconnector{
		connected:       false,
		policy:          p,
		connectFunc:     connectFunc,
		isConnectedFunc: isConnectedFunc,
		closeFunc:       closeFunc,
	}, nil
}

// IsConnected 检查当前是否已连接
func (r *BaseReconnector) IsConnected() bool {
	r.mu.RLock()
	connected := r.connected
	r.mu.RUnlock()

	if !connected {
		return false
	}

	// 双重检查，调用实际的连接检查函数
	return r.isConnectedFunc()
}

// Connect 建立连接
func (r *BaseReconnector) Connect(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.connected && r.isConnectedFunc() {
		return nil // 已连接
	}

	if err := r.connectFunc(ctx); err != nil {
		return errors.Wrap(err, "连接失败")
	}

	r.connected = true
	return nil
}

// Reconnect 重新建立连接
func (r *BaseReconnector) Reconnect(ctx context.Context) error {
	// 如果已经有重连操作在进行中，直接返回
	if r.reconnecting.Load() {
		return errors.New("重连操作已在进行中")
	}

	if r.reconnecting.CompareAndSwap(false, true) {
		defer r.reconnecting.Store(false)
	} else {
		return errors.New("无法启动重连操作")
	}

	// 记录重连尝试
	atomic.AddInt64(&r.stats.ReconnectAttempts, 1)
	r.stats.LastReconnectTime = time.Now()

	// 关闭现有连接
	r.mu.Lock()
	r.connected = false
	closeErr := r.closeFunc()
	if closeErr != nil {
		pr.Warning("关闭现有连接时出错: %v", closeErr)
	}
	r.mu.Unlock()

	// 使用退避策略进行重连
	var lastErr error
	maxRetries := r.policy.MaxRetries
	unlimitedRetries := maxRetries < 0

	for attempt := 0; unlimitedRetries || attempt <= maxRetries; attempt++ {
		// 检查上下文是否已取消
		if ctx.Err() != nil {
			r.recordReconnectFailure(ctx.Err())
			return errors.Wrap(ctx.Err(), "重连被取消")
		}

		// 如果不是第一次尝试，则等待
		if attempt > 0 {
			backoff := r.calculateBackoff(attempt - 1)
			timer := time.NewTimer(backoff)
			select {
			case <-timer.C:
				// 继续尝试
			case <-ctx.Done():
				timer.Stop()
				r.recordReconnectFailure(ctx.Err())
				return errors.Wrap(ctx.Err(), "重连被取消")
			}
		}

		// 尝试连接
		r.mu.Lock()
		err := r.connectFunc(ctx)
		if err == nil {
			r.connected = true
			r.mu.Unlock()
			r.recordReconnectSuccess()
			return nil
		}
		r.mu.Unlock()

		lastErr = err
		pr.Warning("重连尝试 %d 失败: %v", attempt+1, err)
	}

	// 所有重试都失败了
	r.recordReconnectFailure(lastErr)
	return errors.Wrap(lastErr, "重连失败，已达最大重试次数")
}

// SetReconnectPolicy 设置重连策略
func (r *BaseReconnector) SetReconnectPolicy(policy ReconnectPolicy) error {
	if err := policy.Validate(); err != nil {
		return errors.Wrap(err, "无效的重连策略")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.policy = policy
	return nil
}

// GetReconnectStats 获取重连统计信息
func (r *BaseReconnector) GetReconnectStats() ReconnectStats {
	return r.stats.Copy()
}

// 计算退避时间
func (r *BaseReconnector) calculateBackoff(attempt int) time.Duration {
	r.mu.RLock()
	policy := r.policy
	r.mu.RUnlock()

	// 计算基础间隔（指数增长）
	interval := float64(policy.InitialInterval) * math.Pow(policy.Multiplier, float64(attempt))

	// 添加随机因子
	interval = interval * (1 + policy.RandomizationFactor*(rand.Float64()*2-1))

	// 不超过最大间隔
	if interval > float64(policy.MaxInterval) {
		interval = float64(policy.MaxInterval)
	}

	return time.Duration(interval) * time.Millisecond
}

// 记录重连成功
func (r *BaseReconnector) recordReconnectSuccess() {
	atomic.AddInt64(&r.stats.ReconnectSuccesses, 1)
	r.stats.LastReconnectStatus = true
	r.stats.LastReconnectError = ""
	pr.Info("重连成功")
}

// 记录重连失败
func (r *BaseReconnector) recordReconnectFailure(err error) {
	atomic.AddInt64(&r.stats.ReconnectFailures, 1)
	r.stats.LastReconnectStatus = false
	if err != nil {
		r.stats.LastReconnectError = err.Error()
	} else {
		r.stats.LastReconnectError = "未知错误"
	}
	pr.Error("重连失败: %v", err)
}
