# 存储监控模块

## 概述

存储监控模块(`pkg/storage/monitor`)提供对各种存储服务的运行状态、性能指标和资源使用情况的实时监控功能。该模块支持MySQL、Redis和Elasticsearch等存储服务的监控，并允许将监控数据导出到不同的监控系统，如Prometheus、日志文件或自定义处理程序。

## 核心组件

### 1. 监控接口 (interface.go)

定义了所有监控实现必须满足的统一接口，便于系统统一管理不同类型的存储监控。

```go
// Monitor 监控接口
type Monitor interface {
    // Start 启动监控
    Start() error
    
    // Stop 停止监控
    Stop() error
    
    // IsRunning 检查监控是否正在运行
    IsRunning() bool
    
    // SetInterval 设置监控间隔
    SetInterval(interval time.Duration)
    
    // RegisterExporter 注册指标导出器
    RegisterExporter(exporter Exporter) error
    
    // GetSnapshot 获取最新的监控快照
    GetSnapshot() (Snapshot, error)
}

// Snapshot 监控快照接口
type Snapshot interface {
    // GetTimestamp 获取快照时间
    GetTimestamp() time.Time
    
    // GetMetrics 获取指标数据
    GetMetrics() map[string]interface{}
    
    // GetLabels 获取标签
    GetLabels() map[string]string
    
    // GetName 获取服务名称
    GetName() string
    
    // GetType 获取服务类型
    GetType() string
}

// Exporter 指标导出接口
type Exporter interface {
    // Export 导出监控数据
    Export(snapshot Snapshot) error
    
    // Name 导出器名称
    Name() string
    
    // Close 关闭导出器
    Close() error
}
```

### 2. MySQL监控 (mysql.go)

实现MySQL数据库的性能指标采集，包括连接使用情况、查询性能、InnoDB状态等。

```go
// MySQLMonitor MySQL监控
type MySQLMonitor struct {
    db        *sql.DB
    name      string
    exporters []Exporter
    interval  time.Duration
    running   bool
    // 其他字段...
}

// NewMySQLMonitor 创建MySQL监控
func NewMySQLMonitor(name string, db *sql.DB, options ...MySQLMonitorOption) (*MySQLMonitor, error) {
    // ...
}
```

### 3. Redis监控 (redis.go)

实现Redis服务的性能指标采集，包括内存使用情况、命令执行统计、客户端连接等。

```go
// RedisMonitor Redis监控
type RedisMonitor struct {
    client    *redis.Client
    name      string
    exporters []Exporter
    interval  time.Duration
    running   bool
    // 其他字段...
}

// NewRedisMonitor 创建Redis监控
func NewRedisMonitor(name string, client *redis.Client, options ...RedisMonitorOption) (*RedisMonitor, error) {
    // ...
}
```

### 4. Elasticsearch监控 (elasticsearch.go)

实现Elasticsearch服务的性能指标采集，包括集群状态、节点统计、索引性能等。

```go
// ESMonitor Elasticsearch监控
type ESMonitor struct {
    client    *elasticsearch.Client
    name      string
    exporters []Exporter
    interval  time.Duration
    running   bool
    // 其他字段...
}

// NewESMonitor 创建Elasticsearch监控
func NewESMonitor(name string, client *elasticsearch.Client, options ...ESMonitorOption) (*ESMonitor, error) {
    // ...
}
```

### 5. 指标导出器 (exporters/)

提供多种指标导出实现，用于将监控数据发送到不同的目标系统。

#### 5.1 Prometheus导出器 (exporters/prometheus.go)

将监控数据导出为Prometheus格式，通过HTTP端点暴露。

```go
// PrometheusExporter Prometheus指标导出器
type PrometheusExporter struct {
    registry *prometheus.Registry
    addr     string
    server   *http.Server
    // 其他字段...
}

// NewPrometheusExporter 创建Prometheus导出器
func NewPrometheusExporter(options ...PrometheusOption) (*PrometheusExporter, error) {
    // ...
}
```

