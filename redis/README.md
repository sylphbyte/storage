# Redis 存储模块

## 概述

Redis 存储模块提供了与 Redis 服务器交互的功能，包括连接管理、自动重连、连接池管理和各种 Redis 操作封装。该模块基于 go-redis/redis 库构建，提供了更加易用和健壮的接口。

## 核心组件

### 1. 客户端 (client.go)

提供基础的 Redis 客户端实现，包装了 go-redis 库，提供错误处理和管道操作等功能。

```go
// RedisClient Redis客户端结构体
type RedisClient struct {
    client *redis.Client
    config RedisConfig
    // 其他字段...
}

// 基本方法:
// - Connect: 连接Redis服务器
// - Disconnect: 断开连接
// - Ping: 健康检查
// - GetClient: 获取原始Redis客户端
// - WithPipeline: 在管道中执行操作
// - Do: 执行Redis命令
```

### 2. 重连机制 (reconnect.go)

实现自动重连功能，在 Redis 连接断开时自动尝试重新连接。

```go
// ReconnectableClient 支持自动重连的Redis客户端
type ReconnectableClient struct {
    client   *RedisClient
    config   RedisConfig
    policy   reconnect.Policy
    // 其他字段...
}

// 主要方法:
// - SetReconnectPolicy: 设置重连策略
// - TriggerReconnect: 触发重连
// - GetClient: 获取Redis客户端，自动处理重连
// - DoWithReconnect: 执行命令，遇到连接错误时自动重连
```

### 3. 连接池管理 (pool.go)

提供连接池管理功能，优化 Redis 连接的使用和资源分配。

```go
// Pool Redis连接池结构体
type Pool struct {
    client     *redis.Client
    config     PoolConfig
    stats      PoolStats
    // 其他字段...
}

// 主要方法:
// - Stats: 获取连接池统计信息
// - Resize: 调整连接池大小
// - ClearIdleConnections: 清理空闲连接
// - Close: 关闭连接池
```

## 使用示例

### 基本使用

```go
// 创建Redis配置
config := redis.RedisConfig{
    Host:     "localhost",
    Port:     6379,
    Password: "password",
    Database: 0,
}

// 初始化Redis客户端
client, err := redis.NewRedisClient(config)
if err != nil {
    log.Fatalf("初始化Redis客户端失败: %v", err)
}

// 连接Redis服务器
ctx := context.Background()
if err := client.Connect(ctx); err != nil {
    log.Fatalf("连接Redis服务器失败: %v", err)
}

// 基本操作
redisClient := client.GetClient()
err = redisClient.Set(ctx, "key", "value", time.Hour).Err()
if err != nil {
    log.Printf("设置键值失败: %v", err)
}

val, err := redisClient.Get(ctx, "key").Result()
if err != nil {
    log.Printf("获取键值失败: %v", err)
} else {
    log.Printf("获取的值: %s", val)
}

// 使用管道批量执行命令
err = client.WithPipeline(ctx, func(pipe redis.Pipeliner) error {
    pipe.Set(ctx, "key1", "value1", time.Hour)
    pipe.Set(ctx, "key2", "value2", time.Hour)
    pipe.Incr(ctx, "counter")
    _, err := pipe.Exec(ctx)
    return err
})
if err != nil {
    log.Printf("管道操作失败: %v", err)
}

// 执行任意命令
result, err := client.Do(ctx, "HSET", "user:1", "name", "张三", "age", "30").Result()
if err != nil {
    log.Printf("执行HSET命令失败: %v", err)
} else {
    log.Printf("HSET命令结果: %v", result)
}
```

### 使用自动重连

