# 存储自动重连模块

## 概述

自动重连模块(`pkg/storage/reconnect`)为存储服务提供断线自动重连功能，提高系统在网络波动或服务端临时不可用情况下的稳定性和可靠性。该模块支持MySQL、Redis和Elasticsearch等存储服务的自动重连，并提供灵活的重试策略和事件回调。

## 核心组件

### 1. 重连接口 (interface.go)

定义了所有重连实现必须满足的统一接口，提供统一的重连能力和事件处理。

```go
// Reconnector 重连接口
type Reconnector interface {
    // Start 启动重连监控
    Start() error
    
    // Stop 停止重连监控
    Stop() error
    
    // IsConnected 检查是否已连接
    IsConnected() bool
    
    // LastError 获取最后一次错误
    LastError() error
    
    // SetOnDisconnect 设置断开连接回调
    SetOnDisconnect(callback func(error))
    
    // SetOnReconnect 设置重连成功回调
    SetOnReconnect(callback func())
    
    // SetOnReconnectFailed 设置重连失败回调
    SetOnReconnectFailed(callback func(error, int))
    
    // SetMaxRetries 设置最大重试次数
    SetMaxRetries(maxRetries int)
    
    // SetRetryInterval 设置重试间隔函数
    SetRetryInterval(interval func(retryCount int) time.Duration)
}

// Status 连接状态
type Status int

const (
    // StatusDisconnected 断开连接
    StatusDisconnected Status = iota
    
    // StatusConnecting 连接中
    StatusConnecting
    
    // StatusConnected 已连接
    StatusConnected
    
    // StatusReconnecting 重连中
    StatusReconnecting
)

// ReconnectEvent 重连事件
type ReconnectEvent struct {
    // Status 事件状态
    Status Status
    
    // Error 相关错误信息
    Error error
    
    // RetryCount 重试次数
    RetryCount int
    
    // Timestamp 时间戳
    Timestamp time.Time
}
```

### 2. MySQL重连器 (mysql.go)

实现MySQL数据库的自动重连功能，监控连接状态，在断线时自动重连。

```go
// MySQLReconnector MySQL重连器
type MySQLReconnector struct {
    db        *sql.DB
    dsn       string
    status    Status
    lastError error
    // 其他字段...
}

// NewMySQLReconnector 创建MySQL重连器
func NewMySQLReconnector(db *sql.DB, dsn string, options ...MySQLReconnectOption) (*MySQLReconnector, error) {
    // ...
}
```

### 3. Redis重连器 (redis.go)

实现Redis服务的自动重连功能，监控连接状态，在断线时自动重连。

```go
// RedisReconnector Redis重连器
type RedisReconnector struct {
    client    *redis.Client
    options   *redis.Options
    status    Status
    lastError error
    // 其他字段...
}

// NewRedisReconnector 创建Redis重连器
func NewRedisReconnector(client *redis.Client, options *redis.Options, reconnectOptions ...RedisReconnectOption) (*RedisReconnector, error) {
    // ...
}
```

### 4. Elasticsearch重连器 (elasticsearch.go)

实现Elasticsearch服务的自动重连功能，监控连接状态，在节点不可用时自动重连。

```go
// ESReconnector Elasticsearch重连器
type ESReconnector struct {
    client    *elasticsearch.Client
    configs   []elasticsearch.Config
    status    Status
    lastError error
    // 其他字段...
}

// NewESReconnector 创建Elasticsearch重连器
func NewESReconnector(client *elasticsearch.Client, configs []elasticsearch.Config, options ...ESReconnectOption) (*ESReconnector, error) {
    // ...
}
```

### 5. 重连管理器 (manager.go)

管理多个重连器，提供统一的监控和控制接口。

```go
// ReconnectManager 重连管理器
type ReconnectManager struct {
    reconnectors map[string]Reconnector
    // 其他字段...
}

// NewReconnectManager 创建重连管理器
func NewReconnectManager() *ReconnectManager {
    // ...
}

// RegisterReconnector 注册重连器
func (m *ReconnectManager) RegisterReconnector(name string, reconnector Reconnector) {
    // ...
}

// GetReconnector 获取重连器
func (m *ReconnectManager) GetReconnector(name string) (Reconnector, bool) {
    // ...
}

// StartAll 启动所有重连器
func (m *ReconnectManager) StartAll() map[string]error {
    // ...
}

// StopAll 停止所有重连器
func (m *ReconnectManager) StopAll() map[string]error {
    // ...
}
```

## 使用示例

### 1. MySQL自动重连

