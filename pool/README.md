# 存储连接池模块

## 概述

连接池模块(`pkg/storage/pool`)为不同类型的存储服务提供统一的连接池管理接口，用于优化连接资源的分配和利用，提高系统性能和稳定性。该模块实现了针对MySQL、Redis和Elasticsearch等存储服务的连接池，并提供了灵活的配置选项。

## 核心组件

### 1. 连接池接口 (interface.go)

定义了所有连接池实现必须满足的统一接口，便于系统统一管理不同类型的连接池。

```go
// ConnectionPool 连接池通用接口
type ConnectionPool interface {
    // Get 获取连接
    Get(ctx context.Context) (interface{}, error)
    
    // Release 释放连接
    Release(conn interface{}) error
    
    // Close 关闭连接池
    Close() error
    
    // Stats 获取连接池统计信息
    Stats() PoolStats
    
    // SetMaxIdleConns 设置最大空闲连接数
    SetMaxIdleConns(n int)
    
    // SetMaxOpenConns 设置最大打开连接数
    SetMaxOpenConns(n int)
    
    // SetConnMaxLifetime 设置连接最大生命周期
    SetConnMaxLifetime(d time.Duration)
    
    // SetConnMaxIdleTime 设置连接最大空闲时间
    SetConnMaxIdleTime(d time.Duration)
}

// PoolStats 连接池统计信息
type PoolStats struct {
    // MaxOpenConnections 最大连接数
    MaxOpenConnections int
    
    // OpenConnections 当前打开的连接数
    OpenConnections int
    
    // InUseConnections 正在使用的连接数
    InUseConnections int
    
    // IdleConnections 空闲连接数
    IdleConnections int
    
    // WaitCount 等待次数
    WaitCount int64
    
    // WaitDuration 等待时间
    WaitDuration time.Duration
    
    // MaxIdleClosed 由于最大空闲连接数限制而关闭的连接数
    MaxIdleClosed int64
    
    // MaxLifetimeClosed 由于最大生命周期限制而关闭的连接数
    MaxLifetimeClosed int64
    
    // MaxIdleTimeClosed 由于最大空闲时间限制而关闭的连接数
    MaxIdleTimeClosed int64
}
```

### 2. MySQL连接池 (mysql.go)

基于Go标准库`database/sql`的连接池实现，用于管理MySQL数据库连接。

```go
// MySQLPool MySQL连接池
type MySQLPool struct {
    db *sql.DB
    // 其他字段...
}

// NewMySQLPool 创建MySQL连接池
func NewMySQLPool(dsn string, options ...MySQLPoolOption) (*MySQLPool, error) {
    // ...
}
```

### 3. Redis连接池 (redis.go)

基于`github.com/go-redis/redis/v8`的连接池实现，用于管理Redis服务连接。

```go
// RedisPool Redis连接池
type RedisPool struct {
    client *redis.Client
    stats  PoolStats
    // 其他字段...
}

// NewRedisPool 创建Redis连接池
func NewRedisPool(options *redis.Options, poolOptions ...RedisPoolOption) (*RedisPool, error) {
    // ...
}
```

### 4. Elasticsearch连接池 (elasticsearch.go)

为Elasticsearch客户端提供的连接池实现，优化ES客户端连接资源的使用。

```go
// ESPool Elasticsearch连接池
type ESPool struct {
    clients      []*elasticsearch.Client
    configs      []elasticsearch.Config
    currentIndex int32
    // 其他字段...
}

// NewESPool 创建Elasticsearch连接池
func NewESPool(configs []elasticsearch.Config, options ...ESPoolOption) (*ESPool, error) {
    // ...
}
```

### 5. 工厂函数 (factory.go)

提供创建不同类型连接池的工厂方法，简化连接池的创建过程。

```go
// Factory 连接池工厂
type Factory interface {
    // CreateMySQLPool 创建MySQL连接池
    CreateMySQLPool(dsn string, options ...MySQLPoolOption) (*MySQLPool, error)
    
    // CreateRedisPool 创建Redis连接池
    CreateRedisPool(options *redis.Options, poolOptions ...RedisPoolOption) (*RedisPool, error)
    
    // CreateESPool 创建Elasticsearch连接池
    CreateESPool(configs []elasticsearch.Config, options ...ESPoolOption) (*ESPool, error)
}
```

## 使用示例

### 1. MySQL连接池

```go
// 创建MySQL连接池
dsn := "user:password@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
mysqlPool, err := pool.NewMySQLPool(dsn,
    pool.WithMySQLMaxOpenConns(20),
    pool.WithMySQLMaxIdleConns(10),
    pool.WithMySQLConnMaxLifetime(time.Hour),
    pool.WithMySQLConnMaxIdleTime(30*time.Minute),
)
if err != nil {
    log.Fatalf("创建MySQL连接池失败: %v", err)
}
defer mysqlPool.Close()

// 获取连接
ctx := context.Background()
conn, err := mysqlPool.Get(ctx)
if err != nil {
    log.Printf("获取MySQL连接失败: %v", err)
    return
}

// 使用连接
db := conn.(*sql.DB)
rows, err := db.QueryContext(ctx, "SELECT * FROM users LIMIT 10")
// 处理查询结果...

// 释放连接
mysqlPool.Release(conn)

// 获取连接池统计信息
stats := mysqlPool.Stats()
log.Printf("MySQL连接池状态: 已打开=%d, 使用中=%d, 空闲=%d, 等待次数=%d",
    stats.OpenConnections, stats.InUseConnections, stats.IdleConnections, stats.WaitCount)
```

