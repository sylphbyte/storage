# 存储指标模块

## 概述

存储指标模块(`pkg/storage/metrics`)提供了一套统一的指标收集、处理和导出功能，专门针对存储服务的性能和使用情况收集关键指标数据。该模块与监控模块协同工作，但更专注于细粒度的指标收集和性能分析，支持为MySQL、Redis和Elasticsearch等存储服务提供标准化的性能指标。

## 核心组件

### 1. 指标接口 (interface.go)

定义了指标收集和处理的核心接口，实现了统一的指标模型。

```go
// MetricsCollector 指标收集器接口
type MetricsCollector interface {
    // Collect 收集指标
    Collect() ([]Metric, error)
    
    // CollectAsync 异步收集指标
    CollectAsync() (<-chan MetricsResult, error)
    
    // Name 收集器名称
    Name() string
    
    // Type 收集器类型
    Type() string
    
    // SetLabels 设置标签
    SetLabels(labels map[string]string)
}

// Metric 指标接口
type Metric interface {
    // Name 指标名称
    Name() string
    
    // Value 指标值
    Value() interface{}
    
    // Type 指标类型
    Type() MetricType
    
    // Labels 指标标签
    Labels() map[string]string
    
    // Description 指标描述
    Description() string
    
    // Timestamp 采集时间戳
    Timestamp() time.Time
}

// MetricType 指标类型
type MetricType int

const (
    // CounterType 计数器类型
    CounterType MetricType = iota
    
    // GaugeType 仪表盘类型
    GaugeType
    
    // HistogramType 直方图类型
    HistogramType
    
    // SummaryType 摘要类型
    SummaryType
)

// MetricsProcessor 指标处理器接口
type MetricsProcessor interface {
    // Process 处理指标
    Process(metrics []Metric) ([]Metric, error)
    
    // Name 处理器名称
    Name() string
}

// MetricsResult 异步指标结果
type MetricsResult struct {
    // Metrics 指标列表
    Metrics []Metric
    
    // Error 错误信息
    Error error
    
    // Source 指标来源
    Source string
}
```

### 2. MySQL指标收集器 (mysql.go)

专门为MySQL数据库收集性能指标，包括查询性能、连接状态、表操作等。

```go
// MySQLMetricsCollector MySQL指标收集器
type MySQLMetricsCollector struct {
    db          *sql.DB
    name        string
    labels      map[string]string
    collections []string
    // 其他字段...
}

// NewMySQLMetricsCollector 创建MySQL指标收集器
func NewMySQLMetricsCollector(name string, db *sql.DB, options ...MySQLMetricsOption) (*MySQLMetricsCollector, error) {
    // ...
}
```

### 3. Redis指标收集器 (redis.go)

专门为Redis服务收集性能指标，包括内存使用、命令执行、键统计等。

```go
// RedisMetricsCollector Redis指标收集器
type RedisMetricsCollector struct {
    client      *redis.Client
    name        string
    labels      map[string]string
    collections []string
    // 其他字段...
}

// NewRedisMetricsCollector 创建Redis指标收集器
func NewRedisMetricsCollector(name string, client *redis.Client, options ...RedisMetricsOption) (*RedisMetricsCollector, error) {
    // ...
}
```

### 4. Elasticsearch指标收集器 (elasticsearch.go)

专门为Elasticsearch服务收集性能指标，包括集群健康状态、节点性能、搜索性能等。

```go
// ESMetricsCollector Elasticsearch指标收集器
type ESMetricsCollector struct {
    client      *elasticsearch.Client
    name        string
    labels      map[string]string
    collections []string
    // 其他字段...
}

// NewESMetricsCollector 创建Elasticsearch指标收集器
func NewESMetricsCollector(name string, client *elasticsearch.Client, options ...ESMetricsOption) (*ESMetricsCollector, error) {
    // ...
}
```

### 5. 指标处理器 (processors/)

提供各种指标处理能力，如过滤、聚合、转换等。

#### 5.1 过滤处理器 (processors/filter.go)

根据规则过滤指标，保留或排除特定指标。

```go
// FilterProcessor 过滤处理器
type FilterProcessor struct {
    includePatterns []string
    excludePatterns []string
    name            string
    // 其他字段...
}

// NewFilterProcessor 创建过滤处理器
func NewFilterProcessor(name string, options ...FilterOption) (*FilterProcessor, error) {
    // ...
}
```

