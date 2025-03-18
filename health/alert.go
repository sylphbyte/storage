// Package health 提供存储服务的健康检查功能
package health

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
)

// AlertLevel 告警级别
type AlertLevel string

const (
	// AlertLevelInfo 信息级别
	AlertLevelInfo AlertLevel = "info"
	// AlertLevelWarning 警告级别
	AlertLevelWarning AlertLevel = "warning"
	// AlertLevelError 错误级别
	AlertLevelError AlertLevel = "error"
	// AlertLevelCritical 严重级别
	AlertLevelCritical AlertLevel = "critical"
)

// Alert 告警信息
type Alert struct {
	ID           string                 `json:"id"`           // 告警ID
	Name         string                 `json:"name"`         // 告警名称
	Level        AlertLevel             `json:"level"`        // 告警级别
	Message      string                 `json:"message"`      // 告警消息
	ServiceType  string                 `json:"service_type"` // 服务类型
	ServiceName  string                 `json:"service_name"` // 服务名称
	Timestamp    time.Time              `json:"timestamp"`    // 触发时间
	Details      map[string]string      `json:"details"`      // 详细信息
	Metadata     map[string]interface{} `json:"metadata"`     // 元数据
	Acknowledged bool                   `json:"acknowledged"` // 是否已确认
	ResolvedAt   *time.Time             `json:"resolved_at"`  // 解决时间
}

// AlertRule 告警规则
type AlertRule struct {
	ID           string                 `json:"id"`            // 规则ID
	Name         string                 `json:"name"`          // 规则名称
	Description  string                 `json:"description"`   // 规则描述
	Level        AlertLevel             `json:"level"`         // 告警级别
	ServiceTypes []string               `json:"service_types"` // 适用服务类型
	Condition    string                 `json:"condition"`     // 条件表达式
	Message      string                 `json:"message"`       // 告警消息模板
	Enabled      bool                   `json:"enabled"`       // 是否启用
	Metadata     map[string]interface{} `json:"metadata"`      // 元数据
}

// AlertHandler 告警处理器接口
type AlertHandler interface {
	// HandleAlert 处理告警
	HandleAlert(alert Alert) error
	// Name 获取处理器名称
	Name() string
}

// LogAlertHandler 日志告警处理器
type LogAlertHandler struct {
	name string
}

// NewLogAlertHandler 创建新的日志告警处理器
func NewLogAlertHandler(name string) *LogAlertHandler {
	if name == "" {
		name = "log_handler"
	}
	return &LogAlertHandler{name: name}
}

// HandleAlert 处理告警
func (h *LogAlertHandler) HandleAlert(alert Alert) error {
	switch alert.Level {
	case AlertLevelInfo:
		pr.Info("[告警] %s: %s - %s", alert.ServiceName, alert.Name, alert.Message)
	case AlertLevelWarning:
		pr.Warning("[告警] %s: %s - %s", alert.ServiceName, alert.Name, alert.Message)
	case AlertLevelError, AlertLevelCritical:
		pr.Error("[告警] %s: %s - %s", alert.ServiceName, alert.Name, alert.Message)
	}
	return nil
}

// Name 获取处理器名称
func (h *LogAlertHandler) Name() string {
	return h.name
}

// AlertManager 告警管理器
type AlertManager struct {
	rules             map[string]AlertRule    // 告警规则
	handlers          map[string]AlertHandler // 告警处理器
	activeAlerts      map[string]Alert        // 活跃告警
	resolvedAlerts    []Alert                 // 已解决告警
	alertsLock        sync.RWMutex            // 告警锁
	silencedAlerts    map[string]time.Time    // 静默告警
	dedupeWindow      time.Duration           // 去重窗口
	maxResolvedAlerts int                     // 最大已解决告警数
}

// NewAlertManager 创建新的告警管理器
func NewAlertManager() *AlertManager {
	return &AlertManager{
		rules:             make(map[string]AlertRule),
		handlers:          make(map[string]AlertHandler),
		activeAlerts:      make(map[string]Alert),
		resolvedAlerts:    make([]Alert, 0),
		silencedAlerts:    make(map[string]time.Time),
		dedupeWindow:      5 * time.Minute, // 默认5分钟去重
		maxResolvedAlerts: 100,             // 默认最多保留100条已解决告警
	}
}

// RegisterRule 注册告警规则
func (m *AlertManager) RegisterRule(rule AlertRule) error {
	if rule.ID == "" {
		return errors.New("告警规则ID不能为空")
	}

	m.alertsLock.Lock()
	defer m.alertsLock.Unlock()

	// 检查是否已经存在同ID规则
	if _, exists := m.rules[rule.ID]; exists {
		return errors.Errorf("告警规则'%s'已存在", rule.ID)
	}

	m.rules[rule.ID] = rule
	pr.Info("已注册告警规则: %s (%s)", rule.Name, rule.ID)

	return nil
}

