# 存储服务模块手册

## 1. 概述

存储服务模块(`pkg/storage`)提供了一个统一的接口和实现，用于管理和操作多种存储服务，包括MySQL、Redis和Elasticsearch。该模块具有以下特点：

- **统一接口**: 为不同类型的存储提供统一的接口
- **自动重连**: 具备连接断开自动重连机制
- **连接池管理**: 优化的连接池实现，提高资源利用率
- **健康检查**: 全面的健康监控和告警功能
- **可扩展性**: 易于扩展支持新的存储类型

## 2. 模块结构

```
pkg/storage/
├── interface.go     - 定义核心接口和类型
├── storage.go       - 提供初始化和基本实现
├── elasticsearch/   - Elasticsearch客户端实现
├── mysql/           - MySQL客户端实现
├── redis/           - Redis客户端实现
├── pool/            - 连接池抽象和通用实现
├── reconnect/       - 重连机制实现
├── health/          - 健康检查相关组件
├── enhanced/        - 增强型存储管理器实现
├── monitor/         - 监控组件
└── metrics/         - 指标收集和导出
```

## 3. 核心接口

### 3.1 基础接口

存储服务模块定义了一系列接口，支持不同类型的存储服务：

```go
// Storage 基础存储接口
type Storage interface {
    // 获取存储类型
    GetType() StorageType
    
    // 连接到存储服务
    Connect(ctx context.Context) error
    
    // 断开连接
    Disconnect(ctx context.Context) error
    
    // 健康检查
    Ping(ctx context.Context) error
    
    // 获取健康状态
    GetHealthStatus() *HealthStatus
}

// DBStorage 数据库存储接口
type DBStorage interface {
    Storage
    // 获取数据库连接
    GetDB() *gorm.DB
    // 在事务中执行操作
    WithTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error
    // 其他数据库特定方法...
}

// RedisStorage Redis存储接口
type RedisStorage interface {
    Storage
    // 获取Redis客户端
    GetClient() *redis.Client
    // 在管道中执行操作
    WithPipeline(ctx context.Context, fn func(pipe redis.Pipeliner) error) error
    // 其他Redis特定方法...
}

// ESStorage Elasticsearch存储接口
type ESStorage interface {
    Storage
    // 获取ES客户端
    GetClient() *elasticsearch.Client
    // 索引操作
    IndexExists(ctx context.Context, index string) (bool, error)
    CreateIndex(ctx context.Context, index string, mapping string) error
    // 其他ES特定方法...
}
```

### 3.2 存储管理器

存储管理器(`StorageManager`)负责管理多个存储服务实例：

```go
// StorageManager 存储管理器接口
type StorageManager interface {
    // 获取数据库存储
    GetDB(name ...string) (DBStorage, error)
    
    // 注册数据库存储
    RegisterDB(name string, storage DBStorage) error
    
    // 获取Redis存储
    GetRedis(name ...string) (RedisStorage, error)
    
    // 注册Redis存储
    RegisterRedis(name string, storage RedisStorage) error
    
    // 获取ES存储
    GetES(name ...string) (ESStorage, error)
    
    // 注册ES存储
    RegisterES(name string, storage ESStorage) error
    
    // 获取所有存储
    GetAllStorages() map[string]Storage
    
    // 健康检查
    HealthCheck(ctx context.Context) map[string]*HealthStatus
    
    // 关闭所有存储连接
    CloseAll(ctx context.Context) error
}
```

## 4. 使用方法

### 4.1 初始化存储服务

```go
// 使用配置文件初始化存储服务
storageManager, err := storage.InitializeStorage(
    "config/storage.yaml",
    map[string]bool{"default": true},  // 启用的MySQL服务
    map[string]bool{"cache": true},    // 启用的Redis服务
    map[string]bool{"search": true},   // 启用的ES服务
)
if err != nil {
    log.Fatalf("初始化存储服务失败: %v", err)
}
```

### 4.2 使用MySQL

```go
// 获取MySQL连接
db, err := storageManager.GetDB("default")
if err != nil {
    log.Fatalf("获取MySQL连接失败: %v", err)
}

// 在事务中执行操作
err = db.WithTransaction(ctx, func(tx *gorm.DB) error {
    // 执行数据库操作
    result := tx.Create(&User{Name: "张三", Age: 25})
    return result.Error
})
```