#### 5.2 聚合处理器 (processors/aggregate.go)

对指标进行聚合计算，如求和、平均值、最大值、最小值等。

```go
// AggregateProcessor 聚合处理器
type AggregateProcessor struct {
    aggregations map[string]Aggregation
    name         string
    // 其他字段...
}

// NewAggregateProcessor 创建聚合处理器
func NewAggregateProcessor(name string, options ...AggregateOption) (*AggregateProcessor, error) {
    // ...
}

// Aggregation 聚合方式
type Aggregation int

const (
    // SumAggregation 求和聚合
    SumAggregation Aggregation = iota
    
    // AvgAggregation 平均值聚合
    AvgAggregation
    
    // MaxAggregation 最大值聚合
    MaxAggregation
    
    // MinAggregation 最小值聚合
    MinAggregation
    
    // CountAggregation 计数聚合
    CountAggregation
)
```

#### 5.3 转换处理器 (processors/transform.go)

对指标进行数值转换，如单位转换、比率计算等。

```go
// TransformProcessor 转换处理器
type TransformProcessor struct {
    transformations map[string]Transformation
    name            string
    // 其他字段...
}

// NewTransformProcessor 创建转换处理器
func NewTransformProcessor(name string, options ...TransformOption) (*TransformProcessor, error) {
    // ...
}

// Transformation 转换函数
type Transformation func(interface{}) interface{}
```

### 6. 指标服务 (service.go)

集成指标收集器和处理器，提供统一的指标服务。

```go
// MetricsService 指标服务
type MetricsService struct {
    collectors map[string]MetricsCollector
    processors map[string]MetricsProcessor
    interval   time.Duration
    // 其他字段...
}

// NewMetricsService 创建指标服务
func NewMetricsService(options ...ServiceOption) (*MetricsService, error) {
    // ...
}

// RegisterCollector 注册指标收集器
func (s *MetricsService) RegisterCollector(collector MetricsCollector) error {
    // ...
}

// RegisterProcessor 注册指标处理器
func (s *MetricsService) RegisterProcessor(processor MetricsProcessor) error {
    // ...
}

// CollectAll 收集所有指标
func (s *MetricsService) CollectAll() (map[string][]Metric, map[string]error) {
    // ...
}

// ProcessMetrics 处理指标
func (s *MetricsService) ProcessMetrics(metrics []Metric) ([]Metric, error) {
    // ...
}

// Start 启动指标服务
func (s *MetricsService) Start() error {
    // ...
}

// Stop 停止指标服务
func (s *MetricsService) Stop() error {
    // ...
}
```

## 使用示例

### 1. MySQL指标收集

```go
// 创建MySQL指标收集器
db, _ := sql.Open("mysql", "user:password@tcp(127.0.0.1:3306)/dbname")
mysqlCollector, err := metrics.NewMySQLMetricsCollector(
    "main_db",
    db,
    metrics.WithMySQLCollections([]string{
        "performance_schema",
        "query_stats",
        "connection_stats",
        "innodb_stats",
    }),
    metrics.WithMySQLLabels(map[string]string{
        "environment": "production",
        "service":     "user_service",
    }),
)
if err != nil {
    log.Fatalf("创建MySQL指标收集器失败: %v", err)
}

// 收集指标
metricsData, err := mysqlCollector.Collect()
if err != nil {
    log.Printf("收集MySQL指标失败: %v", err)
} else {
    for _, metric := range metricsData {
        log.Printf("指标: %s, 值: %v, 类型: %v, 时间戳: %v",
            metric.Name(), metric.Value(), metric.Type(), metric.Timestamp())
    }
}

// 异步收集指标
resultsCh, err := mysqlCollector.CollectAsync()
if err != nil {
    log.Printf("启动异步收集失败: %v", err)
} else {
    for result := range resultsCh {
        if result.Error != nil {
            log.Printf("收集MySQL指标失败: %v", result.Error)
            continue
        }
        log.Printf("从 %s 收集到 %d 个指标", result.Source, len(result.Metrics))
    }
}
```

### 2. Redis指标收集与处理

