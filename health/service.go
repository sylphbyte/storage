package health

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
)

// HealthService 存储服务健康检查与告警服务
type HealthService struct {
	healthManager *HealthManager // 健康检查管理器
	alertManager  *AlertManager  // 告警管理器
	checkInterval time.Duration  // 检查间隔
	timeout       time.Duration  // 检查超时
	running       bool           // 运行状态
	mutex         sync.RWMutex   // 并发锁
	stopChan      chan struct{}  // 停止信号
	wg            sync.WaitGroup // 等待组
}

// ServiceOption 服务选项函数
type ServiceOption func(*HealthService)

// ServiceWithCheckInterval 设置检查间隔
func ServiceWithCheckInterval(interval time.Duration) ServiceOption {
	return func(s *HealthService) {
		if interval > 0 {
			s.checkInterval = interval
		}
	}
}

// ServiceWithTimeout 设置检查超时
func ServiceWithTimeout(timeout time.Duration) ServiceOption {
	return func(s *HealthService) {
		if timeout > 0 {
			s.timeout = timeout
		}
	}
}

// NewHealthService 创建新的健康检查与告警服务
func NewHealthService(opts ...ServiceOption) *HealthService {
	service := &HealthService{
		healthManager: NewHealthManager(),
		alertManager:  NewAlertManager(),
		checkInterval: 60 * time.Second, // 默认每分钟检查一次
		timeout:       10 * time.Second, // 默认10秒超时
		running:       false,
		stopChan:      make(chan struct{}),
	}

	// 应用选项
	for _, opt := range opts {
		opt(service)
	}

	// 注册默认告警处理器
	logHandler := NewLogAlertHandler("default_log_handler")
	err := service.alertManager.RegisterHandler(logHandler)
	if err != nil {
		pr.Warning("注册默认日志告警处理器失败: %v", err)
	}

	// 设置健康状态变化回调函数
	service.healthManager = NewHealthManager(
		WithHealthyCallback(func(name string, status HealthStatus) {
			service.onServiceHealthy(context.Background(), name, status)
		}),
		WithDegradedCallback(func(name string, status HealthStatus) {
			service.onServiceDegraded(context.Background(), name, status)
		}),
		WithUnhealthyCallback(func(name string, status HealthStatus) {
			service.onServiceUnhealthy(context.Background(), name, status)
		}),
	)

	return service
}

// RegisterChecker 注册健康检查器
func (s *HealthService) RegisterChecker(name string, checker HealthCheck) error {
	if checker == nil {
		return errors.New("健康检查器不能为空")
	}

	return s.healthManager.RegisterChecker(checker)
}

// UnregisterChecker 注销健康检查器
func (s *HealthService) UnregisterChecker(name string) error {
	return s.healthManager.UnregisterChecker(name)
}

// RegisterMySQLChecker 注册MySQL健康检查器
func (s *HealthService) RegisterMySQLChecker(name string, db *sql.DB, settings *MySQLCheckSettings) error {
	checker := NewMySQLChecker(name, db, settings)
	return s.healthManager.RegisterChecker(checker)
}

// RegisterRedisChecker 注册Redis健康检查器
func (s *HealthService) RegisterRedisChecker(name string, client *redis.Client, settings *RedisCheckSettings) error {
	checker := NewRedisChecker(name, client, settings)
	return s.healthManager.RegisterChecker(checker)
}

// RegisterESChecker 注册Elasticsearch健康检查器
func (s *HealthService) RegisterESChecker(name string, client *elasticsearch.Client, settings *ESCheckSettings) error {
	checker := NewESChecker(name, client, settings)
	return s.healthManager.RegisterChecker(checker)
}

// RegisterAlertRule 注册告警规则
func (s *HealthService) RegisterAlertRule(rule AlertRule) error {
	return s.alertManager.RegisterRule(rule)
}

// UnregisterAlertRule 注销告警规则
func (s *HealthService) UnregisterAlertRule(ruleID string) error {
	return s.alertManager.UnregisterRule(ruleID)
}

// RegisterAlertHandler 注册告警处理器
func (s *HealthService) RegisterAlertHandler(handler AlertHandler) error {
	return s.alertManager.RegisterHandler(handler)
}

