# 健康检查与告警模块

## 概述

健康检查与告警模块(`pkg/storage/health`)提供了对各种存储服务的健康状态监控、状态评估和告警通知功能。该模块可以定期检查存储服务的健康状态，发现异常时触发告警，帮助快速发现和解决问题。

## 核心组件

### 1. 健康检查器 (Checker)

不同类型存储服务的健康检查实现，负责评估服务的健康状态。

#### MySQL健康检查器 (mysql.go)

```go
// MySQLChecker MySQL健康检查器
type MySQLChecker struct {
    db       *sql.DB
    settings *MySQLCheckSettings
    // 其他字段...
}

// 主要方法:
// - Check: 执行健康检查
// - checkConnection: 检查数据库连接
// - checkReplication: 检查复制状态
// - checkSlowQueries: 检查慢查询
```

#### Redis健康检查器 (redis.go)

```go
// RedisChecker Redis健康检查器
type RedisChecker struct {
    client   *redis.Client
    settings *RedisCheckSettings
    // 其他字段...
}

// 主要方法:
// - Check: 执行健康检查
// - checkMemory: 检查内存使用情况
// - checkClients: 检查客户端连接
// - checkReplication: 检查复制状态
```

#### Elasticsearch健康检查器 (elasticsearch.go)

```go
// ESChecker Elasticsearch健康检查器
type ESChecker struct {
    client   *elasticsearch.Client
    settings *ESCheckSettings
    // 其他字段...
}

// 主要方法:
// - Check: 执行健康检查
// - checkClusterHealth: 检查集群健康状态
// - checkNodeStats: 检查节点统计信息
// - checkIndices: 检查索引状态
```

### 2. 健康服务 (manager.go)

管理多个健康检查器，提供统一的健康检查调度和状态汇总。

```go
// HealthService 健康检查服务
type HealthService struct {
    checkers       map[string]Checker
    checkResults   map[string]CheckResult
    checkInterval  time.Duration
    alertManager   *AlertManager
    // 其他字段...
}

// 主要方法:
// - RegisterChecker: 注册健康检查器
// - UnregisterChecker: 注销健康检查器
// - Check: 检查特定服务的健康状态
// - CheckAll: 检查所有服务的健康状态
// - GetHealthSummary: 获取健康状态摘要
// - Start/Stop: 启动/停止定期健康检查
```

### 3. 告警管理器 (alert.go)

基于健康检查结果，根据告警规则触发告警，并通过不同的处理器发送通知。

```go
// AlertManager 告警管理器
type AlertManager struct {
    rules         []AlertRule
    handlers      map[string]AlertHandler
    activeAlerts  map[string]Alert
    // 其他字段...
}

// 主要方法:
// - RegisterRule: 注册告警规则
// - RegisterHandler: 注册告警处理器
// - HandleHealthStatus: 处理健康状态变化
// - EvaluateAlerts: 评估告警条件
// - GetActiveAlerts: 获取活跃告警
```

## 使用示例

### 1. 创建和配置健康检查器

```go
// MySQL健康检查器
mysqlSettings := health.DefaultMySQLCheckSettings()
mysqlSettings.ConnectTimeout = 3 * time.Second
mysqlSettings.CheckReplication = true
mysqlSettings.MaxSlowQueries = 5

mysqlChecker, err := health.NewMySQLChecker("main_db", db, &mysqlSettings)
if err != nil {
    log.Fatalf("创建MySQL健康检查器失败: %v", err)
}

// Redis健康检查器
redisSettings := health.DefaultRedisCheckSettings()
redisSettings.MemoryThreshold = 80 // 内存使用率超过80%触发警告
redisSettings.MaxClients = 1000
redisSettings.CheckReplication = true

redisChecker, err := health.NewRedisChecker("cache", redisClient, &redisSettings)
if err != nil {
    log.Fatalf("创建Redis健康检查器失败: %v", err)
}

// Elasticsearch健康检查器
esSettings := health.DefaultESCheckSettings()
esSettings.ClusterHealthTimeout = 5 * time.Second
esSettings.CheckNodeStats = true
esSettings.JVMHeapThreshold = 75 // JVM堆使用率超过75%触发警告

esChecker, err := health.NewESChecker("search", esClient, &esSettings)
if err != nil {
    log.Fatalf("创建Elasticsearch健康检查器失败: %v", err)
}
```