```go
// 创建Redis指标收集器
client := redis.NewClient(&redis.Options{
    Addr:     "localhost:6379",
    Password: "password",
    DB:       0,
})
redisCollector, err := metrics.NewRedisMetricsCollector(
    "cache",
    client,
    metrics.WithRedisCollections([]string{
        "memory",
        "clients",
        "commands",
        "keys",
        "replication",
    }),
    metrics.WithRedisLabels(map[string]string{
        "environment": "production",
        "service":     "cache_service",
    }),
)
if err != nil {
    log.Fatalf("创建Redis指标收集器失败: %v", err)
}

// 创建过滤处理器
filterProcessor, err := metrics.NewFilterProcessor(
    "redis_filter",
    metrics.WithFilterIncludes([]string{
        "memory_*",
        "connected_clients",
        "instantaneous_ops_per_sec",
    }),
    metrics.WithFilterExcludes([]string{
        "*_raw",
        "*_human",
    }),
)
if err != nil {
    log.Fatalf("创建过滤处理器失败: %v", err)
}

// 创建转换处理器
transformProcessor, err := metrics.NewTransformProcessor(
    "redis_transform",
    metrics.WithTransformations(map[string]metrics.Transformation{
        "used_memory": func(val interface{}) interface{} {
            // 将字节转换为MB
            if v, ok := val.(int64); ok {
                return float64(v) / 1024 / 1024
            }
            return val
        },
    }),
)
if err != nil {
    log.Fatalf("创建转换处理器失败: %v", err)
}

// 收集指标
redisMetrics, err := redisCollector.Collect()
if err != nil {
    log.Printf("收集Redis指标失败: %v", err)
    return
}

// 应用过滤处理器
filteredMetrics, err := filterProcessor.Process(redisMetrics)
if err != nil {
    log.Printf("过滤Redis指标失败: %v", err)
    return
}

// 应用转换处理器
transformedMetrics, err := transformProcessor.Process(filteredMetrics)
if err != nil {
    log.Printf("转换Redis指标失败: %v", err)
    return
}

// 输出处理后的指标
for _, metric := range transformedMetrics {
    log.Printf("处理后指标: %s, 值: %v, 标签: %v",
        metric.Name(), metric.Value(), metric.Labels())
}
```

### 3. Elasticsearch指标收集与聚合

```go
// 创建Elasticsearch指标收集器
cfg := elasticsearch.Config{
    Addresses: []string{"http://localhost:9200"},
    Username:  "user",
    Password:  "password",
}
client, _ := elasticsearch.NewClient(cfg)
esCollector, err := metrics.NewESMetricsCollector(
    "search",
    client,
    metrics.WithESCollections([]string{
        "cluster_health",
        "node_stats",
        "indices_stats",
        "jvm_stats",
    }),
    metrics.WithESLabels(map[string]string{
        "environment": "production",
        "service":     "search_service",
    }),
)
if err != nil {
    log.Fatalf("创建Elasticsearch指标收集器失败: %v", err)
}

// 创建聚合处理器
aggregateProcessor, err := metrics.NewAggregateProcessor(
    "es_aggregate",
    metrics.WithAggregations(map[string]metrics.Aggregation{
        "search_query_time_avg": metrics.AvgAggregation,
        "indexing_time_total":   metrics.SumAggregation,
        "jvm_heap_used_max":     metrics.MaxAggregation,
    }),
)
if err != nil {
    log.Fatalf("创建聚合处理器失败: %v", err)
}

// 收集指标
esMetrics, err := esCollector.Collect()
if err != nil {
    log.Printf("收集Elasticsearch指标失败: %v", err)
    return
}

// 应用聚合处理器
aggregatedMetrics, err := aggregateProcessor.Process(esMetrics)
if err != nil {
    log.Printf("聚合Elasticsearch指标失败: %v", err)
    return
}

// 输出聚合后的指标
for _, metric := range aggregatedMetrics {
    log.Printf("聚合后指标: %s, 值: %v, 描述: %s",
        metric.Name(), metric.Value(), metric.Description())
}
```

### 4. 使用指标服务统一管理