// UnregisterRule 注销告警规则
func (m *AlertManager) UnregisterRule(ruleID string) error {
	m.alertsLock.Lock()
	defer m.alertsLock.Unlock()

	// 检查是否存在该规则
	if _, exists := m.rules[ruleID]; !exists {
		return errors.Errorf("告警规则'%s'不存在", ruleID)
	}

	delete(m.rules, ruleID)
	pr.Info("已注销告警规则: %s", ruleID)

	return nil
}

// RegisterHandler 注册告警处理器
func (m *AlertManager) RegisterHandler(handler AlertHandler) error {
	if handler == nil {
		return errors.New("告警处理器不能为空")
	}

	name := handler.Name()
	if name == "" {
		return errors.New("告警处理器名称不能为空")
	}

	m.alertsLock.Lock()
	defer m.alertsLock.Unlock()

	// 检查是否已经存在同名处理器
	if _, exists := m.handlers[name]; exists {
		return errors.Errorf("告警处理器'%s'已存在", name)
	}

	m.handlers[name] = handler
	pr.Info("已注册告警处理器: %s", name)

	return nil
}

// UnregisterHandler 注销告警处理器
func (m *AlertManager) UnregisterHandler(name string) error {
	m.alertsLock.Lock()
	defer m.alertsLock.Unlock()

	// 检查是否存在该处理器
	if _, exists := m.handlers[name]; !exists {
		return errors.Errorf("告警处理器'%s'不存在", name)
	}

	delete(m.handlers, name)
	pr.Info("已注销告警处理器: %s", name)

	return nil
}

// SetDedupeWindow 设置去重窗口
func (m *AlertManager) SetDedupeWindow(window time.Duration) {
	if window <= 0 {
		return
	}

	m.alertsLock.Lock()
	m.dedupeWindow = window
	m.alertsLock.Unlock()

	pr.Info("已设置告警去重窗口为 %v", window)
}

// SetMaxResolvedAlerts 设置最大已解决告警数
func (m *AlertManager) SetMaxResolvedAlerts(max int) {
	if max <= 0 {
		return
	}

	m.alertsLock.Lock()
	m.maxResolvedAlerts = max

	// 如果当前已解决告警数超过新的最大值，裁剪列表
	if len(m.resolvedAlerts) > max {
		m.resolvedAlerts = m.resolvedAlerts[len(m.resolvedAlerts)-max:]
	}

	m.alertsLock.Unlock()

	pr.Info("已设置最大已解决告警数为 %d", max)
}

// Alert 触发告警
func (m *AlertManager) Alert(ctx context.Context, alert Alert) error {
	if alert.ID == "" {
		alert.ID = fmt.Sprintf("%s_%s_%d", alert.ServiceName, alert.Name, time.Now().UnixNano())
	}

	if alert.Timestamp.IsZero() {
		alert.Timestamp = time.Now()
	}

	if alert.Details == nil {
		alert.Details = make(map[string]string)
	}

	// 去重检查
	m.alertsLock.Lock()

	// 检查是否在静默期内
	if silencedUntil, exists := m.silencedAlerts[alert.ID]; exists && time.Now().Before(silencedUntil) {
		m.alertsLock.Unlock()
		return nil
	}

	// 检查是否已经存在相同告警
	if existing, exists := m.activeAlerts[alert.ID]; exists {
		// 如果在去重窗口内，不重复发送
		if time.Since(existing.Timestamp) < m.dedupeWindow {
			m.alertsLock.Unlock()
			return nil
		}
	}

	// 存储告警
	m.activeAlerts[alert.ID] = alert
	m.alertsLock.Unlock()

	// 分发告警给所有处理器
	var errs []error
	for _, handler := range m.getHandlers() {
		if err := handler.HandleAlert(alert); err != nil {
			errs = append(errs, errors.Wrapf(err, "处理器'%s'处理告警失败", handler.Name()))
		}
	}

	if len(errs) > 0 {
		return errors.Errorf("部分处理器处理告警失败: %v", errs)
	}

	return nil
}

// Resolve 解决告警
func (m *AlertManager) Resolve(alertID string) {
	m.alertsLock.Lock()
	defer m.alertsLock.Unlock()

	// 检查是否存在该告警
	alert, exists := m.activeAlerts[alertID]
	if !exists {
		return
	}

	// 标记解决时间
	now := time.Now()
	alert.ResolvedAt = &now

	// 将告警移至已解决列表
	m.resolvedAlerts = append(m.resolvedAlerts, alert)
	delete(m.activeAlerts, alertID)

	// 如果已解决告警数超过最大值，裁剪列表
	if len(m.resolvedAlerts) > m.maxResolvedAlerts {
		m.resolvedAlerts = m.resolvedAlerts[len(m.resolvedAlerts)-m.maxResolvedAlerts:]
	}
}

