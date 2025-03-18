// Package enhanced 提供增强的存储服务管理功能
package enhanced

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
	"app/pkg/storage"
	"app/pkg/storage/elasticsearch"
	"app/pkg/storage/health"
	"app/pkg/storage/mysql"
	"app/pkg/storage/redis"
)

// ManagerOption 增强型存储管理器配置选项
type ManagerOption func(*StorageManager)

// WithHealthCheckInterval 设置健康检查间隔
func WithHealthCheckInterval(interval time.Duration) ManagerOption {
	return func(sm *StorageManager) {
		sm.healthCheckInterval = interval
	}
}

// WithHealthCheckTimeout 设置健康检查超时
func WithHealthCheckTimeout(timeout time.Duration) ManagerOption {
	return func(sm *StorageManager) {
		sm.healthCheckTimeout = timeout
	}
}

// WithLogAlertHandler 添加日志告警处理器
func WithLogAlertHandler() ManagerOption {
	return func(sm *StorageManager) {
		handler := health.NewLogAlertHandler("storage_log_handler")
		sm.alertHandlers = append(sm.alertHandlers, handler)
	}
}

// WithFileAlertHandler 添加文件告警处理器
func WithFileAlertHandler(filePath string) ManagerOption {
	return func(sm *StorageManager) {
		config := health.FileConfig{
			FilePath:     filePath,
			Format:       "json",
			RotateSize:   10 * 1024 * 1024, // 10MB
			MaxFiles:     5,
			Permissions:  0644,
			LevelFilters: []health.AlertLevel{health.AlertLevelWarning, health.AlertLevelError, health.AlertLevelCritical},
		}

		handler, err := health.NewFileHandler("storage_file_handler", config)
		if err != nil {
			pr.Error("创建文件告警处理器失败: %v", err)
			return
		}

		sm.alertHandlers = append(sm.alertHandlers, handler)
	}
}

// WithWebhookAlertHandler 添加Webhook告警处理器
func WithWebhookAlertHandler(url string, headers map[string]string) ManagerOption {
	return func(sm *StorageManager) {
		config := health.WebhookConfig{
			URL:          url,
			Method:       "POST",
			Headers:      headers,
			Timeout:      5 * time.Second,
			RetryCount:   3,
			RetryDelay:   2 * time.Second,
			LevelFilters: []health.AlertLevel{health.AlertLevelError, health.AlertLevelCritical},
		}

		handler, err := health.NewWebhookHandler("storage_webhook_handler", config)
		if err != nil {
			pr.Error("创建Webhook告警处理器失败: %v", err)
			return
		}

		sm.alertHandlers = append(sm.alertHandlers, handler)
	}
}

// StorageManager 增强型存储管理器
type StorageManager struct {
	// 存储映射
	dbStorages    map[string]storage.DBStorage
	redisStorages map[string]storage.RedisStorage
	esStorages    map[string]storage.ESStorage

	// 健康检查和告警
	healthService       *health.HealthService
	healthCheckInterval time.Duration
	healthCheckTimeout  time.Duration
	alertHandlers       []health.AlertHandler

	// 连接池管理
	mysqlPools map[string]*mysql.Pool
	redisPools map[string]*redis.Pool
	esPools    map[string]*elasticsearch.Pool

	// 并发控制
	mu sync.RWMutex
}