### 2. 创建健康服务和告警管理器

```go
// 创建告警管理器
alertManager := health.NewAlertManager()

// 注册告警处理器
logHandler := health.NewLogAlertHandler("log_handler")
alertManager.RegisterHandler(logHandler)

// 配置文件告警处理器
fileConfig := health.FileConfig{
    FilePath:     "./alerts.log",
    Format:       "json",
    RotateSize:   10 * 1024 * 1024, // 10MB
    MaxFiles:     5,
    Permissions:  0644,
    LevelFilters: []health.AlertLevel{health.AlertLevelWarning, health.AlertLevelError},
}
fileHandler, _ := health.NewFileHandler("file_handler", fileConfig)
alertManager.RegisterHandler(fileHandler)

// 配置WebHook告警处理器
webhookConfig := health.WebhookConfig{
    URL:          "http://example.com/alerts",
    Method:       "POST",
    Headers:      map[string]string{"Content-Type": "application/json"},
    Timeout:      5 * time.Second,
    RetryCount:   3,
    RetryDelay:   2 * time.Second,
    LevelFilters: []health.AlertLevel{health.AlertLevelError, health.AlertLevelCritical},
}
webhookHandler, _ := health.NewWebhookHandler("webhook_handler", webhookConfig)
alertManager.RegisterHandler(webhookHandler)

// 创建健康服务
healthService := health.NewHealthService(
    health.ServiceWithAlertManager(alertManager),
    health.ServiceWithCheckInterval(60 * time.Second), // 每60秒检查一次
    health.ServiceWithTimeout(5 * time.Second),        // 每次检查超时时间
)

// 注册健康检查器
healthService.RegisterChecker(mysqlChecker)
healthService.RegisterChecker(redisChecker)
healthService.RegisterChecker(esChecker)
```

### 3. 配置告警规则

```go
// MySQL连接错误告警
alertManager.RegisterRule(health.AlertRule{
    ID:           "mysql_connection_error",
    Name:         "MySQL连接错误",
    Description:  "MySQL数据库连接失败",
    Level:        health.AlertLevelError,
    ServiceTypes: []string{"mysql"},
    Condition:    "status == 'unhealthy' && contains(details.keys(), 'connection_error')",
    Message:      "MySQL数据库 {{.ServiceName}} 连接异常: {{.Details.connection_error}}",
    Enabled:      true,
})

// Redis内存使用警告
alertManager.RegisterRule(health.AlertRule{
    ID:           "redis_memory_warning",
    Name:         "Redis内存使用警告",
    Description:  "Redis内存使用率超过阈值",
    Level:        health.AlertLevelWarning,
    ServiceTypes: []string{"redis"},
    Condition:    "status == 'degraded' && contains(details.keys(), 'memory_usage') && parseFloat(details.memory_usage) > 80",
    Message:      "Redis服务 {{.ServiceName}} 内存使用率高: {{.Details.memory_usage}}%",
    Enabled:      true,
})

// ES集群状态警告
alertManager.RegisterRule(health.AlertRule{
    ID:           "es_cluster_yellow",
    Name:         "ES集群状态警告",
    Description:  "Elasticsearch集群状态为黄色",
    Level:        health.AlertLevelWarning,
    ServiceTypes: []string{"elasticsearch"},
    Condition:    "status == 'degraded' && details.cluster_status == 'yellow'",
    Message:      "Elasticsearch集群 {{.ServiceName}} 状态为黄色，部分分片未分配",
    Enabled:      true,
})

// ES集群状态错误
alertManager.RegisterRule(health.AlertRule{
    ID:           "es_cluster_red",
    Name:         "ES集群状态错误",
    Description:  "Elasticsearch集群状态为红色",
    Level:        health.AlertLevelError,
    ServiceTypes: []string{"elasticsearch"},
    Condition:    "status == 'unhealthy' && details.cluster_status == 'red'",
    Message:      "Elasticsearch集群 {{.ServiceName}} 状态为红色，存在不可用的主分片",
    Enabled:      true,
})
```

