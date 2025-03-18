# Elasticsearch 存储模块

## 概述

Elasticsearch 存储模块提供了与 Elasticsearch 集群交互的功能，包括连接管理、自动重连、连接池管理、索引操作和搜索功能。该模块基于官方的 Elasticsearch Go 客户端构建，提供了更加易用和健壮的接口。

## 核心组件

### 1. 客户端 (client.go)

提供基础的 Elasticsearch 客户端实现，包装了官方 ES 客户端，提供了索引管理、文档操作和搜索功能。

```go
// ESClient Elasticsearch客户端结构体
type ESClient struct {
    client *elasticsearch.Client
    config ESConfig
    // 其他字段...
}

// 基本方法:
// - Connect: 连接ES集群
// - Disconnect: 断开连接
// - Ping: 健康检查
// - GetClient: 获取原始ES客户端
// - IndexExists: 检查索引是否存在
// - CreateIndex: 创建索引
// - DeleteIndex: 删除索引
// - Index: 添加或更新文档
// - Search: 搜索文档
```

### 2. 重连机制 (reconnect.go)

实现自动重连功能，在 Elasticsearch 连接断开时自动尝试重新连接。

```go
// ReconnectableClient 支持自动重连的ES客户端
type ReconnectableClient struct {
    client     *ESClient
    config     ESConfig
    policy     reconnect.Policy
    // 其他字段...
}

// 主要方法:
// - SetReconnectPolicy: 设置重连策略
// - TriggerReconnect: 触发重连
// - GetClient: 获取ES客户端，自动处理重连
// - SearchWithReconnect: 执行搜索，遇到连接错误时自动重连
```

### 3. 连接池管理 (pool.go)

提供连接池管理功能，优化 Elasticsearch 连接的使用和资源分配。

```go
// Pool Elasticsearch连接池结构体
type Pool struct {
    client     *elasticsearch.Client
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
// 创建ES配置
config := elasticsearch.ESConfig{
    Addresses:   []string{"http://localhost:9200"},
    Username:    "elastic",
    Password:    "password",
    EnableHTTPS: false,
    MaxRetries:  3,
}

// 初始化ES客户端
client, err := elasticsearch.NewESClient(config)
if err != nil {
    log.Fatalf("初始化ES客户端失败: %v", err)
}

// 连接ES集群
ctx := context.Background()
if err := client.Connect(ctx); err != nil {
    log.Fatalf("连接ES集群失败: %v", err)
}

// 检查索引是否存在
exists, err := client.IndexExists(ctx, "products")
if err != nil {
    log.Printf("检查索引失败: %v", err)
}

// 创建索引
if !exists {
    mapping := `{
        "settings": {
            "number_of_shards": 1,
            "number_of_replicas": 0
        },
        "mappings": {
            "properties": {
                "name": { "type": "text", "analyzer": "standard" },
                "price": { "type": "float" },
                "created_at": { "type": "date" },
                "tags": { "type": "keyword" }
            }
        }
    }`
    
    err := client.CreateIndex(ctx, "products", mapping)
    if err != nil {
        log.Printf("创建索引失败: %v", err)
    } else {
        log.Println("成功创建索引: products")
    }
}

// 索引文档
product := map[string]interface{}{
    "name":       "高性能笔记本电脑",
    "price":      6999.99,
    "created_at": time.Now().Format(time.RFC3339),
    "tags":       []string{"电子产品", "笔记本", "办公"},
}

err = client.Index(ctx, "products", "1", product)
if err != nil {
    log.Printf("索引文档失败: %v", err)
} else {
    log.Println("成功索引文档")
}

// 搜索文档
searchBody := `{
    "query": {
        "bool": {
            "must": [
                { "match": { "name": "笔记本" } }
            ],
            "filter": [
                { "range": { "price": { "gte": 5000 } } }
            ]
        }
    },
    "sort": [
        { "price": { "order": "desc" } }
    ],
    "_source": ["name", "price", "tags"]
}`

result, err := client.Search(ctx, []string{"products"}, searchBody)
if err != nil {
    log.Printf("搜索失败: %v", err)
} else {
    log.Printf("搜索结果: 总数=%d, 命中=%+v", result.Total, result.Hits)
}
```

