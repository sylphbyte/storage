// Package health 提供存储服务的健康检查功能
package health

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
)

// HealthManager 健康检查管理器
type HealthManager struct {
	checkers             map[string]HealthCheck                   // 健康检查器集合
	lastResults          map[string]HealthStatus                  // 最后一次检查结果
	checkInterval        time.Duration                            // 检查间隔
	timeout              time.Duration                            // 检查超时
	lock                 sync.RWMutex                             // 读写锁
	stop                 chan struct{}                            // 停止信号
	healthyCallback      func(string, HealthStatus)               // 健康状态回调
	degradedCallback     func(string, HealthStatus)               // 降级状态回调
	unhealthyCallback    func(string, HealthStatus)               // 不健康状态回调
	statusChangeCallback func(string, HealthStatus, HealthStatus) // 状态变化回调
}

// ManagerOption 管理器选项
type ManagerOption func(*HealthManager)

// WithCheckInterval 设置检查间隔
func WithCheckInterval(interval time.Duration) ManagerOption {
	return func(m *HealthManager) {
		m.checkInterval = interval
	}
}

// WithTimeout 设置检查超时
func WithTimeout(timeout time.Duration) ManagerOption {
	return func(m *HealthManager) {
		m.timeout = timeout
	}
}

// WithHealthyCallback 设置健康状态回调
func WithHealthyCallback(callback func(string, HealthStatus)) ManagerOption {
	return func(m *HealthManager) {
		m.healthyCallback = callback
	}
}

// WithDegradedCallback 设置降级状态回调
func WithDegradedCallback(callback func(string, HealthStatus)) ManagerOption {
	return func(m *HealthManager) {
		m.degradedCallback = callback
	}
}

// WithUnhealthyCallback 设置不健康状态回调
func WithUnhealthyCallback(callback func(string, HealthStatus)) ManagerOption {
	return func(m *HealthManager) {
		m.unhealthyCallback = callback
	}
}

// WithStatusChangeCallback 设置状态变化回调
func WithStatusChangeCallback(callback func(string, HealthStatus, HealthStatus)) ManagerOption {
	return func(m *HealthManager) {
		m.statusChangeCallback = callback
	}
}

// HealthSummary 健康状态摘要
type HealthSummary struct {
	OverallStatus     string                   `json:"overall_status"`     // 整体状态
	CheckedServices   int                      `json:"checked_services"`   // 已检查服务数
	HealthyServices   int                      `json:"healthy_services"`   // 健康服务数
	DegradedServices  int                      `json:"degraded_services"`  // 降级服务数
	UnhealthyServices int                      `json:"unhealthy_services"` // 不健康服务数
	StatusDetails     map[string]string        `json:"status_details"`     // 各服务状态
	LastCheckTime     map[string]time.Time     `json:"last_check_time"`    // 最后检查时间
	CheckLatency      map[string]time.Duration `json:"check_latency"`      // 检查延迟
}

// NewHealthManager 创建新的健康检查管理器
func NewHealthManager(options ...ManagerOption) *HealthManager {
	manager := &HealthManager{
		checkers:      make(map[string]HealthCheck),
		lastResults:   make(map[string]HealthStatus),
		checkInterval: 60 * time.Second, // 默认60秒检查一次
		timeout:       5 * time.Second,  // 默认超时5秒
		stop:          make(chan struct{}),
	}

	// 应用选项
	for _, option := range options {
		option(manager)
	}

	return manager
}