// NewStorageManager 创建增强型存储管理器
func NewStorageManager(opts ...ManagerOption) *StorageManager {
	sm := &StorageManager{
		dbStorages:          make(map[string]storage.DBStorage),
		redisStorages:       make(map[string]storage.RedisStorage),
		esStorages:          make(map[string]storage.ESStorage),
		mysqlPools:          make(map[string]*mysql.Pool),
		redisPools:          make(map[string]*redis.Pool),
		esPools:             make(map[string]*elasticsearch.Pool),
		healthCheckInterval: 60 * time.Second, // 默认60秒
		healthCheckTimeout:  5 * time.Second,  // 默认5秒
		alertHandlers:       []health.AlertHandler{},
	}

	// 应用配置选项
	for _, opt := range opts {
		opt(sm)
	}

	// 初始化健康检查服务
	healthServiceOpts := []health.ServiceOption{
		health.ServiceWithCheckInterval(sm.healthCheckInterval),
		health.ServiceWithTimeout(sm.healthCheckTimeout),
	}
	sm.healthService = health.NewHealthService(healthServiceOpts...)

	// 注册默认的告警处理器
	if len(sm.alertHandlers) == 0 {
		// 如果没有配置任何处理器，默认添加日志处理器
		WithLogAlertHandler()(sm)
	}

	// 注册告警处理器
	for _, handler := range sm.alertHandlers {
		err := sm.healthService.RegisterAlertHandler(handler)
		if err != nil {
			pr.Error("注册告警处理器 %s 失败: %v", handler.Name(), err)
		}
	}

	// 初始化默认告警规则
	sm.setupDefaultAlertRules()

	pr.Info("增强型存储管理器创建完成")
	return sm
}