```go
// 创建数据库连接
dsn := "user:password@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
db, err := sql.Open("mysql", dsn)
if err != nil {
    log.Fatalf("打开MySQL连接失败: %v", err)
}

// 创建重连器
reconnector, err := reconnect.NewMySQLReconnector(
    db, 
    dsn,
    reconnect.WithMySQLHealthCheck(true),
    reconnect.WithMySQLHealthCheckInterval(10*time.Second),
    reconnect.WithMySQLMaxRetries(10),
    reconnect.WithMySQLRetryInterval(func(retryCount int) time.Duration {
        // 指数退避策略
        return time.Duration(math.Pow(2, float64(retryCount))) * time.Second
    }),
)
if err != nil {
    log.Fatalf("创建MySQL重连器失败: %v", err)
}

// 设置回调函数
reconnector.SetOnDisconnect(func(err error) {
    log.Printf("MySQL断开连接: %v", err)
})

reconnector.SetOnReconnect(func() {
    log.Println("MySQL重连成功")
})

reconnector.SetOnReconnectFailed(func(err error, retryCount int) {
    log.Printf("MySQL重连失败(尝试%d次): %v", retryCount, err)
})

// 启动重连监控
if err := reconnector.Start(); err != nil {
    log.Fatalf("启动MySQL重连监控失败: %v", err)
}
defer reconnector.Stop()

// 使用数据库连接
// ...
```

### 2. Redis自动重连

```go
// 创建Redis客户端
options := &redis.Options{
    Addr:     "localhost:6379",
    Password: "password",
    DB:       0,
}
client := redis.NewClient(options)

// 创建重连器
reconnector, err := reconnect.NewRedisReconnector(
    client, 
    options,
    reconnect.WithRedisHealthCheck(true),
    reconnect.WithRedisHealthCheckInterval(5*time.Second),
    reconnect.WithRedisMaxRetries(5),
    reconnect.WithRedisRetryInterval(func(retryCount int) time.Duration {
        return time.Duration(retryCount+1) * 2 * time.Second
    }),
)
if err != nil {
    log.Fatalf("创建Redis重连器失败: %v", err)
}

// 设置回调函数
reconnector.SetOnDisconnect(func(err error) {
    log.Printf("Redis断开连接: %v", err)
    // 可以发送告警通知
})

reconnector.SetOnReconnect(func() {
    log.Println("Redis重连成功")
    // 可以恢复正常操作
})

// 启动重连监控
if err := reconnector.Start(); err != nil {
    log.Fatalf("启动Redis重连监控失败: %v", err)
}
defer reconnector.Stop()

// 使用Redis客户端
// ...
```

### 3. Elasticsearch自动重连

```go
// 创建Elasticsearch客户端
cfg := elasticsearch.Config{
    Addresses: []string{
        "http://localhost:9200",
        "http://localhost:9201",
    },
    Username: "user",
    Password: "password",
}
client, err := elasticsearch.NewClient(cfg)
if err != nil {
    log.Fatalf("创建Elasticsearch客户端失败: %v", err)
}

// 创建重连器
reconnector, err := reconnect.NewESReconnector(
    client, 
    []elasticsearch.Config{cfg},
    reconnect.WithESHealthCheck(true),
    reconnect.WithESHealthCheckInterval(15*time.Second),
    reconnect.WithESMaxRetries(15),
    reconnect.WithESRetryInterval(func(retryCount int) time.Duration {
        // 线性策略
        return time.Duration(retryCount+1) * 5 * time.Second
    }),
)
if err != nil {
    log.Fatalf("创建Elasticsearch重连器失败: %v", err)
}

// 启动重连监控
if err := reconnector.Start(); err != nil {
    log.Fatalf("启动Elasticsearch重连监控失败: %v", err)
}
defer reconnector.Stop()

// 使用Elasticsearch客户端
// ...
```

### 4. 使用重连管理器

```go
// 创建重连管理器
manager := reconnect.NewReconnectManager()

// 注册重连器
manager.RegisterReconnector("main_db", mysqlReconnector)
manager.RegisterReconnector("cache", redisReconnector)
manager.RegisterReconnector("search", esReconnector)

// 启动所有重连器
errors := manager.StartAll()
for name, err := range errors {
    if err != nil {
        log.Printf("启动重连器 %s 失败: %v", name, err)
    }
}

// 获取特定重连器
if reconnector, exists := manager.GetReconnector("main_db"); exists {
    connected := reconnector.IsConnected()
    log.Printf("main_db 连接状态: %v", connected)
}

// 应用关闭时停止所有重连器
defer manager.StopAll()
```

## 配置参数

### 1. MySQL重连器配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| MaxRetries | int | 最大重试次数 | 10 |
| RetryInterval | func(int) time.Duration | 重试间隔函数 | 指数退避 |
| HealthCheck | bool | 是否启用健康检查 | true |
| HealthCheckInterval | time.Duration | 健康检查间隔 | 10s |
| HealthCheckTimeout | time.Duration | 健康检查超时 | 3s |
| HealthCheckQuery | string | 健康检查查询 | "SELECT 1" |
| Logger | Logger | 自定义日志记录器 | nil |

