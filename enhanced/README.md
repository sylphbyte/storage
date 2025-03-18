# 增强版存储管理器模块

## 概述

增强版存储管理器模块(`pkg/storage/enhanced`)是对标准存储管理器的功能扩展，整合了连接池、自动重连、健康检查和监控指标等功能，为应用提供强大且可靠的存储服务管理能力。该模块以标准存储接口为基础，无缝集成各种增强功能，使开发者能够更轻松地构建高可用、高性能的数据存储层。

## 核心组件

### 1. 增强版存储管理器 (manager.go)

增强版存储管理器整合了所有存储服务的管理，并提供统一的增强功能。

```go
// StorageManager 增强版存储管理器
type StorageManager struct {
    storages       map[string]storage.Storage
    pools          map[string]pool.ConnectionPool
    reconnectors   map[string]reconnect.Reconnector
    healthManager  *health.HealthManager
    alertManager   *health.AlertManager
    monitorManager *monitor.MonitorManager
    metricsService *metrics.MetricsService
    // 其他字段...
}

// NewStorageManager 创建增强版存储管理器
func NewStorageManager(options ...ManagerOption) (*StorageManager, error) {
    // ...
}
```

### 2. 增强版MySQL存储服务 (mysql.go)

具有增强功能的MySQL存储服务实现。

```go
// MySQLStorage 增强版MySQL存储
type MySQLStorage struct {
    storage.MySQLStorage
    pool        *pool.MySQLPool
    reconnector *reconnect.MySQLReconnector
    // 其他字段...
}

// NewMySQLStorage 创建增强版MySQL存储
func NewMySQLStorage(config storage.MySQLConfig, options ...MySQLOption) (*MySQLStorage, error) {
    // ...
}
```

### 3. 增强版Redis存储服务 (redis.go)

具有增强功能的Redis存储服务实现。

```go
// RedisStorage 增强版Redis存储
type RedisStorage struct {
    storage.RedisStorage
    pool        *pool.RedisPool
    reconnector *reconnect.RedisReconnector
    // 其他字段...
}

// NewRedisStorage 创建增强版Redis存储
func NewRedisStorage(config storage.RedisConfig, options ...RedisOption) (*RedisStorage, error) {
    // ...
}
```

### 4. 增强版Elasticsearch存储服务 (elasticsearch.go)

具有增强功能的Elasticsearch存储服务实现。

```go
// ESStorage 增强版Elasticsearch存储
type ESStorage struct {
    storage.ESStorage
    pool        *pool.ESPool
    reconnector *reconnect.ESReconnector
    // 其他字段...
}

// NewESStorage 创建增强版Elasticsearch存储
func NewESStorage(config storage.ESConfig, options ...ESOption) (*ESStorage, error) {
    // ...
}
```

### 5. 配置工厂 (factory.go)

用于创建各种存储服务的配置工厂。

```go
// ConfigFactory 配置工厂
type ConfigFactory interface {
    // CreateMySQLConfig 创建MySQL配置
    CreateMySQLConfig(name string, dsn string, options ...MySQLConfigOption) storage.MySQLConfig
    
    // CreateRedisConfig 创建Redis配置
    CreateRedisConfig(name string, addr string, options ...RedisConfigOption) storage.RedisConfig
    
    // CreateESConfig 创建Elasticsearch配置
    CreateESConfig(name string, addresses []string, options ...ESConfigOption) storage.ESConfig
}
```

## 功能特性

### 1. 连接池管理

自动为每个存储服务配置连接池，优化连接资源使用。

```go
// 获取MySQL连接池配置
func (m *StorageManager) GetMySQLPoolConfig(name string) pool.MySQLPoolConfig {
    // ...
}

// 设置MySQL连接池配置
func (m *StorageManager) SetMySQLPoolConfig(name string, config pool.MySQLPoolConfig) error {
    // ...
}

// 获取连接池统计信息
func (m *StorageManager) GetPoolStats(name string) (pool.PoolStats, error) {
    // ...
}
```

### 2. 自动重连机制

监控连接状态，自动处理断连和重连。