### 2. Redis连接池

```go
// 创建Redis连接池
redisOptions := &redis.Options{
    Addr:     "localhost:6379",
    Password: "password",
    DB:       0,
}
redisPool, err := pool.NewRedisPool(redisOptions,
    pool.WithRedisMaxOpenConns(100),
    pool.WithRedisMaxIdleConns(20),
    pool.WithRedisConnMaxLifetime(time.Hour),
    pool.WithRedisDialTimeout(5*time.Second),
)
if err != nil {
    log.Fatalf("创建Redis连接池失败: %v", err)
}
defer redisPool.Close()

// 获取连接
ctx := context.Background()
conn, err := redisPool.Get(ctx)
if err != nil {
    log.Printf("获取Redis连接失败: %v", err)
    return
}

// 使用连接
client := conn.(*redis.Client)
val, err := client.Get(ctx, "key").Result()
// 处理结果...

// 释放连接
redisPool.Release(conn)

// 获取连接池统计信息
stats := redisPool.Stats()
log.Printf("Redis连接池状态: 已打开=%d, 使用中=%d, 空闲=%d",
    stats.OpenConnections, stats.InUseConnections, stats.IdleConnections)
```

### 3. Elasticsearch连接池

```go
// 创建Elasticsearch连接池
esConfigs := []elasticsearch.Config{
    {
        Addresses: []string{"http://localhost:9200"},
        Username:  "user",
        Password:  "password",
    },
    {
        Addresses: []string{"http://localhost:9201"},
        Username:  "user",
        Password:  "password",
    },
}
esPool, err := pool.NewESPool(esConfigs,
    pool.WithESMaxRetries(3),
    pool.WithESRetryBackoff(func(attempt int) time.Duration {
        return time.Duration(math.Pow(2, float64(attempt))) * time.Second
    }),
    pool.WithESRequestTimeout(10*time.Second),
)
if err != nil {
    log.Fatalf("创建Elasticsearch连接池失败: %v", err)
}
defer esPool.Close()

// 获取连接
ctx := context.Background()
conn, err := esPool.Get(ctx)
if err != nil {
    log.Printf("获取Elasticsearch连接失败: %v", err)
    return
}

// 使用连接
client := conn.(*elasticsearch.Client)
res, err := client.Info()
// 处理结果...

// 释放连接
esPool.Release(conn)

// 获取连接池统计信息
stats := esPool.Stats()
log.Printf("Elasticsearch连接池状态: 总连接数=%d, 使用中=%d", stats.OpenConnections, stats.InUseConnections)
```

## 配置参数

### 1. MySQL连接池配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| MaxOpenConns | int | 最大打开连接数 | 10 |
| MaxIdleConns | int | 最大空闲连接数 | 5 |
| ConnMaxLifetime | time.Duration | 连接最大生命周期 | 1h |
| ConnMaxIdleTime | time.Duration | 连接最大空闲时间 | 30m |
| SetLogger | Logger | 自定义日志记录器 | nil |

### 2. Redis连接池配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| MaxOpenConns | int | 最大连接数 | 100 |
| MaxIdleConns | int | 最大空闲连接数 | 10 |
| ConnMaxLifetime | time.Duration | 连接最大生命周期 | 1h |
| DialTimeout | time.Duration | 连接超时时间 | 5s |
| ReadTimeout | time.Duration | 读取超时时间 | 3s |
| WriteTimeout | time.Duration | 写入超时时间 | 3s |
| PoolTimeout | time.Duration | 池等待超时时间 | 4s |
| IdleTimeout | time.Duration | 空闲超时时间 | 5m |
| SetLogger | Logger | 自定义日志记录器 | nil |

### 3. Elasticsearch连接池配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| MaxRetries | int | 最大重试次数 | 3 |
| RetryBackoff | func(int) time.Duration | 重试间隔函数 | 指数退避 |
| RetryStatuses | []int | 需要重试的HTTP状态码 | [502, 503, 504] |
| RequestTimeout | time.Duration | 请求超时时间 | 10s |
| CompressRequestBody | bool | 是否压缩请求体 | true |
| EnableMetrics | bool | 是否启用指标 | false |
| SetLogger | Logger | 自定义日志记录器 | nil |

## 最佳实践

1. **合理设置连接池大小**
   - 根据服务器资源和应用负载合理设置最大连接数
   - 考虑数据库或服务端的连接限制
   - 公式: `最大连接数 = CPU核心数 * (1 + 每个查询的磁盘等待时间)`

2. **设置合适的超时参数**
   - 连接超时应该根据网络情况设置，通常在1-5秒范围
   - 读写超时应根据操作复杂性设置，简单操作可以较短
   - 避免过长的超时时间占用资源，也避免过短导致正常操作被中断

3. **管理连接生命周期**
   - 设置`ConnMaxLifetime`避免长时间占用连接
   - 使用`ConnMaxIdleTime`释放长时间不用的连接
   - 定期监控连接池状态，检查异常连接

4. **错误处理与重试机制**
   - 对临时性错误实现重试逻辑
   - 使用指数退避算法避免重试风暴
   - 设置最大重试次数，避免无限重试

5. **日志与监控**
   - 记录关键连接池事件，如创建、关闭、异常等
   - 监控连接池统计数据，设置适当的告警阈值
   - 跟踪连接获取的等待时间，及时发现性能瓶颈

6. **资源清理**
   - 始终在操作完成后释放连接
   - 使用`defer`确保即使发生异常也能释放连接
   - 应用关闭时正确关闭连接池 