### 4.3 使用Redis

```go
// 获取Redis客户端
redisClient, err := storageManager.GetRedis("cache")
if err != nil {
    log.Fatalf("获取Redis客户端失败: %v", err)
}

// 执行Redis操作
err = redisClient.GetClient().Set(ctx, "key", "value", time.Hour).Err()
if err != nil {
    log.Printf("设置Redis键值失败: %v", err)
}

// 在管道中执行操作
err = redisClient.WithPipeline(ctx, func(pipe redis.Pipeliner) error {
    pipe.Set(ctx, "key1", "value1", time.Hour)
    pipe.Set(ctx, "key2", "value2", time.Hour)
    _, err := pipe.Exec(ctx)
    return err
})
```

### 4.4 使用Elasticsearch

```go
// 获取ES客户端
esClient, err := storageManager.GetES("search")
if err != nil {
    log.Fatalf("获取ES客户端失败: %v", err)
}

// 检查索引是否存在
exists, err := esClient.IndexExists(ctx, "users")
if err != nil {
    log.Printf("检查索引失败: %v", err)
}

// 创建索引
if !exists {
    err = esClient.CreateIndex(ctx, "users", `{
        "settings": {
            "number_of_shards": 1,
            "number_of_replicas": 0
        },
        "mappings": {
            "properties": {
                "name": { "type": "text" },
                "age": { "type": "integer" }
            }
        }
    }`)
    if err != nil {
        log.Printf("创建索引失败: %v", err)
    }
}
```

## 5. 增强型存储管理器

增强型存储管理器(`enhanced.StorageManager`)提供了健康检查、告警和连接池管理等功能：

```go
// 创建增强型存储管理器
manager := enhanced.NewStorageManager(
    // 设置健康检查间隔
    enhanced.WithHealthCheckInterval(30*time.Second),
    // 设置健康检查超时
    enhanced.WithHealthCheckTimeout(3*time.Second),
    // 添加日志告警处理器
    enhanced.WithLogAlertHandler(),
    // 添加文件告警处理器
    enhanced.WithFileAlertHandler("./alerts.log"),
)

// 启动健康检查服务
if err := manager.StartHealthCheck(); err != nil {
    log.Fatalf("启动健康检查服务失败: %v", err)
}

// 获取健康状态摘要
summary := manager.GetHealthSummary()
fmt.Printf("存储服务总体状态: %s\n", summary.OverallStatus)
```

## 6. 连接池管理

存储服务模块提供了连接池管理功能，可以优化资源使用并提高性能：

### 6.1 MySQL连接池

```go
// 创建MySQL连接池配置
poolConfig := mysql.DefaultPoolConfig()
poolConfig.MaxIdleConns = 10
poolConfig.MaxOpenConns = 100
poolConfig.ConnMaxLifetime = 30 * time.Minute

// 创建连接池
pool, err := mysql.NewPool(db, poolConfig)
if err != nil {
    log.Fatalf("创建MySQL连接池失败: %v", err)
}

// 获取连接池统计信息
stats := pool.Stats()
fmt.Printf("当前连接数: %d, 空闲连接数: %d\n", stats.OpenConnections, stats.IdleConnections)
```

### 6.2 Redis连接池

```go
// 创建Redis连接池配置
poolConfig := redis.DefaultPoolConfig()
poolConfig.MaxActive = 100
poolConfig.MaxIdle = 20
poolConfig.IdleTimeout = 10 * time.Minute

// 创建连接池
pool, err := redis.NewPool(client, poolConfig)
if err != nil {
    log.Fatalf("创建Redis连接池失败: %v", err)
}
```

## 7. 健康检查和监控

存储服务模块提供了全面的健康检查和监控功能：