### 4. 启动健康检查服务

```go
// 启动健康检查服务
if err := healthService.Start(); err != nil {
    log.Fatalf("启动健康检查服务失败: %v", err)
}

// 在程序退出时停止健康检查服务
defer func() {
    if err := healthService.Stop(); err != nil {
        log.Printf("停止健康检查服务失败: %v", err)
    }
}()

// 获取健康状态摘要
summary := healthService.GetHealthSummary()
log.Printf("健康状态摘要: 总体=%s, 已检查=%d, 健康=%d, 降级=%d, 不健康=%d",
    summary.OverallStatus,
    summary.CheckedServices,
    summary.HealthyServices,
    summary.DegradedServices,
    summary.UnhealthyServices)

// 获取活跃告警
activeAlerts := alertManager.GetActiveAlerts()
for _, alert := range activeAlerts {
    log.Printf("活跃告警: [%s] %s - %s", alert.Level, alert.Name, alert.Message)
}
```

### 5. 手动触发健康检查

```go
// 检查特定服务
ctx := context.Background()
result, err := healthService.Check(ctx, "main_db")
if err != nil {
    log.Printf("检查 main_db 失败: %v", err)
} else {
    log.Printf("main_db 健康状态: %s, 延迟: %v", result.Status, result.Latency)
    for key, value := range result.Details {
        log.Printf("  %s: %v", key, value)
    }
}

// 检查所有服务
results := healthService.CheckAllNow(ctx)
for name, result := range results {
    log.Printf("服务 %s 健康状态: %s", name, result.Status)
}
```

## 健康状态定义

健康检查模块定义了三种健康状态：

1. **健康 (StatusHealthy)**
   - 所有核心功能正常运行
   - 性能指标在预期范围内
   - 没有明显的错误或异常

2. **降级 (StatusDegraded)**
   - 核心功能可用，但有轻微问题
   - 性能指标接近警戒线
   - 存在非关键性错误或警告
   - 例如：Redis内存使用率高、ES集群状态为黄色

3. **不健康 (StatusUnhealthy)**
   - 核心功能不可用
   - 性能指标严重超标
   - 存在严重错误
   - 例如：连接失败、ES集群状态为红色

## 告警级别

告警管理器支持四种告警级别：

1. **信息 (AlertLevelInfo)**
   - 纯信息性通知，不表示问题
   - 例如：服务重启、配置更改

2. **警告 (AlertLevelWarning)**
   - 表示潜在问题或性能降级
   - 需要关注但不需要立即处理
   - 例如：内存使用率高、连接数接近上限

3. **错误 (AlertLevelError)**
   - 表示严重问题，影响服务正常运行
   - 需要及时处理
   - 例如：连接失败、查询超时

4. **严重 (AlertLevelCritical)**
   - 表示紧急问题，服务完全不可用
   - 需要立即处理
   - 例如：集群崩溃、数据损坏

## 告警处理器

健康检查模块提供了多种告警处理器：

### 1. 日志告警处理器 (LogAlertHandler)

将告警信息记录到应用日志中。

```go
handler := health.NewLogAlertHandler("log_handler")
```

### 2. 文件告警处理器 (FileHandler)

将告警信息写入独立的文件，支持日志轮转。

```go
config := health.FileConfig{
    FilePath:     "./alerts.log",
    Format:       "json",  // 支持text、json格式
    RotateSize:   10 * 1024 * 1024, // 10MB
    MaxFiles:     5,
    Permissions:  0644,
    LevelFilters: []health.AlertLevel{health.AlertLevelWarning, health.AlertLevelError},
}
handler, err := health.NewFileHandler("file_handler", config)
```

### 3. WebHook告警处理器 (WebhookHandler)

将告警信息通过HTTP请求发送到指定的端点。

```go
config := health.WebhookConfig{
    URL:          "http://example.com/alerts",
    Method:       "POST",
    Headers:      map[string]string{"Content-Type": "application/json"},
    Timeout:      5 * time.Second,
    RetryCount:   3,
    RetryDelay:   2 * time.Second,
    LevelFilters: []health.AlertLevel{health.AlertLevelError, health.AlertLevelCritical},
}
handler, err := health.NewWebhookHandler("webhook_handler", config)
```