// RegisterChecker 注册健康检查器
func (m *HealthManager) RegisterChecker(checker HealthCheck) error {
	if checker == nil {
		return errors.New("健康检查器不能为空")
	}

	name := checker.Name()
	if name == "" {
		return errors.New("健康检查器名称不能为空")
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	// 检查是否已经存在同名检查器
	if _, exists := m.checkers[name]; exists {
		return errors.Errorf("健康检查器'%s'已存在", name)
	}

	m.checkers[name] = checker
	pr.Info("已注册健康检查器: %s (%s)", name, checker.Type())

	return nil
}

// UnregisterChecker 注销健康检查器
func (m *HealthManager) UnregisterChecker(name string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	// 检查是否存在该检查器
	if _, exists := m.checkers[name]; !exists {
		return errors.Errorf("健康检查器'%s'不存在", name)
	}

	delete(m.checkers, name)
	delete(m.lastResults, name)
	pr.Info("已注销健康检查器: %s", name)

	return nil
}

// CheckAll 检查所有服务的健康状态
func (m *HealthManager) CheckAll(ctx context.Context) (map[string]HealthStatus, error) {
	m.lock.RLock()
	checkers := make(map[string]HealthCheck, len(m.checkers))
	for name, checker := range m.checkers {
		checkers[name] = checker
	}
	m.lock.RUnlock()

	results := make(map[string]HealthStatus, len(checkers))
	var wg sync.WaitGroup
	var resultLock sync.Mutex

	for name, checker := range checkers {
		wg.Add(1)
		go func(name string, checker HealthCheck) {
			defer wg.Done()

			// 创建带超时的上下文
			checkCtx, cancel := context.WithTimeout(ctx, m.timeout)
			defer cancel()

			// 执行健康检查
			status, err := checker.Check(checkCtx)
			if err != nil {
				pr.Warning("健康检查'%s'失败: %v", name, err)
				// 出错已经在Check方法中处理，这里只需记录结果
			}

			resultLock.Lock()
			results[name] = status
			resultLock.Unlock()

			// 处理状态变化和回调
			m.handleStatusChange(name, status)
		}(name, checker)
	}

	wg.Wait()

	// 更新最后结果
	m.lock.Lock()
	for name, status := range results {
		m.lastResults[name] = status
	}
	m.lock.Unlock()

	return results, nil
}

// CheckOne 检查指定服务的健康状态
func (m *HealthManager) CheckOne(ctx context.Context, name string) (HealthStatus, error) {
	m.lock.RLock()
	checker, exists := m.checkers[name]
	m.lock.RUnlock()

	if !exists {
		return HealthStatus{}, errors.Errorf("健康检查器'%s'不存在", name)
	}

	// 创建带超时的上下文
	checkCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	// 执行健康检查
	status, err := checker.Check(checkCtx)
	if err != nil {
		pr.Warning("健康检查'%s'失败: %v", name, err)
		// 出错已经在Check方法中处理，这里只需记录结果
	}

	// 更新最后结果
	m.lock.Lock()
	m.lastResults[name] = status
	m.lock.Unlock()

	// 处理状态变化和回调
	m.handleStatusChange(name, status)

	return status, nil
}

// GetHealthSummary 获取健康状态摘要
func (m *HealthManager) GetHealthSummary() HealthSummary {
	m.lock.RLock()
	defer m.lock.RUnlock()

	summary := HealthSummary{
		OverallStatus:   StatusHealthy,
		CheckedServices: len(m.lastResults),
		StatusDetails:   make(map[string]string),
		LastCheckTime:   make(map[string]time.Time),
		CheckLatency:    make(map[string]time.Duration),
	}

	// 统计各状态服务数量
	for name, status := range m.lastResults {
		summary.StatusDetails[name] = status.Status
		summary.LastCheckTime[name] = status.LastCheck
		summary.CheckLatency[name] = status.Latency

		switch status.Status {
		case StatusHealthy:
			summary.HealthyServices++
		case StatusDegraded:
			summary.DegradedServices++
			// 如果有降级服务，整体状态为降级
			if summary.OverallStatus == StatusHealthy {
				summary.OverallStatus = StatusDegraded
			}
		case StatusUnhealthy:
			summary.UnhealthyServices++
			// 如果有不健康服务，整体状态为不健康
			summary.OverallStatus = StatusUnhealthy
		}
	}

	return summary
}

// GetLastCheckResult 获取最后一次检查结果
func (m *HealthManager) GetLastCheckResult(name string) (HealthStatus, bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	status, exists := m.lastResults[name]
	return status, exists
}

// GetAllLastCheckResults 获取所有最后一次检查结果
func (m *HealthManager) GetAllLastCheckResults() map[string]HealthStatus {
	m.lock.RLock()
	defer m.lock.RUnlock()

	results := make(map[string]HealthStatus, len(m.lastResults))
	for name, status := range m.lastResults {
		results[name] = status
	}

	return results
}

// StartPeriodicalCheck 开始定期健康检查
func (m *HealthManager) StartPeriodicalCheck(ctx context.Context) {
	ticker := time.NewTicker(m.checkInterval)

	go func() {
		// 启动时先执行一次检查
		m.CheckAll(ctx)

		for {
			select {
			case <-ticker.C:
				// 定时执行健康检查
				m.CheckAll(ctx)
			case <-m.stop:
				ticker.Stop()
				return
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()

	pr.Info("已启动定期健康检查，间隔 %v", m.checkInterval)
}

// StopPeriodicalCheck 停止定期健康检查
func (m *HealthManager) StopPeriodicalCheck() {
	close(m.stop)
	m.stop = make(chan struct{})
	pr.Info("已停止定期健康检查")
}

// SetCheckInterval 设置检查间隔
func (m *HealthManager) SetCheckInterval(interval time.Duration) {
	if interval <= 0 {
		return
	}

	m.lock.Lock()
	m.checkInterval = interval
	m.lock.Unlock()

	pr.Info("已设置健康检查间隔为 %v", interval)
}

// SetTimeout 设置检查超时
func (m *HealthManager) SetTimeout(timeout time.Duration) {
	if timeout <= 0 {
		return
	}

	m.lock.Lock()
	m.timeout = timeout
	m.lock.Unlock()

	pr.Info("已设置健康检查超时为 %v", timeout)
}

// handleStatusChange 处理状态变化并执行回调
func (m *HealthManager) handleStatusChange(name string, currentStatus HealthStatus) {
	m.lock.Lock()
	previousStatus, exists := m.lastResults[name]
	m.lastResults[name] = currentStatus
	m.lock.Unlock()

	// 执行状态回调
	switch currentStatus.Status {
	case StatusHealthy:
		if m.healthyCallback != nil {
			m.healthyCallback(name, currentStatus)
		}
	case StatusDegraded:
		if m.degradedCallback != nil {
			m.degradedCallback(name, currentStatus)
		}
	case StatusUnhealthy:
		if m.unhealthyCallback != nil {
			m.unhealthyCallback(name, currentStatus)
		}
	}

	// 只有之前有状态记录且状态发生变化时，才执行状态变化回调
	if exists && previousStatus.Status != currentStatus.Status && m.statusChangeCallback != nil {
		m.statusChangeCallback(name, previousStatus, currentStatus)
	}
}