// Acknowledge 确认告警
func (m *AlertManager) Acknowledge(alertID string) bool {
	m.alertsLock.Lock()
	defer m.alertsLock.Unlock()

	// 检查是否存在该告警
	alert, exists := m.activeAlerts[alertID]
	if !exists {
		return false
	}

	// 标记为已确认
	alert.Acknowledged = true
	m.activeAlerts[alertID] = alert

	return true
}

// Silence 静默告警
func (m *AlertManager) Silence(alertID string, duration time.Duration) {
	if duration <= 0 {
		return
	}

	m.alertsLock.Lock()
	defer m.alertsLock.Unlock()

	m.silencedAlerts[alertID] = time.Now().Add(duration)
}

// CheckRules 根据健康状态检查告警规则
func (m *AlertManager) CheckRules(ctx context.Context, serviceName string, serviceType string, status HealthStatus) {
	m.alertsLock.RLock()
	rules := make([]AlertRule, 0)
	for _, rule := range m.rules {
		if !rule.Enabled {
			continue
		}

		// 检查服务类型是否匹配
		if len(rule.ServiceTypes) > 0 {
			matched := false
			for _, ruleServiceType := range rule.ServiceTypes {
				if ruleServiceType == serviceType {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		rules = append(rules, rule)
	}
	m.alertsLock.RUnlock()

	// 针对每个规则评估状态
	for _, rule := range rules {
		// 这里简化规则评估逻辑，仅根据状态判断
		// 实际项目中应该使用更复杂的规则引擎
		switch rule.Condition {
		case "status == unhealthy", "unhealthy":
			if status.Status == StatusUnhealthy {
				m.createAlertFromRule(ctx, rule, serviceName, serviceType, status)
			}
		case "status == degraded", "degraded":
			if status.Status == StatusDegraded {
				m.createAlertFromRule(ctx, rule, serviceName, serviceType, status)
			}
		case "status != healthy", "!healthy":
			if status.Status != StatusHealthy {
				m.createAlertFromRule(ctx, rule, serviceName, serviceType, status)
			}
		}
	}
}

// 创建基于规则的告警
func (m *AlertManager) createAlertFromRule(ctx context.Context, rule AlertRule, serviceName string, serviceType string, status HealthStatus) {
	alert := Alert{
		ID:          fmt.Sprintf("%s_%s_%s", serviceName, rule.ID, time.Now().Format("20060102150405")),
		Name:        rule.Name,
		Level:       rule.Level,
		Message:     rule.Message,
		ServiceType: serviceType,
		ServiceName: serviceName,
		Timestamp:   time.Now(),
		Details:     status.Details,
		Metadata:    rule.Metadata,
	}

	// 触发告警
	m.Alert(ctx, alert)
}

// GetActiveAlerts 获取活跃告警
func (m *AlertManager) GetActiveAlerts() []Alert {
	m.alertsLock.RLock()
	defer m.alertsLock.RUnlock()

	alerts := make([]Alert, 0, len(m.activeAlerts))
	for _, alert := range m.activeAlerts {
		alerts = append(alerts, alert)
	}

	return alerts
}

// GetResolvedAlerts 获取已解决告警
func (m *AlertManager) GetResolvedAlerts() []Alert {
	m.alertsLock.RLock()
	defer m.alertsLock.RUnlock()

	// 返回副本
	alerts := make([]Alert, len(m.resolvedAlerts))
	copy(alerts, m.resolvedAlerts)

	return alerts
}

// GetAlert 获取特定告警
func (m *AlertManager) GetAlert(alertID string) (Alert, bool) {
	m.alertsLock.RLock()
	defer m.alertsLock.RUnlock()

	// 先检查活跃告警
	if alert, exists := m.activeAlerts[alertID]; exists {
		return alert, true
	}

	// 再检查已解决告警
	for _, alert := range m.resolvedAlerts {
		if alert.ID == alertID {
			return alert, true
		}
	}

	return Alert{}, false
}

// getHandlers 获取所有告警处理器（线程安全）
func (m *AlertManager) getHandlers() []AlertHandler {
	m.alertsLock.RLock()
	defer m.alertsLock.RUnlock()

	handlers := make([]AlertHandler, 0, len(m.handlers))
	for _, handler := range m.handlers {
		handlers = append(handlers, handler)
	}

	return handlers
}