```go
// 获取重连器状态
func (m *StorageManager) GetReconnectorStatus(name string) (reconnect.Status, error) {
    // ...
}

// 设置重连回调
func (m *StorageManager) SetReconnectCallback(name string, callback func(error)) error {
    // ...
}

// 手动触发重连
func (m *StorageManager) TriggerReconnect(name string) error {
    // ...
}
```

### 3. 健康检查

内置健康检查功能，自动监控存储服务的健康状态。

```go
// 获取健康状态
func (m *StorageManager) GetHealthStatus(name string) (health.Status, error) {
    // ...
}

// 获取健康摘要
func (m *StorageManager) GetHealthSummary() health.HealthSummary {
    // ...
}

// 设置健康状态变更回调
func (m *StorageManager) SetHealthStatusCallback(callback func(string, health.Status, health.Status)) {
    // ...
}
```

### 4. 告警管理

基于健康状态自动触发告警，并通过多种渠道发送通知。

```go
// 获取活跃告警
func (m *StorageManager) GetActiveAlerts() []health.Alert {
    // ...
}

// 添加告警规则
func (m *StorageManager) AddAlertRule(rule health.AlertRule) error {
    // ...
}

// 注册告警处理器
func (m *StorageManager) RegisterAlertHandler(handler health.AlertHandler) error {
    // ...
}
```

### 5. 性能监控

收集存储服务的性能指标，用于分析和优化。

```go
// 获取监控快照
func (m *StorageManager) GetMonitorSnapshot(name string) (monitor.Snapshot, error) {
    // ...
}

// 启用监控
func (m *StorageManager) EnableMonitoring(name string, interval time.Duration) error {
    // ...
}

// 禁用监控
func (m *StorageManager) DisableMonitoring(name string) error {
    // ...
}
```

### 6. 指标收集

提供详细的性能指标收集和处理功能。

```go
// 获取指标数据
func (m *StorageManager) GetMetrics(name string) ([]metrics.Metric, error) {
    // ...
}

// 添加指标处理器
func (m *StorageManager) AddMetricsProcessor(processor metrics.MetricsProcessor) error {
    // ...
}

// 导出指标
func (m *StorageManager) ExportMetrics(exporter string) error {
    // ...
}
```

## 使用示例

### 1. 创建增强版存储管理器

```go
// 创建增强版存储管理器
manager, err := enhanced.NewStorageManager(
    enhanced.WithHealthCheck(true),
    enhanced.WithHealthCheckInterval(30*time.Second),
    enhanced.WithAlertManager(true),
    enhanced.WithMonitoring(true),
    enhanced.WithMetricsCollection(true),
    enhanced.WithLogging(true),
)
if err != nil {
    log.Fatalf("创建增强版存储管理器失败: %v", err)
}
defer manager.Close()
```

### 2. 注册MySQL存储服务

```go
// 创建MySQL存储配置
mysqlConfig := manager.ConfigFactory().CreateMySQLConfig(
    "main_db",
    "user:password@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local",
    enhanced.WithMySQLMaxOpenConns(20),
    enhanced.WithMySQLMaxIdleConns(10),
    enhanced.WithMySQLConnMaxLifetime(time.Hour),
)

// 注册MySQL存储服务
if err := manager.RegisterMySQL(mysqlConfig); err != nil {
    log.Fatalf("注册MySQL存储失败: %v", err)
}

// 获取MySQL存储实例
mysql, err := manager.GetMySQL("main_db")
if err != nil {
    log.Fatalf("获取MySQL存储失败: %v", err)
}

// 使用MySQL存储服务
db := mysql.GetDB()
rows, err := db.QueryContext(context.Background(), "SELECT * FROM users LIMIT 10")
// 处理查询结果...
```

### 3. 注册Redis存储服务

```go
// 创建Redis存储配置
redisConfig := manager.ConfigFactory().CreateRedisConfig(
    "cache",
    "localhost:6379",
    enhanced.WithRedisPassword("password"),
    enhanced.WithRedisDB(0),
    enhanced.WithRedisPoolSize(100),
)

// 注册Redis存储服务
if err := manager.RegisterRedis(redisConfig); err != nil {
    log.Fatalf("注册Redis存储失败: %v", err)
}

// 获取Redis存储实例
redis, err := manager.GetRedis("cache")
if err != nil {
    log.Fatalf("获取Redis存储失败: %v", err)
}

// 使用Redis存储服务
client := redis.GetClient()
val, err := client.Get(context.Background(), "key").Result()
// 处理结果...
```