// GetDB 获取数据库存储
func (sm *StorageManager) GetDB(name ...string) (storage.DBStorage, error) {
	dbName := "default"
	if len(name) > 0 && name[0] != "" {
		dbName = name[0]
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	db, ok := sm.dbStorages[dbName]
	if !ok {
		return nil, errors.Errorf("数据库 %s 未找到", dbName)
	}

	return db, nil
}

// RegisterDB 注册数据库存储
func (sm *StorageManager) RegisterDB(name string, storage storage.DBStorage) error {
	if name == "" {
		return errors.New("数据库名称不能为空")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.dbStorages[name]; exists {
		return errors.Errorf("数据库 %s 已存在", name)
	}

	// 注册数据库
	sm.dbStorages[name] = storage
	pr.Info("数据库 %s 已注册", name)

	// 为数据库配置健康检查
	err := sm.configureMySQLHealthCheck(name, storage)
	if err != nil {
		pr.Warning("为数据库 %s 配置健康检查失败: %v", name, err)
	}

	// 为数据库配置连接池管理
	err = sm.configureMySQLPoolManagement(name, storage)
	if err != nil {
		pr.Warning("为数据库 %s 配置连接池管理失败: %v", name, err)
	}

	return nil
}

// GetRedis 获取Redis存储
func (sm *StorageManager) GetRedis(name ...string) (storage.RedisStorage, error) {
	redisName := "default"
	if len(name) > 0 && name[0] != "" {
		redisName = name[0]
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	redis, ok := sm.redisStorages[redisName]
	if !ok {
		return nil, errors.Errorf("Redis %s 未找到", redisName)
	}

	return redis, nil
}

// RegisterRedis 注册Redis存储
func (sm *StorageManager) RegisterRedis(name string, storage storage.RedisStorage) error {
	if name == "" {
		return errors.New("Redis名称不能为空")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.redisStorages[name]; exists {
		return errors.Errorf("Redis %s 已存在", name)
	}

	// 注册Redis
	sm.redisStorages[name] = storage
	pr.Info("Redis %s 已注册", name)

	// 为Redis配置健康检查
	err := sm.configureRedisHealthCheck(name, storage)
	if err != nil {
		pr.Warning("为Redis %s 配置健康检查失败: %v", name, err)
	}

	// 为Redis配置连接池管理
	err = sm.configureRedisPoolManagement(name, storage)
	if err != nil {
		pr.Warning("为Redis %s 配置连接池管理失败: %v", name, err)
	}

	return nil
}

// GetES 获取ES存储
func (sm *StorageManager) GetES(name ...string) (storage.ESStorage, error) {
	esName := "default"
	if len(name) > 0 && name[0] != "" {
		esName = name[0]
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	es, ok := sm.esStorages[esName]
	if !ok {
		return nil, errors.Errorf("Elasticsearch %s 未找到", esName)
	}

	return es, nil
}

// RegisterES 注册ES存储
func (sm *StorageManager) RegisterES(name string, storage storage.ESStorage) error {
	if name == "" {
		return errors.New("ES名称不能为空")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.esStorages[name]; exists {
		return errors.Errorf("Elasticsearch %s 已存在", name)
	}

	// 注册ES
	sm.esStorages[name] = storage
	pr.Info("Elasticsearch %s 已注册", name)

	// 为ES配置健康检查
	err := sm.configureESHealthCheck(name, storage)
	if err != nil {
		pr.Warning("为Elasticsearch %s 配置健康检查失败: %v", name, err)
	}

	// 为ES配置连接池管理
	err = sm.configureESPoolManagement(name, storage)
	if err != nil {
		pr.Warning("为Elasticsearch %s 配置连接池管理失败: %v", name, err)
	}

	return nil
}

// GetAllStorages 获取所有存储
func (sm *StorageManager) GetAllStorages() map[string]storage.Storage {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]storage.Storage)

	// 添加数据库存储
	for name, db := range sm.dbStorages {
		result[name] = db
	}

	// 添加Redis存储
	for name, redis := range sm.redisStorages {
		result[name] = redis
	}

	// 添加ES存储
	for name, es := range sm.esStorages {
		result[name] = es
	}

	return result
}

// HealthCheck 健康检查
func (sm *StorageManager) HealthCheck(ctx context.Context) map[string]*storage.HealthStatus {
	// 使用原生接口进行基本健康检查
	result := make(map[string]*storage.HealthStatus)

	// 并发执行健康检查
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 检查所有存储
	for name, s := range sm.GetAllStorages() {
		wg.Add(1)
		go func(n string, storage storage.Storage) {
			defer wg.Done()

			// 使用带超时的上下文
			pingCtx, cancel := context.WithTimeout(ctx, sm.healthCheckTimeout)
			defer cancel()

			// 执行Ping操作
			_ = storage.Ping(pingCtx)

			// 获取健康状态
			status := storage.GetHealthStatus()

			mu.Lock()
			result[n] = status
			mu.Unlock()
		}(name, s)
	}

	wg.Wait()

	// 补充增强型健康检查信息
	sm.enrichHealthStatus(result)

	return result
}

// CloseAll 关闭所有存储连接
func (sm *StorageManager) CloseAll(ctx context.Context) error {
	// 停止健康检查服务
	if sm.healthService != nil {
		err := sm.healthService.Stop()
		if err != nil {
			pr.Warning("停止健康检查服务失败: %v", err)
		}
	}

	// 关闭所有连接池
	sm.closeAllPools()

	// 关闭所有存储连接
	var errs []error

	// 关闭所有数据库连接
	for name, db := range sm.dbStorages {
		if err := db.Disconnect(ctx); err != nil {
			errs = append(errs, errors.Wrapf(err, "关闭数据库 %s 失败", name))
		}
	}

	// 关闭所有Redis连接
	for name, redis := range sm.redisStorages {
		if err := redis.Disconnect(ctx); err != nil {
			errs = append(errs, errors.Wrapf(err, "关闭Redis %s 失败", name))
		}
	}

	// 关闭所有ES连接
	for name, es := range sm.esStorages {
		if err := es.Disconnect(ctx); err != nil {
			errs = append(errs, errors.Wrapf(err, "关闭Elasticsearch %s 失败", name))
		}
	}

	if len(errs) > 0 {
		return errors.Errorf("关闭存储连接出现 %d 个错误", len(errs))
	}

	return nil
}

// StartHealthCheck 启动健康检查服务
func (sm *StorageManager) StartHealthCheck() error {
	if sm.healthService == nil {
		return errors.New("健康检查服务未初始化")
	}

	return sm.healthService.Start()
}

// StopHealthCheck 停止健康检查服务
func (sm *StorageManager) StopHealthCheck() error {
	if sm.healthService == nil {
		return errors.New("健康检查服务未初始化")
	}

	return sm.healthService.Stop()
}

// GetHealthSummary 获取健康状态摘要
func (sm *StorageManager) GetHealthSummary() health.HealthSummary {
	if sm.healthService == nil {
		return health.HealthSummary{
			OverallStatus:   health.StatusUnhealthy,
			CheckedServices: 0,
		}
	}

	return sm.healthService.GetHealthSummary()
}

// GetActiveAlerts 获取活跃告警
func (sm *StorageManager) GetActiveAlerts() []health.Alert {
	if sm.healthService == nil {
		return nil
	}

	return sm.healthService.GetActiveAlerts()
}

// 以下是私有辅助方法

// configureMySQLHealthCheck 配置MySQL健康检查
func (sm *StorageManager) configureMySQLHealthCheck(name string, storage storage.DBStorage) error {
	// 获取底层的数据库连接
	db, err := storage.GetDB().DB()
	if err != nil {
		return errors.Wrap(err, "获取数据库连接失败")
	}

	if db == nil {
		return errors.New("无法获取数据库连接")
	}

	// 创建默认的MySQL检查设置
	settings := health.DefaultMySQLCheckSettings()

	// 注册MySQL健康检查器
	return sm.healthService.RegisterMySQLChecker(name, db, &settings)
}

// configureRedisHealthCheck 配置Redis健康检查
func (sm *StorageManager) configureRedisHealthCheck(name string, storage storage.RedisStorage) error {
	// 获取底层的Redis客户端
	client := storage.GetClient()
	if client == nil {
		return errors.New("无法获取Redis客户端")
	}

	// 创建默认的Redis检查设置
	settings := health.DefaultRedisCheckSettings()

	// 注册Redis健康检查器
	return sm.healthService.RegisterRedisChecker(name, client, &settings)
}

// configureESHealthCheck 配置Elasticsearch健康检查
func (sm *StorageManager) configureESHealthCheck(name string, storage storage.ESStorage) error {
	// 获取底层的ES客户端
	client := storage.GetClient()
	if client == nil {
		return errors.New("无法获取Elasticsearch客户端")
	}

	// 创建默认的ES检查设置
	settings := health.DefaultESCheckSettings()

	// 注册ES健康检查器
	return sm.healthService.RegisterESChecker(name, client, &settings)
}

// configureMySQLPoolManagement 配置MySQL连接池管理
func (sm *StorageManager) configureMySQLPoolManagement(name string, storage storage.DBStorage) error {
	// 获取底层的数据库连接
	db := storage.GetDB()
	if db == nil {
		return errors.New("无法获取数据库连接")
	}

	// 创建默认的MySQL连接池配置
	poolConfig := mysql.DefaultPoolConfig()

	// 创建连接池管理器
	pool, err := mysql.NewPool(db, poolConfig)
	if err != nil {
		return errors.Wrap(err, "创建MySQL连接池失败")
	}

	// 保存连接池引用
	sm.mysqlPools[name] = pool

	return nil
}

// configureRedisPoolManagement 配置Redis连接池管理
func (sm *StorageManager) configureRedisPoolManagement(name string, storage storage.RedisStorage) error {
	// 获取底层的Redis客户端
	client := storage.GetClient()
	if client == nil {
		return errors.New("无法获取Redis客户端")
	}

	// 创建默认的Redis连接池配置
	poolConfig := redis.DefaultPoolConfig()

	// 创建连接池管理器
	pool, err := redis.NewPool(client, poolConfig)
	if err != nil {
		return errors.Wrap(err, "创建Redis连接池失败")
	}

	// 保存连接池引用
	sm.redisPools[name] = pool

	return nil
}

// configureESPoolManagement 配置Elasticsearch连接池管理
func (sm *StorageManager) configureESPoolManagement(name string, storage storage.ESStorage) error {
	// 获取底层的ES客户端
	client := storage.GetClient()
	if client == nil {
		return errors.New("无法获取Elasticsearch客户端")
	}

	// 创建默认的ES连接池配置
	poolConfig := elasticsearch.DefaultPoolConfig()

	// 创建连接池管理器
	pool, err := elasticsearch.NewPool(client, poolConfig)
	if err != nil {
		return errors.Wrap(err, "创建Elasticsearch连接池失败")
	}

	// 保存连接池引用
	sm.esPools[name] = pool

	return nil
}

// closeAllPools 关闭所有连接池
func (sm *StorageManager) closeAllPools() {
	// 关闭MySQL连接池
	for name, pool := range sm.mysqlPools {
		if err := pool.Close(); err != nil {
			pr.Warning("关闭MySQL连接池 %s 失败: %v", name, err)
		}
	}

	// 关闭Redis连接池
	for name, pool := range sm.redisPools {
		if err := pool.Close(); err != nil {
			pr.Warning("关闭Redis连接池 %s 失败: %v", name, err)
		}
	}

	// 关闭ES连接池
	for name, pool := range sm.esPools {
		if err := pool.Close(); err != nil {
			pr.Warning("关闭Elasticsearch连接池 %s 失败: %v", name, err)
		}
	}
}

// enrichHealthStatus 丰富健康状态信息
func (sm *StorageManager) enrichHealthStatus(basicStatus map[string]*storage.HealthStatus) {
	// 获取详细的健康检查结果
	detailedStatus := sm.healthService.CheckAllNow(context.Background())

	// 整合结果
	for name, status := range detailedStatus {
		if basicStatus[name] != nil {
			// 将详细状态转换为基础状态
			basicStatus[name].ErrorMsg = status.Details["error"]

			// 更新状态
			switch status.Status {
			case health.StatusHealthy:
				basicStatus[name].State = storage.StorageStateConnected
				basicStatus[name].FailCount = 0
			case health.StatusDegraded:
				// 降级状态仍然是已连接，但可能有警告
				basicStatus[name].State = storage.StorageStateConnected
			case health.StatusUnhealthy:
				basicStatus[name].State = storage.StorageStateError
				basicStatus[name].FailCount++
			}

			// 更新延迟
			basicStatus[name].Latency = int64(status.Latency / time.Millisecond)
		}
	}
}

// setupDefaultAlertRules 设置默认告警规则
func (sm *StorageManager) setupDefaultAlertRules() {
	// MySQL 告警规则
	sm.healthService.RegisterAlertRule(health.AlertRule{
		ID:           "mysql_connection_error",
		Name:         "MySQL连接错误",
		Description:  "MySQL数据库连接失败或连接池异常",
		Level:        health.AlertLevelError,
		ServiceTypes: []string{"mysql"},
		Condition:    "status == 'unhealthy'",
		Message:      "MySQL数据库 {{.ServiceName}} 连接异常: {{.Details.error}}",
		Enabled:      true,
	})

	// Redis 告警规则
	sm.healthService.RegisterAlertRule(health.AlertRule{
		ID:           "redis_memory_warning",
		Name:         "Redis内存使用警告",
		Description:  "Redis内存使用率超过阈值",
		Level:        health.AlertLevelWarning,
		ServiceTypes: []string{"redis"},
		Condition:    "status == 'degraded' && contains(details.keys(), 'memory_usage')",
		Message:      "Redis服务 {{.ServiceName}} 内存使用率高: {{.Details.memory_usage}}",
		Enabled:      true,
	})

	// ES 告警规则
	sm.healthService.RegisterAlertRule(health.AlertRule{
		ID:           "es_cluster_warning",
		Name:         "ES集群状态警告",
		Description:  "Elasticsearch集群状态不健康",
		Level:        health.AlertLevelWarning,
		ServiceTypes: []string{"elasticsearch"},
		Condition:    "status != 'healthy'",
		Message:      "Elasticsearch集群 {{.ServiceName}} 状态异常: {{.Details.cluster_status}}",
		Enabled:      true,
	})
}