### 2. Redis重连器配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| MaxRetries | int | 最大重试次数 | 5 |
| RetryInterval | func(int) time.Duration | 重试间隔函数 | 指数退避 |
| HealthCheck | bool | 是否启用健康检查 | true |
| HealthCheckInterval | time.Duration | 健康检查间隔 | 5s |
| HealthCheckTimeout | time.Duration | 健康检查超时 | 2s |
| Logger | Logger | 自定义日志记录器 | nil |

### 3. Elasticsearch重连器配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| MaxRetries | int | 最大重试次数 | 15 |
| RetryInterval | func(int) time.Duration | 重试间隔函数 | 指数退避 |
| HealthCheck | bool | 是否启用健康检查 | true |
| HealthCheckInterval | time.Duration | 健康检查间隔 | 15s |
| HealthCheckTimeout | time.Duration | 健康检查超时 | 5s |
| NodeFailureThreshold | int | 节点失败阈值 | 3 |
| Logger | Logger | 自定义日志记录器 | nil |

## 重试策略

模块提供了多种内置重试策略，以适应不同的场景需求：

### 1. 指数退避策略

重试间隔随着重试次数的增加呈指数增长，适合处理临时性故障，避免重试风暴。

```go
func ExponentialBackoff(base time.Duration) func(int) time.Duration {
    return func(retryCount int) time.Duration {
        return time.Duration(math.Pow(2, float64(retryCount))) * base
    }
}

// 使用示例
reconnector.SetRetryInterval(reconnect.ExponentialBackoff(time.Second))
// 重试间隔：1s, 2s, 4s, 8s, 16s, ...
```

### 2. 线性策略

重试间隔随着重试次数线性增长，适合需要均匀增加间隔的场景。

```go
func LinearBackoff(base time.Duration) func(int) time.Duration {
    return func(retryCount int) time.Duration {
        return time.Duration(retryCount+1) * base
    }
}

// 使用示例
reconnector.SetRetryInterval(reconnect.LinearBackoff(2*time.Second))
// 重试间隔：2s, 4s, 6s, 8s, ...
```

### 3. 固定间隔策略

每次重试使用相同的间隔时间，适合简单场景。

```go
func FixedInterval(interval time.Duration) func(int) time.Duration {
    return func(retryCount int) time.Duration {
        return interval
    }
}

// 使用示例
reconnector.SetRetryInterval(reconnect.FixedInterval(5*time.Second))
// 重试间隔：5s, 5s, 5s, ...
```

### 4. 随机抖动策略

在基本间隔上增加随机抖动，避免多个客户端同时重试造成的"惊群效应"。

```go
func JitteredBackoff(base time.Duration, maxJitter time.Duration) func(int) time.Duration {
    return func(retryCount int) time.Duration {
        baseInterval := time.Duration(math.Pow(2, float64(retryCount))) * base
        jitter := time.Duration(rand.Int63n(int64(maxJitter)))
        return baseInterval + jitter
    }
}

// 使用示例
reconnector.SetRetryInterval(reconnect.JitteredBackoff(time.Second, 500*time.Millisecond))
// 重试间隔：1s+随机值, 2s+随机值, 4s+随机值, ...
```

## 健康检查机制

自动重连模块使用健康检查来主动探测连接状态，而不仅仅依赖于操作失败时的被动检测。

### 1. MySQL健康检查

通过执行简单查询检查数据库连接是否正常。

```go
// 默认健康检查查询
SELECT 1
```

### 2. Redis健康检查

通过执行PING命令检查Redis服务是否正常响应。

### 3. Elasticsearch健康检查

通过检查集群健康状态API确定Elasticsearch服务是否可用。

```
GET _cluster/health
```

## 最佳实践

1. **设置合适的重试策略**
   - 对暂时性故障使用指数退避策略
   - 避免过于频繁的重试，以免增加服务负担
   - 考虑添加随机抖动，避免重试风暴

2. **正确处理重连事件**
   - 断开连接时记录详细日志，并可能发送告警
   - 重连成功后恢复正常操作，可能需要重新初始化某些资源
   - 重连失败达到最大次数后采取备用方案

3. **配置健康检查**
   - 根据网络环境设置合适的健康检查间隔
   - 避免过于频繁的健康检查，以免增加服务负担
   - 设置合理的健康检查超时时间

4. **资源管理**
   - 在应用关闭时正确停止重连器
   - 避免在高频操作中反复创建和停止重连器
   - 使用重连管理器统一管理多个重连器

5. **与系统其他部分集成**
   - 将重连事件与日志系统集成
   - 将重连状态与监控系统集成
   - 考虑将严重的连接问题与告警系统集成 