// UnregisterAlertHandler 注销告警处理器
func (s *HealthService) UnregisterAlertHandler(name string) error {
	return s.alertManager.UnregisterHandler(name)
}

// RegisterWebhookHandler 注册Webhook告警处理器
func (s *HealthService) RegisterWebhookHandler(name string, config WebhookConfig) error {
	handler, err := NewWebhookHandler(name, config)
	if err != nil {
		return err
	}
	return s.RegisterAlertHandler(handler)
}

// RegisterFileHandler 注册文件告警处理器
func (s *HealthService) RegisterFileHandler(name string, config FileConfig) error {
	handler, err := NewFileHandler(name, config)
	if err != nil {
		return err
	}
	return s.RegisterAlertHandler(handler)
}

// Start 启动健康检查与告警服务
func (s *HealthService) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return errors.New("健康检查服务已经在运行")
	}

	pr.Info("启动健康检查与告警服务")

	// 执行一次初始检查
	s.checkAll()

	// 开启定期检查
	s.running = true
	s.stopChan = make(chan struct{})
	s.wg.Add(1)

	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.checkAll()
			case <-s.stopChan:
				return
			}
		}
	}()

	return nil
}

// Stop 停止健康检查与告警服务
func (s *HealthService) Stop() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return errors.New("健康检查服务未运行")
	}

	pr.Info("停止健康检查与告警服务")

	// 发送停止信号
	close(s.stopChan)
	s.running = false

	// 等待goroutine结束
	s.wg.Wait()

	return nil
}

// IsRunning 检查服务是否运行中
func (s *HealthService) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.running
}