### 4. 注册Elasticsearch存储服务

```go
// 创建Elasticsearch存储配置
esConfig := manager.ConfigFactory().CreateESConfig(
    "search",
    []string{"http://localhost:9200"},
    enhanced.WithESUsername("user"),
    enhanced.WithESPassword("password"),
    enhanced.WithESMaxRetries(3),
)

// 注册Elasticsearch存储服务
if err := manager.RegisterES(esConfig); err != nil {
    log.Fatalf("注册Elasticsearch存储失败: %v", err)
}

// 获取Elasticsearch存储实例
es, err := manager.GetES("search")
if err != nil {
    log.Fatalf("获取Elasticsearch存储失败: %v", err)
}

// 使用Elasticsearch存储服务
client := es.GetClient()
res, err := client.Search(
    client.Search.WithContext(context.Background()),
    client.Search.WithIndex("products"),
    client.Search.WithQuery(`{"match": {"name": "phone"}}`),
)
// 处理结果...
```

### 5. 使用健康检查和告警功能

```go
// 注册告警处理器
logHandler := health.NewLogAlertHandler("log")
manager.RegisterAlertHandler(logHandler)

webhookConfig := health.WebhookConfig{
    URL:          "http://example.com/alerts",
    Method:       "POST",
    Headers:      map[string]string{"Content-Type": "application/json"},
    Timeout:      5 * time.Second,
    RetryCount:   3,
    RetryDelay:   2 * time.Second,
    LevelFilters: []health.AlertLevel{health.AlertLevelError, health.AlertLevelCritical},
}
webhookHandler, _ := health.NewWebhookHandler("webhook", webhookConfig)
manager.RegisterAlertHandler(webhookHandler)

// 添加自定义告警规则
manager.AddAlertRule(health.AlertRule{
    ID:           "mysql_connection_error",
    Name:         "MySQL连接错误",
    Description:  "MySQL数据库连接失败",
    Level:        health.AlertLevelError,
    ServiceTypes: []string{"mysql"},
    Condition:    "status == 'unhealthy' && contains(details.keys(), 'connection_error')",
    Message:      "MySQL数据库 {{.ServiceName}} 连接异常: {{.Details.connection_error}}",
    Enabled:      true,
})

// 获取健康摘要
summary := manager.GetHealthSummary()
log.Printf("健康状态摘要: 总体=%s, 已检查=%d, 健康=%d, 降级=%d, 不健康=%d",
    summary.OverallStatus,
    summary.CheckedServices,
    summary.HealthyServices,
    summary.DegradedServices,
    summary.UnhealthyServices)

// 获取活跃告警
alerts := manager.GetActiveAlerts()
for _, alert := range alerts {
    log.Printf("活跃告警: [%s] %s - %s", alert.Level, alert.Name, alert.Message)
}

// 设置健康状态变更回调
manager.SetHealthStatusCallback(func(name string, oldStatus, newStatus health.Status) {
    log.Printf("服务 %s 健康状态变更: %s -> %s", name, oldStatus, newStatus)
    // 可以触发其他操作，如自动恢复或通知
})
```

### 6. 使用性能监控和指标功能