## 告警规则表达式

告警规则使用表达式来判断是否触发告警。表达式支持以下变量和函数：

### 变量

- `status`: 健康状态，可能的值为 "healthy"、"degraded"、"unhealthy"
- `serviceName`: 服务名称
- `serviceType`: 服务类型，如 "mysql"、"redis"、"elasticsearch"
- `details`: 健康检查详情，是一个映射，可通过 `details.key` 访问

### 函数

- `contains(arr, value)`: 判断数组是否包含值
- `startsWith(str, prefix)`: 判断字符串是否以前缀开头
- `endsWith(str, suffix)`: 判断字符串是否以后缀结尾
- `parseFloat(str)`: 将字符串解析为浮点数
- `parseInt(str)`: 将字符串解析为整数

### 示例表达式

```
// Redis内存使用率超过80%触发警告
status == 'degraded' && contains(details.keys(), 'memory_usage') && parseFloat(details.memory_usage) > 80

// MySQL连接错误
status == 'unhealthy' && contains(details.keys(), 'connection_error')

// ES集群状态为红色
status == 'unhealthy' && details.cluster_status == 'red'
```

## 配置参数

### 1. MySQL健康检查设置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| ConnectTimeout | time.Duration | 连接超时 | 3s |
| QueryTimeout | time.Duration | 查询超时 | 2s |
| CheckConnection | bool | 是否检查连接 | true |
| CheckMaxConnections | bool | 是否检查最大连接数 | true |
| MaxConnectionsThreshold | float64 | 最大连接数使用率阈值 | 80% |
| CheckReplication | bool | 是否检查复制状态 | false |
| CheckSlowQueries | bool | 是否检查慢查询 | true |
| MaxSlowQueries | int | 慢查询数量阈值 | 10 |
| TestQuery | string | 测试查询语句 | "SELECT 1" |

### 2. Redis健康检查设置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| PingTimeout | time.Duration | Ping超时 | a3s |
| CheckMemory | bool | 是否检查内存 | true |
| MemoryThreshold | float64 | 内存使用率阈值 | 75% |
| CheckClients | bool | 是否检查客户端连接 | true |
| MaxClients | int | 最大客户端连接数 | 0 (无限制) |
| CheckReplication | bool | 是否检查复制状态 | false |
| CheckCommandTime | bool | 是否检查命令执行时间 | true |
| MaxCommandTime | time.Duration | 最大命令执行时间 | 100ms |

### 3. Elasticsearch健康检查设置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| ClusterHealthTimeout | time.Duration | 集群健康检查超时 | 5s |
| CheckClusterHealth | bool | 是否检查集群健康 | true |
| CheckNodeStats | bool | 是否检查节点统计 | true |
| CheckIndices | bool | 是否检查索引状态 | true |
| ActiveShardsThreshold | float64 | 活跃分片百分比阈值 | 90% |
| JVMHeapThreshold | float64 | JVM堆使用率阈值 | 80% |
| DiskWatermarkThreshold | float64 | 磁盘使用率阈值 | 85% |
| TestSearch | bool | 是否执行测试搜索 | false |
| TestIndex | string | 测试搜索的索引 | "" |

## 最佳实践

1. **合理设置检查间隔**
   - 根据服务重要性和资源消耗设置适当的检查间隔
   - 关键服务可设置较短的检查间隔
   - 避免过于频繁的检查导致额外负载

2. **优化检查项目**
   - 启用最相关的检查项，禁用不必要的检查
   - 为不同环境设置不同的阈值
   - 测试环境可放宽阈值，生产环境应更严格

3. **告警管理**
   - 将告警分级，避免告警风暴
   - 为不同级别的告警配置不同的处理方式
   - 实现告警抑制和聚合机制

4. **健康检查与自动恢复**
   - 将健康检查与自动重连机制结合
   - 对于可自动恢复的问题，实现自动修复流程
   - 记录修复行为和成功率 