// SetCheckInterval 设置检查间隔
func (s *HealthService) SetCheckInterval(interval time.Duration) error {
	if interval <= 0 {
		return errors.New("检查间隔必须大于0")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.checkInterval = interval

	// 如果服务正在运行，需要重启定时器
	if s.running {
		err := s.Stop()
		if err != nil {
			return errors.Wrap(err, "停止服务失败")
		}

		err = s.Start()
		if err != nil {
			return errors.Wrap(err, "重启服务失败")
		}
	}

	return nil
}

// CheckNow 立即执行检查
func (s *HealthService) CheckNow(ctx context.Context, name string) (HealthStatus, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// 创建超时上下文
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	status, err := s.healthManager.CheckOne(ctx, name)
	if err != nil {
		return HealthStatus{}, err
	}
	return status, nil
}

// CheckAllNow 立即检查所有服务
func (s *HealthService) CheckAllNow(ctx context.Context) map[string]HealthStatus {
	if ctx == nil {
		ctx = context.Background()
	}

	// 创建超时上下文
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	results, _ := s.healthManager.CheckAll(ctx)
	return results
}

// GetHealthSummary 获取健康状态摘要
func (s *HealthService) GetHealthSummary() HealthSummary {
	return s.healthManager.GetHealthSummary()
}

// GetActiveAlerts 获取活跃告警
func (s *HealthService) GetActiveAlerts() []Alert {
	return s.alertManager.GetActiveAlerts()
}

// GetResolvedAlerts 获取已解决告警
func (s *HealthService) GetResolvedAlerts() []Alert {
	return s.alertManager.GetResolvedAlerts()
}

// AcknowledgeAlert 确认告警
func (s *HealthService) AcknowledgeAlert(alertID string) bool {
	return s.alertManager.Acknowledge(alertID)
}

// ResolveAlert 解决告警
func (s *HealthService) ResolveAlert(alertID string) {
	s.alertManager.Resolve(alertID)
}

// SilenceAlert 静默告警
func (s *HealthService) SilenceAlert(alertID string, duration time.Duration) {
	s.alertManager.Silence(alertID, duration)
}

// 执行所有健康检查
func (s *HealthService) checkAll() {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	results, _ := s.healthManager.CheckAll(ctx)

	// 处理检查结果
	for name, status := range results {
		// 确定服务类型
		serviceType := "unknown"
		if status.Details != nil {
			if typ, ok := status.Details["service_type"]; ok {
				serviceType = typ
			}
		}

		// 根据健康状态检查告警规则
		s.alertManager.CheckRules(ctx, name, serviceType, status)
	}
}

// 服务变为健康状态的回调
func (s *HealthService) onServiceHealthy(ctx context.Context, name string, status HealthStatus) {
	pr.Info("服务 [%s] 变为健康状态", name)

	// 为该服务的所有未解决告警创建解决告警
	alerts := s.alertManager.GetActiveAlerts()
	for _, alert := range alerts {
		if alert.ServiceName == name {
			s.alertManager.Resolve(alert.ID)
		}
	}
}

// 服务变为降级状态的回调
func (s *HealthService) onServiceDegraded(ctx context.Context, name string, status HealthStatus) {
	var message string
	if status.Details != nil && status.Details["message"] != "" {
		message = status.Details["message"]
	} else {
		message = "服务性能降级"
	}

	pr.Warning("服务 [%s] 变为降级状态: %s", name, message)

	// 确定服务类型
	serviceType := "unknown"
	if status.Details != nil {
		if typ, ok := status.Details["service_type"]; ok {
			serviceType = typ
		}
	}

	// 创建降级告警
	s.alertManager.CheckRules(ctx, name, serviceType, status)
}

// 服务变为不健康状态的回调
func (s *HealthService) onServiceUnhealthy(ctx context.Context, name string, status HealthStatus) {
	var message string
	if status.Details != nil && status.Details["message"] != "" {
		message = status.Details["message"]
	} else {
		message = "服务不健康"
	}

	pr.Error("服务 [%s] 变为不健康状态: %s", name, message)

	// 确定服务类型
	serviceType := "unknown"
	if status.Details != nil {
		if typ, ok := status.Details["service_type"]; ok {
			serviceType = typ
		}
	}

	// 创建不健康告警
	s.alertManager.CheckRules(ctx, name, serviceType, status)
}

// CreateDefaultAlertRules 创建默认告警规则
func (s *HealthService) CreateDefaultAlertRules() {
	// 不健康规则
	unhealthyRule := AlertRule{
		ID:           "default_unhealthy",
		Name:         "服务不健康",
		Description:  "检测到服务处于不健康状态",
		Level:        AlertLevelError,
		ServiceTypes: []string{}, // 应用于所有服务类型
		Condition:    "status == unhealthy",
		Message:      "服务不健康，可能需要立即干预",
		Enabled:      true,
	}

	// 降级规则
	degradedRule := AlertRule{
		ID:           "default_degraded",
		Name:         "服务降级",
		Description:  "检测到服务处于降级状态",
		Level:        AlertLevelWarning,
		ServiceTypes: []string{}, // 应用于所有服务类型
		Condition:    "status == degraded",
		Message:      "服务性能降级，建议调查",
		Enabled:      true,
	}

	// MySQL特定规则
	mysqlRule := AlertRule{
		ID:           "mysql_connection_issues",
		Name:         "MySQL连接问题",
		Description:  "MySQL连接失败或连接池不足",
		Level:        AlertLevelError,
		ServiceTypes: []string{"mysql"},
		Condition:    "status != healthy",
		Message:      "MySQL数据库连接问题，需要检查数据库状态",
		Enabled:      true,
	}

	// Redis特定规则
	redisRule := AlertRule{
		ID:           "redis_memory_high",
		Name:         "Redis内存占用过高",
		Description:  "Redis内存占用率过高",
		Level:        AlertLevelWarning,
		ServiceTypes: []string{"redis"},
		Condition:    "status != healthy",
		Message:      "Redis内存使用率过高，可能需要扩容或清理数据",
		Enabled:      true,
	}

	// ES特定规则
	esRule := AlertRule{
		ID:           "es_cluster_issues",
		Name:         "ES集群问题",
		Description:  "Elasticsearch集群状态异常",
		Level:        AlertLevelError,
		ServiceTypes: []string{"elasticsearch"},
		Condition:    "status != healthy",
		Message:      "Elasticsearch集群状态异常，需要检查集群节点",
		Enabled:      true,
	}

	// 注册规则
	rules := []AlertRule{unhealthyRule, degradedRule, mysqlRule, redisRule, esRule}
	for _, rule := range rules {
		err := s.alertManager.RegisterRule(rule)
		if err != nil {
			pr.Warning("注册默认告警规则失败 [%s]: %v", rule.ID, err)
		} else {
			pr.Info("成功注册默认告警规则: %s", rule.ID)
		}
	}
}