### 使用自动重连

```go
// 创建支持自动重连的客户端
reconnectableClient, err := elasticsearch.NewReconnectableClient(config)
if err != nil {
    log.Fatalf("创建可重连ES客户端失败: %v", err)
}

// 设置重连策略
reconnectableClient.SetReconnectPolicy(reconnect.NewExponentialBackoff(
    3,                   // 最大重试次数
    2*time.Second,       // 初始重试间隔
    30*time.Second,      // 最大重试间隔
))

// 使用客户端执行搜索，自动处理重连
result, err := reconnectableClient.SearchWithReconnect(ctx, []string{"products"}, searchBody)
if err != nil {
    log.Printf("搜索失败: %v", err)
} else {
    log.Printf("搜索结果: 总数=%d", result.Total)
}

// 获取底层客户端执行其他操作
esClient := reconnectableClient.GetClient()
// 如果连接中断，GetClient方法会自动尝试重连
```

### 连接池管理

```go
// 创建连接池配置
poolConfig := elasticsearch.DefaultPoolConfig()
poolConfig.MaxIdleConnections = 10
poolConfig.MaxConnectionsPerHost = 20
poolConfig.IdleTimeout = 15 * time.Minute
poolConfig.EnableStats = true

// 创建连接池
pool, err := elasticsearch.NewPool(client.GetClient(), poolConfig)
if err != nil {
    log.Fatalf("创建ES连接池失败: %v", err)
}

// 获取连接池统计信息
stats := pool.Stats()
log.Printf("连接池状态: 总连接=%d, 空闲连接=%d", 
    stats.TotalConnections, 
    stats.IdleConnections)

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

## 索引与搜索操作

### 索引管理

```go
// 获取所有索引
indices, err := client.GetIndices(ctx)
if err != nil {
    log.Printf("获取索引列表失败: %v", err)
} else {
    log.Printf("索引列表: %v", indices)
}

// 获取索引设置
settings, err := client.GetIndexSettings(ctx, "products")
if err != nil {
    log.Printf("获取索引设置失败: %v", err)
} else {
    log.Printf("索引设置: %+v", settings)
}

// 更新索引设置
settingsUpdate := `{
    "index": {
        "number_of_replicas": 1
    }
}`
err = client.UpdateIndexSettings(ctx, "products", settingsUpdate)
if err != nil {
    log.Printf("更新索引设置失败: %v", err)
}

// 获取索引映射
mapping, err := client.GetMapping(ctx, "products")
if err != nil {
    log.Printf("获取索引映射失败: %v", err)
} else {
    log.Printf("索引映射: %+v", mapping)
}

// 创建索引别名
err = client.CreateAlias(ctx, "products", "active_products")
if err != nil {
    log.Printf("创建索引别名失败: %v", err)
}
```

### 文档操作

```go
// 批量索引文档
products := []map[string]interface{}{
    {
        "id": "2",
        "name": "智能手机",
        "price": 4999.99,
        "tags": []string{"电子产品", "手机"},
    },
    {
        "id": "3",
        "name": "无线耳机",
        "price": 1299.99,
        "tags": []string{"电子产品", "音频设备"},
    },
}

err = client.BulkIndex(ctx, "products", products, "id")
if err != nil {
    log.Printf("批量索引文档失败: %v", err)
}

// 获取文档
doc, err := client.Get(ctx, "products", "1")
if err != nil {
    log.Printf("获取文档失败: %v", err)
} else {
    log.Printf("文档内容: %+v", doc)
}

// 更新文档
update := map[string]interface{}{
    "price": 6799.99,
    "tags": []string{"电子产品", "笔记本", "办公", "促销"},
}
err = client.Update(ctx, "products", "1", update)
if err != nil {
    log.Printf("更新文档失败: %v", err)
}