#### 5.2 日志导出器 (exporters/log.go)

将监控数据记录到日志文件。

```go
// LogExporter 日志导出器
type LogExporter struct {
    logger Logger
    level  LogLevel
    format string
    // 其他字段...
}

// NewLogExporter 创建日志导出器
func NewLogExporter(options ...LogExporterOption) (*LogExporter, error) {
    // ...
}
```

#### 5.3 自定义回调导出器 (exporters/callback.go)

通过回调函数处理监控数据，允许自定义处理逻辑。

```go
// CallbackExporter 回调函数导出器
type CallbackExporter struct {
    callback func(Snapshot) error
    name     string
    // 其他字段...
}

// NewCallbackExporter 创建回调函数导出器
func NewCallbackExporter(name string, callback func(Snapshot) error) (*CallbackExporter, error) {
    // ...
}
```

### 6. 监控管理器 (manager.go)

管理多个监控器，提供统一的控制和管理接口。

```go
// MonitorManager 监控管理器
type MonitorManager struct {
    monitors map[string]Monitor
    // 其他字段...
}

// NewMonitorManager 创建监控管理器
func NewMonitorManager() *MonitorManager {
    // ...
}

// RegisterMonitor 注册监控器
func (m *MonitorManager) RegisterMonitor(name string, monitor Monitor) error {
    // ...
}

// GetMonitor 获取监控器
func (m *MonitorManager) GetMonitor(name string) (Monitor, bool) {
    // ...
}

// StartAll 启动所有监控器
func (m *MonitorManager) StartAll() map[string]error {
    // ...
}

// StopAll 停止所有监控器
func (m *MonitorManager) StopAll() map[string]error {
    // ...
}

// GetAllSnapshots 获取所有监控快照
func (m *MonitorManager) GetAllSnapshots() map[string]Snapshot {
    // ...
}
```

## 使用示例

### 1. MySQL监控

```go
// 创建MySQL监控器
db, _ := sql.Open("mysql", "user:password@tcp(127.0.0.1:3306)/dbname")
mysqlMonitor, err := monitor.NewMySQLMonitor("main_db", db,
    monitor.WithMySQLInterval(30*time.Second),
    monitor.WithMySQLCollectTableStats(true),
    monitor.WithMySQLCollectInnoDBStats(true),
    monitor.WithMySQLCollectGlobalStats(true),
)
if err != nil {
    log.Fatalf("创建MySQL监控器失败: %v", err)
}

// 创建Prometheus导出器
promExporter, err := monitor.NewPrometheusExporter(
    monitor.WithPrometheusAddr(":9090"),
    monitor.WithPrometheusNamespace("app_mysql"),
)
if err != nil {
    log.Fatalf("创建Prometheus导出器失败: %v", err)
}

// 创建日志导出器
logExporter, err := monitor.NewLogExporter(
    monitor.WithLogLevel(monitor.LogLevelInfo),
    monitor.WithLogFormat("json"),
)
if err != nil {
    log.Fatalf("创建日志导出器失败: %v", err)
}

// 注册导出器
mysqlMonitor.RegisterExporter(promExporter)
mysqlMonitor.RegisterExporter(logExporter)

// 启动监控
if err := mysqlMonitor.Start(); err != nil {
    log.Fatalf("启动MySQL监控失败: %v", err)
}
defer mysqlMonitor.Stop()

// 获取监控快照
snapshot, err := mysqlMonitor.GetSnapshot()
if err != nil {
    log.Printf("获取监控快照失败: %v", err)
} else {
    metrics := snapshot.GetMetrics()
    log.Printf("当前最大连接数: %v", metrics["max_connections"])
    log.Printf("活跃连接数: %v", metrics["threads_connected"])
    log.Printf("等待连接数: %v", metrics["threads_waiting"])
    log.Printf("慢查询数: %v", metrics["slow_queries"])
}
```

### 2. Redis监控