```go
// 创建指标服务
service, err := metrics.NewMetricsService(
    metrics.WithServiceInterval(1*time.Minute),
    metrics.WithServiceBufferSize(1000),
    metrics.WithServiceLabels(map[string]string{
        "application": "my_app",
        "version":     "1.0.0",
    }),
)
if err != nil {
    log.Fatalf("创建指标服务失败: %v", err)
}

// 注册收集器
service.RegisterCollector(mysqlCollector)
service.RegisterCollector(redisCollector)
service.RegisterCollector(esCollector)

// 注册处理器
service.RegisterProcessor(filterProcessor)
service.RegisterProcessor(transformProcessor)
service.RegisterProcessor(aggregateProcessor)

// 启动指标服务
if err := service.Start(); err != nil {
    log.Fatalf("启动指标服务失败: %v", err)
}
defer service.Stop()

// 获取所有指标
allMetrics, errors := service.CollectAll()
for source, metrics := range allMetrics {
    log.Printf("从 %s 收集到 %d 个指标", source, len(metrics))
}
for source, err := range errors {
    if err != nil {
        log.Printf("从 %s 收集指标失败: %v", source, err)
    }
}

// 处理特定来源的指标
if metrics, ok := allMetrics["main_db"]; ok {
    processedMetrics, err := service.ProcessMetrics(metrics)
    if err != nil {
        log.Printf("处理指标失败: %v", err)
    } else {
        log.Printf("处理后得到 %d 个指标", len(processedMetrics))
    }
}

// 使用指标自定义回调函数
service.RegisterCallback("high_memory_alert", func(metrics []metrics.Metric) {
    for _, metric := range metrics {
        if metric.Name() == "memory_usage_percent" {
            if value, ok := metric.Value().(float64); ok && value > 85 {
                log.Printf("[ALERT] 高内存使用率: %.2f%%", value)
                // 发送告警通知
            }
        }
    }
})
```

## 指标类型

模块支持四种基本指标类型，用于表示不同的度量:

### 1. 计数器 (Counter)

持续累加的指标，只增不减，除非重置。适合表示如请求总数、错误总数等。

```go
// 创建计数器指标
counter := metrics.NewCounter(
    "mysql_queries_total",
    123456,
    map[string]string{"db": "main_db"},
    "MySQL查询总数",
)
```

### 2. 仪表盘 (Gauge)

可增可减的指标，代表某个可变的瞬时值。适合表示如内存使用率、连接数等。

```go
// 创建仪表盘指标
gauge := metrics.NewGauge(
    "redis_memory_usage_percent",
    75.5,
    map[string]string{"instance": "cache01"},
    "Redis内存使用百分比",
)
```

### 3. 直方图 (Histogram)

对观测值进行采样，并在可配置的桶中对其进行计数。适合表示如请求响应时间分布等。

```go
// 创建直方图指标
histogram := metrics.NewHistogram(
    "es_search_latency_ms",
    []float64{5, 10, 25, 50, 100, 250, 500, 1000},
    map[string]int{
        "5": 100,    // 5ms以下有100个请求
        "10": 150,   // 5-10ms有150个请求
        "25": 200,   // 10-25ms有200个请求
        ... 
    },
    map[string]string{"operation": "search"},
    "Elasticsearch搜索延迟分布(毫秒)",
)
```

### 4. 摘要 (Summary)

类似直方图，但提供分位数计算。适合需要精确百分位数的场景。

```go
// 创建摘要指标
summary := metrics.NewSummary(
    "mysql_query_time_summary",
    map[string]float64{
        "0.5": 12.3,  // 50分位数(中位数)为12.3ms
        "0.9": 45.6,  // 90分位数为45.6ms
        "0.99": 78.9, // 99分位数为78.9ms
    },
    100, // 样本数
    42.5, // 平均值
    map[string]string{"query_type": "select"},
    "MySQL查询时间分布摘要(毫秒)",
)
```

## 指标数据模型

### 1. 指标命名

指标名称遵循`{service}_{subsystem}_{metric_name}_{unit}`的命名规则:

- `service`: 服务名，如mysql, redis, elasticsearch
- `subsystem`: 子系统名，如connection, query, memory
- `metric_name`: 指标名，如count, time, ratio
- `unit`: 单位(可选)，如bytes, seconds, percent

例如: `mysql_connection_active_count`, `redis_memory_used_bytes`

### 2. 标签使用

标签用于为指标添加维度，便于分析和过滤:

