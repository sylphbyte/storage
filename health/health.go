// Package health 提供存储服务的健康检查功能
package health

import (
	"context"
	"time"
)

// HealthStatus 表示健康检查的结果状态
type HealthStatus struct {
	// Status 健康状态: "healthy", "degraded", "unhealthy"
	Status string `json:"status"`

	// Details 包含健康检查详细信息
	Details map[string]string `json:"details"`

	// LastCheck 最后一次检查时间
	LastCheck time.Time `json:"last_check"`

	// Latency 检查耗时
	Latency time.Duration `json:"latency"`
}

// HealthCheck 健康检查接口
type HealthCheck interface {
	// Check 执行健康检查
	Check(ctx context.Context) (HealthStatus, error)

	// Name 获取健康检查名称
	Name() string

	// Type 获取健康检查类型
	Type() string
}

// 健康状态常量
const (
	StatusHealthy   = "healthy"   // 健康状态
	StatusDegraded  = "degraded"  // 服务降级状态
	StatusUnhealthy = "unhealthy" // 不健康状态
)

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	CheckType string       `json:"check_type"` // 检查类型
	CheckName string       `json:"check_name"` // 检查名称
	Status    HealthStatus `json:"status"`     // 健康状态
	Error     string       `json:"error"`      // 错误信息
	Timestamp time.Time    `json:"timestamp"`  // 时间戳
}

// NewHealthStatus 创建新的健康状态对象
func NewHealthStatus(status string) HealthStatus {
	return HealthStatus{
		Status:    status,
		Details:   make(map[string]string),
		LastCheck: time.Now(),
	}
}

// WithDetail 添加详细信息
func (h HealthStatus) WithDetail(key, value string) HealthStatus {
	h.Details[key] = value
	return h
}

// WithLatency 设置延迟
func (h HealthStatus) WithLatency(latency time.Duration) HealthStatus {
	h.Latency = latency
	return h
}

// Checker 健康检查器的基本结构
type Checker struct {
	name       string              // 检查名称
	checkType  string              // 检查类型
	history    []HealthCheckResult // 历史结果
	maxHistory int                 // 最大历史记录数
}

// NewChecker 创建新的健康检查器
func NewChecker(name, checkType string) *Checker {
	return &Checker{
		name:       name,
		checkType:  checkType,
		history:    make([]HealthCheckResult, 0, 10),
		maxHistory: 10,
	}
}

// Name 获取健康检查名称
func (c *Checker) Name() string {
	return c.name
}

// Type 获取健康检查类型
func (c *Checker) Type() string {
	return c.checkType
}

// RecordResult 记录健康检查结果
func (c *Checker) RecordResult(status HealthStatus, err error) {
	result := HealthCheckResult{
		CheckType: c.checkType,
		CheckName: c.name,
		Status:    status,
		Timestamp: time.Now(),
	}

	if err != nil {
		result.Error = err.Error()
	}

	// 保持历史记录不超过最大限制
	if len(c.history) >= c.maxHistory {
		// 移除最旧的记录
		c.history = c.history[1:]
	}

	c.history = append(c.history, result)
}

// GetHistory 获取历史记录
func (c *Checker) GetHistory() []HealthCheckResult {
	return c.history
}

// GetLastResult 获取最近一次结果
func (c *Checker) GetLastResult() (HealthCheckResult, bool) {
	if len(c.history) == 0 {
		return HealthCheckResult{}, false
	}
	return c.history[len(c.history)-1], true
}