```go
// 启用MySQL监控
manager.EnableMonitoring("main_db", 30*time.Second)

// 获取监控快照
snapshot, err := manager.GetMonitorSnapshot("main_db")
if err != nil {
    log.Printf("获取监控快照失败: %v", err)
} else {
    metrics := snapshot.GetMetrics()
    log.Printf("当前最大连接数: %v", metrics["max_connections"])
    log.Printf("活跃连接数: %v", metrics["threads_connected"])
    log.Printf("等待连接数: %v", metrics["threads_waiting"])
    log.Printf("慢查询数: %v", metrics["slow_queries"])
}

// 获取性能指标
dbMetrics, err := manager.GetMetrics("main_db")
if err != nil {
    log.Printf("获取指标失败: %v", err)
} else {
    for _, metric := range dbMetrics {
        if metric.Name() == "mysql_connection_usage_percent" {
            log.Printf("MySQL连接使用率: %v%%", metric.Value())
            // 可以基于阈值触发自动扩容
            if v, ok := metric.Value().(float64); ok && v > 80 {
                manager.SetMySQLPoolConfig("main_db", pool.MySQLPoolConfig{
                    MaxOpenConns: 30, // 增加连接数
                    MaxIdleConns: 15,
                })
            }
        }
    }
}

// 导出指标到Prometheus
manager.ExportMetrics("prometheus")
```

### 7. 使用一键初始化功能

```go
// 从配置文件一键初始化所有存储服务
if err := manager.InitFromConfig("config/storage.yaml"); err != nil {
    log.Fatalf("初始化存储服务失败: %v", err)
}

// 或者使用代码一键初始化
storageConfigs := enhanced.StorageConfigs{
    MySQL: map[string]storage.MySQLConfig{
        "main_db": mysqlConfig,
        "log_db":  logDBConfig,
    },
    Redis: map[string]storage.RedisConfig{
        "cache":    cacheConfig,
        "sessions": sessionConfig,
    },
    ES: map[string]storage.ESConfig{
        "search": searchConfig,
        "logs":   logsConfig,
    },
}

if err := manager.InitFromConfigs(storageConfigs); err != nil {
    log.Fatalf("初始化存储服务失败: %v", err)
}
```

## 配置参数

### 1. 管理器配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| EnableHealthCheck | bool | 是否启用健康检查 | true |
| HealthCheckInterval | time.Duration | 健康检查间隔 | 30s |
| EnableAlertManager | bool | 是否启用告警管理 | true |
| EnableMonitoring | bool | 是否启用性能监控 | true |
| MonitoringInterval | time.Duration | 监控间隔 | 1m |
| EnableMetricsCollection | bool | 是否启用指标收集 | true |
| MetricsInterval | time.Duration | 指标收集间隔 | 1m |
| EnableLogging | bool | 是否启用日志 | true |
| LogLevel | string | 日志级别 | "info" |
| AutoRecovery | bool | 是否启用自动恢复 | true |
| GlobalLabels | map[string]string | 全局标签 | nil |
| ConfigPath | string | 配置文件路径 | "" |

### 2. MySQL存储配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| Name | string | 存储名称 | "" |
| DSN | string | 数据源名称 | "" |
| MaxOpenConns | int | 最大连接数 | 10 |
| MaxIdleConns | int | 最大空闲连接数 | 5 |
| ConnMaxLifetime | time.Duration | 连接最大生命周期 | 1h |
| ConnMaxIdleTime | time.Duration | 连接最大空闲时间 | 30m |
| EnableHealthCheck | bool | 是否启用健康检查 | true |
| EnableReconnect | bool | 是否启用自动重连 | true |
| ReconnectMaxRetries | int | 最大重试次数 | 10 |
| EnableMonitoring | bool | 是否启用监控 | true |
| Labels | map[string]string | 标签 | nil |

### 3. Redis存储配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| Name | string | 存储名称 | "" |
| Addr | string | 地址 | "localhost:6379" |
| Password | string | 密码 | "" |
| DB | int | 数据库编号 | 0 |
| PoolSize | int | 连接池大小 | 10 |
| MinIdleConns | int | 最小空闲连接数 | 0 |
| DialTimeout | time.Duration | 连接超时 | 5s |
| ReadTimeout | time.Duration | 读取超时 | 3s |
| WriteTimeout | time.Duration | 写入超时 | 3s |
| EnableHealthCheck | bool | 是否启用健康检查 | true |
| EnableReconnect | bool | 是否启用自动重连 | true |
| ReconnectMaxRetries | int | 最大重试次数 | 5 |
| EnableMonitoring | bool | 是否启用监控 | true |
| Labels | map[string]string | 标签 | nil |

