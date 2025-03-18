# MySQL 存储模块

## 概述

MySQL 存储模块提供了与 MySQL 数据库交互的功能，包括连接管理、自动重连、连接池管理和健康检查等功能。

## 核心组件

### 1. 客户端 (client.go)

提供基础的 MySQL 客户端实现，包装了 GORM 库，提供了事务管理和错误处理等功能。

```go
// MySQLClient MySQL客户端结构体
type MySQLClient struct {
    db *gorm.DB
    // 其他字段...
}

// 基本方法:
// - Connect: 连接数据库
// - Disconnect: 断开连接
// - Ping: 健康检查
// - GetDB: 获取GORM实例
// - WithTransaction: 在事务中执行操作
```

### 2. 重连机制 (reconnect.go)

实现自动重连功能，在数据库连接断开时自动尝试重新连接。

```go
// ReconnectableClient 支持自动重连的MySQL客户端
type ReconnectableClient struct {
    client  *MySQLClient
    config  MysqlConfig
    policy  reconnect.Policy
    // 其他字段...
}

// 主要方法:
// - SetReconnectPolicy: 设置重连策略
// - TriggerReconnect: 触发重连
// - GetDB: 获取数据库连接，自动处理重连
```

### 3. 连接池管理 (pool.go)

提供连接池管理功能，优化数据库连接的使用和资源分配。

```go
// Pool MySQL连接池结构体
type Pool struct {
    db          *gorm.DB
    config      PoolConfig
    stats       PoolStats
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
// 创建MySQL配置
config := mysql.MysqlConfig{
    Host:        "localhost",
    Port:        3306,
    Username:    "root",
    Password:    "password",
    Database:    "test_db",
    MaxIdleConn: 10,
    MaxOpenConn: 100,
    MaxLifeTime: 3600,
}

// 初始化MySQL客户端
client, err := mysql.NewMySQLClient(config)
if err != nil {
    log.Fatalf("初始化MySQL客户端失败: %v", err)
}

// 连接数据库
ctx := context.Background()
if err := client.Connect(ctx); err != nil {
    log.Fatalf("连接MySQL数据库失败: %v", err)
}

// 执行数据库操作
db := client.GetDB()
result := db.Create(&User{Name: "张三", Age: 30})
if result.Error != nil {
    log.Printf("创建用户失败: %v", result.Error)
}

// 在事务中执行操作
err = client.WithTransaction(ctx, func(tx *gorm.DB) error {
    // 执行多个数据库操作...
    tx.Create(&User{Name: "李四", Age: 25})
    return tx.Create(&User{Name: "王五", Age: 35}).Error
})
```

### 使用自动重连

```go
// 创建支持自动重连的客户端
reconnectableClient, err := mysql.NewReconnectableClient(config)
if err != nil {
    log.Fatalf("创建可重连MySQL客户端失败: %v", err)
}

// 设置重连策略
reconnectableClient.SetReconnectPolicy(reconnect.NewExponentialBackoff(
    5,                   // 最大重试次数
    2*time.Second,       // 初始重试间隔
    30*time.Second,      // 最大重试间隔
))

// 使用客户端执行操作
db := reconnectableClient.GetDB()
// 如果连接中断，GetDB方法会自动尝试重连
```

### 连接池管理

```go
// 创建连接池配置
poolConfig := mysql.DefaultPoolConfig()
poolConfig.MaxIdleConns = 20
poolConfig.MaxOpenConns = 200
poolConfig.ConnMaxLifetime = 1 * time.Hour
poolConfig.EnableStats = true

// 创建连接池
pool, err := mysql.NewPool(client.GetDB(), poolConfig)
if err != nil {
    log.Fatalf("创建MySQL连接池失败: %v", err)
}

// 获取连接池统计信息
stats := pool.Stats()
log.Printf("连接池状态: 活跃=%d, 空闲=%d, 等待=%d", 
    stats.ActiveConnections, 
    stats.IdleConnections, 
    stats.WaitCount)

// 定期清理空闲连接
go func() {
    ticker := time.NewTicker(10 * time.Minute)
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
| Host | string | 数据库主机 | localhost |
| Port | int | 数据库端口 | 3306 |
| Username | string | 用户名 | root |
| Password | string | 密码 | - |
| Database | string | 数据库名 | - |
| Charset | string | 字符集 | utf8mb4 |
| MaxIdleConn | int | 最大空闲连接数 | 10 |
| MaxOpenConn | int | 最大打开连接数 | 100 |
| MaxLifeTime | int | 连接最大生命周期(秒) | 3600 |

### 连接池配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| MaxIdleConns | int | 最大空闲连接数 | 10 |
| MaxOpenConns | int | 最大打开连接数 | 100 |
| ConnMaxLifetime | time.Duration | 连接最大生命周期 | 1小时 |
| ConnMaxIdleTime | time.Duration | 连接最大空闲时间 | 30分钟 |
| EnableStats | bool | 是否启用统计 | true |

## 最佳实践

1. **合理配置连接池**
   - 根据应用的并发需求和数据库服务器能力设置连接池大小
   - 避免设置过大的连接池，可能导致数据库服务器压力过大
   - 避免设置过小的连接池，可能导致连接竞争和等待

2. **使用自动重连**
   - 对于长期运行的应用，启用自动重连功能
   - 设置合理的重试策略，避免在服务器故障时频繁重试
   - 在重连事件中添加监控和告警

3. **事务管理**
   - 使用WithTransaction方法确保事务正确提交或回滚
   - 避免长事务，可能导致锁冲突和性能问题
   - 事务中仅包含必要的操作，保持简洁 