// Package pool 提供连接池管理相关的接口和实现
package pool

import (
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
)

// PoolStats 连接池统计信息
type PoolStats struct {
	// 当前活跃连接数
	ActiveConnections int `json:"active_connections"`

	// 当前空闲连接数
	IdleConnections int `json:"idle_connections"`

	// 等待获取连接的请求数
	WaitCount int64 `json:"wait_count"`

	// 等待时间总和(ms)
	WaitDuration int64 `json:"wait_duration"`

	// 最大等待时间(ms)
	MaxWaitDuration int64 `json:"max_wait_duration"`

	// 平均等待时间(ms)，由WaitDuration/WaitCount计算得出
	AvgWaitDuration int64 `json:"avg_wait_duration"`

	// 连接获取超时次数
	TimeoutCount int64 `json:"timeout_count"`

	// 连接错误次数
	ErrorCount int64 `json:"error_count"`

	// 上次统计时间
	LastUpdated time.Time `json:"last_updated"`
}

// Clone 创建统计信息的副本
func (s *PoolStats) Clone() *PoolStats {
	if s == nil {
		return nil
	}

	return &PoolStats{
		ActiveConnections: s.ActiveConnections,
		IdleConnections:   s.IdleConnections,
		WaitCount:         s.WaitCount,
		WaitDuration:      s.WaitDuration,
		MaxWaitDuration:   s.MaxWaitDuration,
		AvgWaitDuration:   s.AvgWaitDuration,
		TimeoutCount:      s.TimeoutCount,
		ErrorCount:        s.ErrorCount,
		LastUpdated:       s.LastUpdated,
	}
}

// Merge 合并其他统计信息
func (s *PoolStats) Merge(other *PoolStats) {
	if s == nil || other == nil {
		return
	}

	s.ActiveConnections += other.ActiveConnections
	s.IdleConnections += other.IdleConnections
	s.WaitCount += other.WaitCount
	s.WaitDuration += other.WaitDuration

	if other.MaxWaitDuration > s.MaxWaitDuration {
		s.MaxWaitDuration = other.MaxWaitDuration
	}

	// 重新计算平均等待时间
	if s.WaitCount > 0 {
		s.AvgWaitDuration = s.WaitDuration / s.WaitCount
	}

	s.TimeoutCount += other.TimeoutCount
	s.ErrorCount += other.ErrorCount

	// 使用最新的更新时间
	if other.LastUpdated.After(s.LastUpdated) {
		s.LastUpdated = other.LastUpdated
	}
}

// UpdateAvg 更新平均值
func (s *PoolStats) UpdateAvg() {
	if s == nil || s.WaitCount == 0 {
		return
	}

	s.AvgWaitDuration = s.WaitDuration / s.WaitCount
}

// ConnectionPool 连接池接口
type ConnectionPool interface {
	// Stats 获取连接池统计信息
	Stats() *PoolStats

	// Close 关闭连接池
	Close() error

	// Resize 调整连接池大小
	// maxIdle: 最大空闲连接数
	// maxActive: 最大活跃连接数
	Resize(maxIdle, maxActive int) error

	// ClearIdle 清除空闲连接，返回清除的连接数
	ClearIdle() int
}

// PoolConfig 连接池配置接口
type PoolConfig interface {
	// Validate 验证配置
	Validate() error

	// IsStatsEnabled 是否启用统计
	IsStatsEnabled() bool
}

// BasePoolStats 基础连接池统计实现
type BasePoolStats struct {
	stats      *PoolStats
	mu         sync.RWMutex
	updateTime time.Time
}

// NewBasePoolStats 创建新的基础连接池统计
func NewBasePoolStats() *BasePoolStats {
	return &BasePoolStats{
		stats: &PoolStats{
			LastUpdated: time.Now(),
		},
		updateTime: time.Now(),
	}
}

// GetStats 获取统计数据
func (b *BasePoolStats) GetStats() *PoolStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.stats.Clone()
}

// UpdateStats 更新统计数据
func (b *BasePoolStats) UpdateStats(updater func(*PoolStats)) {
	b.mu.Lock()
	defer b.mu.Unlock()

	updater(b.stats)
	b.stats.LastUpdated = time.Now()
	b.stats.UpdateAvg()
}

// RecordWait 记录等待情况
func (b *BasePoolStats) RecordWait(waitDuration time.Duration, timeout bool, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.stats.WaitCount++
	waitMs := waitDuration.Milliseconds()
	b.stats.WaitDuration += waitMs

	if waitMs > b.stats.MaxWaitDuration {
		b.stats.MaxWaitDuration = waitMs
	}

	if timeout {
		b.stats.TimeoutCount++
	}

	if err != nil {
		b.stats.ErrorCount++
		pr.Error("连接池记录错误: %v", err)
	}

	b.stats.UpdateAvg()
	b.stats.LastUpdated = time.Now()
}

// UpdateConnections 更新连接数量
func (b *BasePoolStats) UpdateConnections(active, idle int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.stats.ActiveConnections = active
	b.stats.IdleConnections = idle
	b.stats.LastUpdated = time.Now()
}

// Reset 重置统计数据
func (b *BasePoolStats) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.stats = &PoolStats{
		LastUpdated: time.Now(),
	}
}

// PoolError 连接池错误
type PoolError struct {
	Message string
	Cause   error
}

// Error 实现error接口
func (e *PoolError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// NewPoolError 创建新的连接池错误
func NewPoolError(message string, cause error) error {
	return errors.Wrap(cause, message)
}