```go
// 创建Redis监控器
client := redis.NewClient(&redis.Options{
    Addr:     "localhost:6379",
    Password: "password",
    DB:       0,
})
redisMonitor, err := monitor.NewRedisMonitor("cache", client,
    monitor.WithRedisInterval(15*time.Second),
    monitor.WithRedisCollectCommandStats(true),
    monitor.WithRedisCollectMemoryStats(true),
    monitor.WithRedisCollectKeyspaceStats(true),
)
if err != nil {
    log.Fatalf("创建Redis监控器失败: %v", err)
}

// 创建自定义回调导出器
callbackExporter := monitor.NewCallbackExporter("custom_callback", func(s monitor.Snapshot) error {
    metrics := s.GetMetrics()
    
    // 内存使用超过80%告警
    if memoryUsage, ok := metrics["memory_usage_percent"].(float64); ok && memoryUsage > 80 {
        log.Printf("[ALERT] Redis内存使用率高: %.2f%%", memoryUsage)
    }
    
    // 客户端连接数超过1000告警
    if connectedClients, ok := metrics["connected_clients"].(int64); ok && connectedClients > 1000 {
        log.Printf("[ALERT] Redis客户端连接数过多: %d", connectedClients)
    }
    
    return nil
})

// 注册导出器
redisMonitor.RegisterExporter(callbackExporter)

// 启动监控
if err := redisMonitor.Start(); err != nil {
    log.Fatalf("启动Redis监控失败: %v", err)
}
defer redisMonitor.Stop()
```

### 3. Elasticsearch监控

```go
// 创建Elasticsearch监控器
cfg := elasticsearch.Config{
    Addresses: []string{"http://localhost:9200"},
    Username:  "user",
    Password:  "password",
}
client, _ := elasticsearch.NewClient(cfg)
esMonitor, err := monitor.NewESMonitor("search", client,
    monitor.WithESInterval(60*time.Second),
    monitor.WithESCollectClusterStats(true),
    monitor.WithESCollectNodeStats(true),
    monitor.WithESCollectIndicesStats(true),
    monitor.WithESCollectJVMStats(true),
)
if err != nil {
    log.Fatalf("创建Elasticsearch监控器失败: %v", err)
}

// 创建Prometheus导出器
promExporter, _ := monitor.NewPrometheusExporter(
    monitor.WithPrometheusAddr(":9091"),
    monitor.WithPrometheusNamespace("app_es"),
)

// 注册导出器
esMonitor.RegisterExporter(promExporter)

// 启动监控
if err := esMonitor.Start(); err != nil {
    log.Fatalf("启动Elasticsearch监控失败: %v", err)
}
defer esMonitor.Stop()
```

### 4. 使用监控管理器

```go
// 创建监控管理器
manager := monitor.NewMonitorManager()

// 注册监控器
manager.RegisterMonitor("main_db", mysqlMonitor)
manager.RegisterMonitor("cache", redisMonitor)
manager.RegisterMonitor("search", esMonitor)

// 启动所有监控器
errors := manager.StartAll()
for name, err := range errors {
    if err != nil {
        log.Printf("启动监控器 %s 失败: %v", name, err)
    }
}

// 获取所有监控快照
snapshots := manager.GetAllSnapshots()
for name, snapshot := range snapshots {
    log.Printf("服务 %s 的监控数据:", name)
    for key, value := range snapshot.GetMetrics() {
        log.Printf("  %s: %v", key, value)
    }
}

// 应用关闭时停止所有监控器
defer manager.StopAll()
```

## 监控指标

### 1. MySQL监控指标