### 4. Elasticsearch存储配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| Name | string | 存储名称 | "" |
| Addresses | []string | 地址列表 | [] |
| Username | string | 用户名 | "" |
| Password | string | 密码 | "" |
| CloudID | string | 云ID | "" |
| APIKey | string | API密钥 | "" |
| MaxRetries | int | 最大重试次数 | 3 |
| RetryStatuses | []int | 重试状态码 | [502, 503, 504] |
| EnableHealthCheck | bool | 是否启用健康检查 | true |
| EnableReconnect | bool | 是否启用自动重连 | true |
| ReconnectMaxRetries | int | 最大重试次数 | 15 |
| EnableMonitoring | bool | 是否启用监控 | true |
| Labels | map[string]string | 标签 | nil |

## 配置文件示例

在`config/storage.yaml`中定义所有存储服务：

```yaml
manager:
  enable_health_check: true
  health_check_interval: 30s
  enable_alert_manager: true
  enable_monitoring: true
  monitoring_interval: 1m
  enable_metrics_collection: true
  metrics_interval: 1m
  enable_logging: true
  log_level: info
  auto_recovery: true
  global_labels:
    environment: production
    application: my_app

mysql:
  main_db:
    dsn: "user:password@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
    max_open_conns: 20
    max_idle_conns: 10
    conn_max_lifetime: 1h
    enable_health_check: true
    enable_reconnect: true
    reconnect_max_retries: 10
    enable_monitoring: true
    labels:
      service: user_service
  
  log_db:
    dsn: "user:password@tcp(127.0.0.1:3306)/logs?charset=utf8mb4&parseTime=True&loc=Local"
    max_open_conns: 10
    max_idle_conns: 5
    conn_max_lifetime: 1h
    enable_health_check: true
    enable_reconnect: true
    reconnect_max_retries: 10
    enable_monitoring: true
    labels:
      service: log_service

redis:
  cache:
    addr: "localhost:6379"
    password: "password"
    db: 0
    pool_size: 100
    min_idle_conns: 10
    dial_timeout: 5s
    read_timeout: 3s
    write_timeout: 3s
    enable_health_check: true
    enable_reconnect: true
    reconnect_max_retries: 5
    enable_monitoring: true
    labels:
      service: cache_service
  
  sessions:
    addr: "localhost:6380"
    password: "password"
    db: 0
    pool_size: 50
    enable_health_check: true
    enable_reconnect: true
    reconnect_max_retries: 5
    enable_monitoring: true
    labels:
      service: session_service

elasticsearch:
  search:
    addresses:
      - "http://localhost:9200"
    username: "user"
    password: "password"
    max_retries: 3
    retry_statuses: [502, 503, 504]
    enable_health_check: true
    enable_reconnect: true
    reconnect_max_retries: 15
    enable_monitoring: true
    labels:
      service: search_service
  
  logs:
    addresses:
      - "http://localhost:9201"
    username: "user"
    password: "password"
    max_retries: 3
    enable_health_check: true
    enable_reconnect: true
    reconnect_max_retries: 15
    enable_monitoring: true
    labels:
      service: log_search_service
```

## 最佳实践

1. **统一管理存储服务**
   - 使用增强版存储管理器统一管理所有存储服务
   - 通过配置文件定义存储配置，便于环境切换
   - 将存储管理器作为应用的核心组件初始化

2. **合理配置连接池**
   - 根据实际负载调整连接池大小
   - 设置合适的空闲连接和最大生命周期
   - 定期监控连接池使用情况

3. **自动重连与故障恢复**
   - 启用自动重连功能，提高系统可用性
   - 设置合理的重试策略，避免频繁重试
   - 利用健康检查监控存储服务状态

4. **告警和监控**
   - 配置不同级别的告警规则
   - 设置多种告警通知渠道
   - 将监控数据导出到可视化平台

5. **性能优化**
   - 收集和分析性能指标
   - 根据使用模式优化配置
   - 实现自动调整机制

6. **资源管理**
   - 优雅关闭存储服务，释放资源
   - 避免连接泄漏和资源耗尽
   - 定期清理不再使用的连接

7. **安全性**
   - 安全存储敏感配置信息
   - 实现访问控制和权限管理
   - 对敏感操作进行审计 