- 通用标签: 适用于大多数指标，如`environment`, `service`, `instance`
- 特定标签: 适用于特定指标，如MySQL的`database`, Redis的`db`
- 动态标签: 根据监控对象动态生成，如Elasticsearch的`node_name`, `index_name`

### 3. 指标元数据

每个指标包含以下元数据:

- 名称: 唯一标识符
- 描述: 人类可读的说明
- 类型: 计数器、仪表盘、直方图或摘要
- 单位: 度量单位
- 来源: 指标收集的源头

## 配置参数

### 1. MySQL指标收集器配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| Collections | []string | 要收集的指标集合 | ["performance_schema", "query_stats"] |
| Timeout | time.Duration | 收集超时时间 | 5s |
| DetailLevel | int | 收集详细程度(1-3) | 2 |
| UsePerformanceSchema | bool | 是否使用performance_schema | true |
| Labels | map[string]string | 自定义标签 | nil |
| Logger | Logger | 自定义日志记录器 | nil |

### 2. Redis指标收集器配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| Collections | []string | 要收集的指标集合 | ["memory", "clients", "commands"] |
| Timeout | time.Duration | 收集超时时间 | 3s |
| DetailLevel | int | 收集详细程度(1-3) | 2 |
| IncludeCommandStats | bool | 是否包含命令统计 | true |
| Labels | map[string]string | 自定义标签 | nil |
| Logger | Logger | 自定义日志记录器 | nil |

### 3. Elasticsearch指标收集器配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| Collections | []string | 要收集的指标集合 | ["cluster_health", "node_stats"] |
| Timeout | time.Duration | 收集超时时间 | 10s |
| DetailLevel | int | 收集详细程度(1-3) | 2 |
| SelectedIndices | []string | 选择的索引列表 | [] (全部) |
| Labels | map[string]string | 自定义标签 | nil |
| Logger | Logger | 自定义日志记录器 | nil |

### 4. 过滤处理器配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| IncludePatterns | []string | 包含模式列表 | [] |
| ExcludePatterns | []string | 排除模式列表 | [] |
| IncludeTypes | []MetricType | 包含的指标类型 | [] (全部) |
| CaseSensitive | bool | 是否区分大小写 | false |

### 5. 聚合处理器配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| Aggregations | map[string]Aggregation | 聚合配置 | nil |
| GroupByLabels | []string | 按标签分组 | [] |
| OutputPrefix | string | 输出指标前缀 | "" |
| PreserveSource | bool | 是否保留原始指标 | false |

### 6. 转换处理器配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| Transformations | map[string]Transformation | 转换配置 | nil |
| AddLabels | map[string]string | 添加的标签 | nil |
| RemoveLabels | []string | 移除的标签 | [] |
| RenameMetrics | map[string]string | 指标重命名 | nil |

### 7. 指标服务配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| Interval | time.Duration | 定期收集间隔 | 1m |
| BufferSize | int | 指标缓冲区大小 | 1000 |
| FailurePolicy | FailurePolicy | 失败策略 | ContinueOnError |
| Labels | map[string]string | 全局标签 | nil |
| Timeout | time.Duration | 全局超时 | 30s |
| MaxConcurrency | int | 最大并发收集数 | 3 |
| Logger | Logger | 自定义日志记录器 | nil |

## 最佳实践

1. **规划指标维度**
   - 确定业务关注的关键指标
   - 为不同环境和场景设计合理的标签体系
   - 避免过多的标签导致指标爆炸

2. **优化收集性能**
   - 设置合适的收集间隔和超时时间
   - 选择性收集真正需要的指标集合
   - 使用异步收集减少阻塞

3. **合理处理指标**
   - 使用过滤器移除不相关指标
   - 用聚合处理器降低指标基数
   - 通过转换处理器标准化数据格式

4. **与监控系统集成**
   - 将指标导出到专业监控系统(如Prometheus)
   - 建立基于指标的告警规则
   - 创建可视化仪表盘展示趋势

5. **指标存储管理**
   - 实施合理的指标保留策略
   - 考虑不同时间精度的采样率
   - 规划高可用的指标存储方案

6. **异常值处理**
   - 识别和标记异常指标
   - 实现离群值检测机制
   - 预防指标收集故障的影响 