| 指标类别 | 指标名称 | 说明 | 类型 |
|---------|---------|------|------|
| 连接统计 | max_connections | 最大连接数 | int64 |
| 连接统计 | threads_connected | 当前连接线程数 | int64 |
| 连接统计 | threads_running | 当前活动线程数 | int64 |
| 连接统计 | connection_errors_total | 连接错误总数 | int64 |
| 查询统计 | questions_total | 查询总数 | int64 |
| 查询统计 | slow_queries | 慢查询数 | int64 |
| 查询统计 | selects_total | SELECT查询数 | int64 |
| 查询统计 | inserts_total | INSERT查询数 | int64 |
| 查询统计 | updates_total | UPDATE查询数 | int64 |
| 查询统计 | deletes_total | DELETE查询数 | int64 |
| InnoDB统计 | innodb_buffer_pool_pages_total | 缓冲池总页数 | int64 |
| InnoDB统计 | innodb_buffer_pool_pages_free | 缓冲池空闲页数 | int64 |
| InnoDB统计 | innodb_row_lock_waits | 行锁等待次数 | int64 |
| InnoDB统计 | innodb_row_lock_time_avg | 行锁平均等待时间 | float64 |
| 表统计 | table_open_cache | 表缓存大小 | int64 |
| 表统计 | open_tables | 当前打开的表数 | int64 |
| 表统计 | opened_tables | 已打开表的数量 | int64 |

### 2. Redis监控指标

| 指标类别 | 指标名称 | 说明 | 类型 |
|---------|---------|------|------|
| 内存统计 | used_memory | 已用内存 | int64 |
| 内存统计 | used_memory_peak | 内存使用峰值 | int64 |
| 内存统计 | memory_usage_percent | 内存使用百分比 | float64 |
| 内存统计 | used_memory_rss | 物理内存使用量 | int64 |
| 客户端统计 | connected_clients | 连接的客户端数 | int64 |
| 客户端统计 | blocked_clients | 阻塞的客户端数 | int64 |
| 客户端统计 | client_longest_output_list | 最长输出列表 | int64 |
| 命令统计 | total_commands_processed | 处理的命令总数 | int64 |
| 命令统计 | instantaneous_ops_per_sec | 每秒执行命令数 | int64 |
| 命令统计 | rejected_connections | 拒绝的连接数 | int64 |
| 键统计 | expired_keys | 过期的键数量 | int64 |
| 键统计 | evicted_keys | 驱逐的键数量 | int64 |
| 键统计 | keyspace_hits | 键命中次数 | int64 |
| 键统计 | keyspace_misses | 键未命中次数 | int64 |
| 复制统计 | role | 实例角色(master/slave) | string |
| 复制统计 | connected_slaves | 连接的从节点数 | int64 |
| 复制统计 | master_link_status | 主链接状态 | string |

### 3. Elasticsearch监控指标

| 指标类别 | 指标名称 | 说明 | 类型 |
|---------|---------|------|------|
| 集群统计 | cluster_status | 集群状态(green/yellow/red) | string |
| 集群统计 | number_of_nodes | 节点数 | int64 |
| 集群统计 | active_primary_shards | 活跃主分片数 | int64 |
| 集群统计 | active_shards | 活跃分片总数 | int64 |
| 集群统计 | relocating_shards | 正在重定位的分片数 | int64 |
| 集群统计 | initializing_shards | 正在初始化的分片数 | int64 |
| 集群统计 | unassigned_shards | 未分配的分片数 | int64 |
| 节点统计 | jvm_heap_used_percent | JVM堆使用百分比 | float64 |
| 节点统计 | jvm_heap_used_bytes | JVM堆使用字节数 | int64 |
| 节点统计 | jvm_heap_max_bytes | JVM堆最大字节数 | int64 |
| 节点统计 | os_cpu_percent | 系统CPU使用率 | float64 |
| 节点统计 | process_cpu_percent | 进程CPU使用率 | float64 |
| 节点统计 | fs_total_bytes | 文件系统总大小 | int64 |
| 节点统计 | fs_available_bytes | 文件系统可用大小 | int64 |
| 索引统计 | indices_count | 索引数量 | int64 |
| 索引统计 | docs_count | 文档数量 | int64 |
| 索引统计 | store_size_bytes | 存储大小 | int64 |
| 索引统计 | search_query_total | 搜索查询总数 | int64 |
| 索引统计 | search_query_time_ms | 搜索查询总时间 | int64 |
| 索引统计 | indexing_index_total | 索引操作总数 | int64 |
| 索引统计 | indexing_index_time_ms | 索引操作总时间 | int64 |