// 删除文档
err = client.Delete(ctx, "products", "3")
if err != nil {
    log.Printf("删除文档失败: %v", err)
}
```

### 高级搜索

```go
// 聚合查询
aggregationQuery := `{
    "size": 0,
    "aggs": {
        "price_ranges": {
            "range": {
                "field": "price",
                "ranges": [
                    { "to": 1000 },
                    { "from": 1000, "to": 5000 },
                    { "from": 5000 }
                ]
            }
        },
        "avg_price": {
            "avg": {
                "field": "price"
            }
        },
        "tags": {
            "terms": {
                "field": "tags",
                "size": 10
            }
        }
    }
}`

result, err := client.Search(ctx, []string{"products"}, aggregationQuery)
if err != nil {
    log.Printf("聚合查询失败: %v", err)
} else {
    log.Printf("聚合结果: %+v", result.Aggregations)
}

// 模糊搜索与高亮
fuzzyQuery := `{
    "query": {
        "multi_match": {
            "query": "笔记",
            "fields": ["name^3", "tags"],
            "fuzziness": "AUTO"
        }
    },
    "highlight": {
        "fields": {
            "name": {}
        }
    }
}`

result, err = client.Search(ctx, []string{"products"}, fuzzyQuery)
if err != nil {
    log.Printf("模糊搜索失败: %v", err)
} else {
    for _, hit := range result.Hits {
        log.Printf("匹配文档: %+v, 高亮: %+v", hit.Source, hit.Highlight)
    }
}

// 复合查询与分页
compoundQuery := `{
    "from": 0,
    "size": 10,
    "query": {
        "bool": {
            "should": [
                { "match": { "name": "笔记本" } },
                { "match": { "name": "电脑" } }
            ],
            "must_not": [
                { "term": { "tags": "停产" } }
            ],
            "minimum_should_match": 1,
            "boost": 1.0
        }
    },
    "sort": [
        { "_score": { "order": "desc" } },
        { "created_at": { "order": "desc" } }
    ]
}`

result, err = client.Search(ctx, []string{"products"}, compoundQuery)
if err != nil {
    log.Printf("复合查询失败: %v", err)
}
```

## 配置参数

### 客户端配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| Addresses | []string | ES服务器地址 | ["http://localhost:9200"] |
| Username | string | 用户名 | "" |
| Password | string | 密码 | "" |
| CloudID | string | 云ID | "" |
| APIKey | string | API密钥 | "" |
| EnableHTTPS | bool | 是否启用HTTPS | false |
| SkipVerify | bool | 是否跳过证书验证 | false |
| MaxRetries | int | 最大重试次数 | 3 |
| RetryTimeout | int | 重试超时时间(秒) | 5 |

### 连接池配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| MaxIdleConnections | int | 最大空闲连接数 | 10 |
| MaxConnectionsPerHost | int | 每个主机最大连接数 | 20 |
| IdleTimeout | time.Duration | 空闲连接超时 | 30分钟 |
| EnableStats | bool | 是否启用统计 | true |

## 最佳实践

1. **索引设计**
   - 根据数据特性和查询需求设计合理的索引结构
   - 使用合适的分片和副本数，平衡可用性和性能
   - 根据数据类型选择合适的映射类型和分析器

2. **搜索优化**
   - 使用过滤器(filter)代替查询(query)，利用缓存提高性能
   - 使用聚合和分页减少返回数据量
   - 针对大数据量搜索使用滚动查询(scroll API)

3. **连接管理**
   - 合理配置连接池，避免连接泄漏
   - 对于分布式集群，设置合理的连接超时和重试策略
   - 使用健康检查监控集群状态

4. **错误处理**
   - 区分临时错误和集群故障
   - 实现熔断机制，避免在集群过载时持续发送请求
   - 记录和监控搜索性能指标

## 高级功能

1. **文档更新策略**
   - 乐观锁并发控制
   - 部分更新与脚本更新
   - 批量操作与错误处理

2. **索引生命周期管理**
   - 索引创建与模板
   - 索引别名与零停机切换
   - 索引滚动与归档

3. **集群监控与管理**
   - 集群健康状态检查
   - 节点性能监控
   - 分片均衡与恢复 