```go
// 创建MySQL健康检查器
mysqlSettings := health.DefaultMySQLCheckSettings()
mysqlSettings.ConnectTimeout = 3 * time.Second
mysqlSettings.QueryTimeout = 1 * time.Second

mysqlChecker, err := health.NewMySQLChecker("main_db", db, &mysqlSettings)
if err != nil {
    log.Fatalf("创建MySQL健康检查器失败: %v", err)
}

// 创建健康服务
healthService := health.NewHealthService(
    health.ServiceWithCheckInterval(60 * time.Second),
    health.ServiceWithTimeout(5 * time.Second),
)

// 注册健康检查器
healthService.RegisterChecker(mysqlChecker)

// 启动健康检查服务
healthService.Start()

// 获取健康状态摘要
summary := healthService.GetHealthSummary()
fmt.Printf("健康服务总体状态: %s\n", summary.OverallStatus)
```

## 8. 告警管理

存储服务的告警管理功能可以在服务状态异常时发送通知：

```go
// 创建告警管理器
alertManager := health.NewAlertManager()

// 注册告警处理器
logHandler := health.NewLogAlertHandler("storage_log_handler")
alertManager.RegisterHandler(logHandler)

// 配置WebHook告警处理器
webhookConfig := health.WebhookConfig{
    URL:      "http://example.com/alerts",
    Method:   "POST",
    Headers:  map[string]string{"Content-Type": "application/json"},
    Timeout:  5 * time.Second,
}
webhookHandler, _ := health.NewWebhookHandler("webhook_handler", webhookConfig)
alertManager.RegisterHandler(webhookHandler)

// 注册告警规则
alertManager.RegisterRule(health.AlertRule{
    ID:           "mysql_connection_error",
    Name:         "MySQL连接错误",
    Description:  "MySQL数据库连接失败",
    Level:        health.AlertLevelError,
    ServiceTypes: []string{"mysql"},
    Condition:    "status == 'unhealthy'",
    Message:      "MySQL数据库 {{.ServiceName}} 连接异常: {{.Details.error}}",
    Enabled:      true,
})
```

## 9. 配置示例

### 9.1 YAML配置文件 (storage.yaml)

```yaml
mysql_group:
  default:
    debug: false
    log_mode: 2
    host: localhost
    port: 3306
    username: root
    password: password
    database: app_db
    charset: utf8mb4
    max_idle_conn: 10
    max_open_conn: 100
    max_life_time: 3600

redis_group:
  cache:
    host: localhost
    port: 6379
    password: ""
    database: 0

es_group:
  search:
    addresses:
      - http://localhost:9200
    username: ""
    password: ""
    max_retries: 3
    retry_timeout: 5
```

## 10. 最佳实践

### 10.1 资源管理

- 始终在应用退出时关闭所有连接
- 合理配置连接池参数，避免资源浪费
- 使用超时上下文控制操作时间

```go
// 在程序退出时关闭所有连接
defer func() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    storageManager.CloseAll(ctx)
}()
```

### 10.2 错误处理

- 使用 `github.com/pkg/errors` 包装错误
- 为不同类型的错误提供明确的处理策略
- 记录详细的错误信息以便问题排查

```go
if err != nil {
    return errors.Wrap(err, "连接Redis失败")
}
```

### 10.3 监控与告警

- 定期检查存储服务的健康状态
- 配置适当的告警规则和阈值
- 使用多种通知渠道确保告警被及时处理

```go
// 定期检查健康状态
go func() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            status := storageManager.HealthCheck(ctx)
            // 处理状态...
        case <-ctx.Done():
            return
        }
    }
}()
```

## 11. 故障排查

常见问题及解决方法：

1. **连接失败**
   - 检查网络连接和防火墙设置
   - 验证连接参数（主机、端口、用户名、密码）
   - 查看服务器日志

2. **连接池耗尽**
   - 增加连接池大小
   - 检查是否有连接泄漏
   - 缩短连接生命周期

3. **查询超时**
   - 优化SQL语句和索引
   - 检查服务器负载
   - 增加查询超时设置

4. **健康检查失败**
   - 查看具体的错误消息
   - 验证服务器可访问性
   - 调整健康检查参数

## 12. 版本与兼容性

- Go 版本: 1.24.1+
- 依赖项:
  - github.com/go-redis/redis/v8 v8.11.5
  - github.com/elastic/go-elasticsearch/v7 v7.17.10
  - gorm.io/gorm v1.25.12
  - gorm.io/driver/mysql v1.5.7

---

文档更新日期: 2024-05-20 