```go
// 创建支持自动重连的客户端
reconnectableClient, err := redis.NewReconnectableClient(config)
if err != nil {
    log.Fatalf("创建可重连Redis客户端失败: %v", err)
}

// 设置重连策略
reconnectableClient.SetReconnectPolicy(reconnect.NewExponentialBackoff(
    5,                   // 最大重试次数
    1*time.Second,       // 初始重试间隔
    10*time.Second,      // 最大重试间隔
))

// 使用客户端执行操作，自动处理重连
result, err := reconnectableClient.DoWithReconnect(ctx, "GET", "key").Result()
if err != nil {
    log.Printf("执行GET命令失败: %v", err)
} else {
    log.Printf("GET命令结果: %v", result)
}

// 获取底层客户端执行其他操作
redisClient := reconnectableClient.GetClient()
// 如果连接中断，GetClient方法会自动尝试重连
```

### 连接池管理

```go
// 创建连接池配置
poolConfig := redis.DefaultPoolConfig()
poolConfig.MaxActive = 50
poolConfig.MaxIdle = 10
poolConfig.IdleTimeout = 5 * time.Minute
poolConfig.EnableStats = true

// 创建连接池
pool, err := redis.NewPool(client.GetClient(), poolConfig)
if err != nil {
    log.Fatalf("创建Redis连接池失败: %v", err)
}

// 获取连接池统计信息
stats := pool.Stats()
log.Printf("连接池状态: 活跃=%d, 空闲=%d, 等待=%d", 
    stats.ActiveConnections, 
    stats.IdleConnections, 
    stats.WaitCount)

// 定期清理空闲连接
go func() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            pool.ClearIdleConnections()
        case <-ctx.Done():
            return
        }
    }
}()
```

## 配置参数

### 客户端配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| Host | string | Redis主机 | localhost |
| Port | int | Redis端口 | 6379 |
| Password | string | 密码 | "" |
| Database | int | 数据库索引 | 0 |
| MaxRetries | int | 最大重试次数 | 3 |
| MinRetryBackoff | time.Duration | 最小重试间隔 | 8ms |
| MaxRetryBackoff | time.Duration | 最大重试间隔 | 512ms |
| DialTimeout | time.Duration | 连接超时 | 5s |
| ReadTimeout | time.Duration | 读取超时 | 3s |
| WriteTimeout | time.Duration | 写入超时 | 3s |

### 连接池配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| MaxIdle | int | 最大空闲连接数 | 10 |
| MaxActive | int | 最大活跃连接数 | 100 |
| IdleTimeout | time.Duration | 空闲连接超时 | 5分钟 |
| Wait | bool | 是否等待连接 | true |
| EnableStats | bool | 是否启用统计 | true |

## 支持的操作

Redis 客户端支持所有标准的 Redis 操作，包括但不限于：

- **字符串操作**: GET, SET, INCR, DECR, MGET, MSET 等
- **哈希操作**: HGET, HSET, HMGET, HMSET, HGETALL 等
- **列表操作**: LPUSH, RPUSH, LPOP, RPOP, LRANGE 等
- **集合操作**: SADD, SMEMBERS, SISMEMBER, SREM 等
- **有序集合操作**: ZADD, ZRANGE, ZRANK, ZREM 等
- **地理位置操作**: GEOADD, GEOPOS, GEODIST 等
- **发布订阅**: PUBLISH, SUBSCRIBE, PSUBSCRIBE 等
- **事务操作**: MULTI, EXEC, DISCARD 等
- **Lua脚本**: EVAL, EVALSHA 等

## 最佳实践

1. **连接管理**
   - 应用启动时创建Redis客户端，应用结束时关闭
   - 使用自动重连功能处理瞬时网络问题
   - 设置合理的连接超时和操作超时

2. **批量操作**
   - 使用管道（Pipeline）批量执行多个命令，减少网络往返
   - 对于原子性要求高的场景，使用事务（MULTI/EXEC）
   - 对于复杂操作，使用Lua脚本减少网络通信

3. **错误处理**
   - 区分临时错误和永久错误
   - 对于临时错误（如连接断开），使用重试机制
   - 实现熔断机制，避免在Redis不可用时持续重试

4. **缓存策略**
   - 设置合理的过期时间，避免内存溢出
   - 实现缓存预热和缓存穿透保护
   - 考虑分布式环境下的缓存一致性问题 