## 配置参数

### 1. MySQL监控配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| Interval | time.Duration | 监控间隔 | 30s |
| CollectGlobalStats | bool | 是否采集全局状态 | true |
| CollectTableStats | bool | 是否采集表统计 | true |
| CollectInnoDBStats | bool | 是否采集InnoDB统计 | true |
| CollectSlaveStats | bool | 是否采集从节点统计 | false |
| CollectQueryResponseTime | bool | 是否采集查询响应时间 | false |
| Labels | map[string]string | 自定义标签 | nil |
| Logger | Logger | 自定义日志记录器 | nil |

### 2. Redis监控配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| Interval | time.Duration | 监控间隔 | 15s |
| CollectMemoryStats | bool | 是否采集内存统计 | true |
| CollectClientStats | bool | 是否采集客户端统计 | true |
| CollectCommandStats | bool | 是否采集命令统计 | true |
| CollectKeyspaceStats | bool | 是否采集键空间统计 | true |
| CollectReplicationStats | bool | 是否采集复制统计 | true |
| CollectClusterStats | bool | 是否采集集群统计 | false |
| Labels | map[string]string | 自定义标签 | nil |
| Logger | Logger | 自定义日志记录器 | nil |

### 3. Elasticsearch监控配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| Interval | time.Duration | 监控间隔 | 60s |
| CollectClusterStats | bool | 是否采集集群统计 | true |
| CollectNodeStats | bool | 是否采集节点统计 | true |
| CollectIndicesStats | bool | 是否采集索引统计 | true |
| CollectJVMStats | bool | 是否采集JVM统计 | true |
| CollectThreadPoolStats | bool | 是否采集线程池统计 | false |
| SelectedIndices | []string | 选择监控的索引 | [] (全部) |
| Labels | map[string]string | 自定义标签 | nil |
| Logger | Logger | 自定义日志记录器 | nil |

### 4. Prometheus导出器配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| Addr | string | 监听地址 | ":9090" |
| Path | string | 指标路径 | "/metrics" |
| Namespace | string | 指标名称空间 | "app" |
| Subsystem | string | 指标子系统 | "" |
| Labels | map[string]string | 全局标签 | nil |

### 5. 日志导出器配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| Level | LogLevel | 日志级别 | LogLevelInfo |
| Format | string | 日志格式 | "text" |
| OutputPath | string | 输出路径 | "" (标准输出) |
| RotateSize | int64 | 日志轮转大小 | 0 (不轮转) |
| MaxFiles | int | 最大文件数 | 0 (不限制) |
| FlushInterval | time.Duration | 刷新间隔 | 5s |

## 最佳实践

1. **合理设置监控间隔**
   - 根据服务重要性和性能影响设置适当的监控间隔
   - 关键服务可以设置较短的间隔，非关键服务可以设置较长的间隔
   - 避免过于频繁的监控对服务造成性能影响

2. **选择合适的监控指标**
   - 启用最相关的监控指标，禁用不必要的指标
   - 减少不必要的指标采集可以降低监控开销
   - 针对不同的存储服务关注其关键指标

3. **设置有意义的标签**
   - 使用标签区分不同的服务实例、环境和用途
   - 标签命名要有意义且一致
   - 避免使用过多的标签，以免增加存储和查询开销

4. **有效使用导出器**
   - 根据监控需求选择合适的导出器
   - Prometheus适合长期趋势分析和告警
   - 日志导出器适合详细记录和问题排查
   - 回调导出器适合自定义处理逻辑和即时响应

5. **监控数据可视化**
   - 将监控数据与Grafana等工具集成
   - 创建直观的仪表盘展示关键指标
   - 设置适当的阈值和告警规则

6. **资源管理**
   - 在应用关闭时正确停止监控器和导出器
   - 管理监控数据的存储和过期策略
   - 定期检查监控系统自身的资源使用情况 