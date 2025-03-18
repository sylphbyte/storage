// Package health 提供存储服务的健康检查功能
package health

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
)

// ESChecker Elasticsearch健康检查器
type ESChecker struct {
	*Checker
	client        *elasticsearch.Client // ES客户端
	pingTimeout   time.Duration         // ping超时时间
	checkSettings ESCheckSettings       // 检查设置
}

// ESCheckSettings Elasticsearch健康检查设置
type ESCheckSettings struct {
	CheckClusterHealth bool     // 是否检查集群健康状态
	CheckNodeStats     bool     // 是否检查节点状态
	CheckIndices       bool     // 是否检查索引状态
	TestSearchIndex    string   // 测试搜索的索引
	TestSearchQuery    string   // 测试搜索的查询
	VitalIndices       []string // 关键索引列表
	MinActiveShards    float64  // 最小活跃分片百分比
	MaxJvmUsagePct     float64  // JVM最大使用率阈值
	MaxDiskUsagePct    float64  // 磁盘最大使用率阈值
	SlowQueryMs        int64    // 慢查询阈值（毫秒）
}

// DefaultESCheckSettings 创建默认ES检查设置
func DefaultESCheckSettings() ESCheckSettings {
	return ESCheckSettings{
		CheckClusterHealth: true,
		CheckNodeStats:     true,
		CheckIndices:       true,
		TestSearchIndex:    "_all",
		TestSearchQuery:    `{"query":{"match_all":{}}}`,
		VitalIndices:       []string{},
		MinActiveShards:    80.0,
		MaxJvmUsagePct:     85.0,
		MaxDiskUsagePct:    85.0,
		SlowQueryMs:        200,
	}
}

// NewESChecker 创建新的ES健康检查器
func NewESChecker(name string, client *elasticsearch.Client, settings *ESCheckSettings) *ESChecker {
	if settings == nil {
		defaultSettings := DefaultESCheckSettings()
		settings = &defaultSettings
	}

	return &ESChecker{
		Checker:       NewChecker(name, "elasticsearch"),
		client:        client,
		pingTimeout:   5 * time.Second,
		checkSettings: *settings,
	}
}

// Check 执行ES健康检查
func (e *ESChecker) Check(ctx context.Context) (HealthStatus, error) {
	startTime := time.Now()

	// 创建健康状态对象
	healthStatus := NewHealthStatus(StatusHealthy)

	// 添加健康检查基本信息
	healthStatus.Details["service_type"] = "elasticsearch"
	healthStatus.Details["check_name"] = e.Name()

	// 创建带超时的上下文
	timeoutCtx, cancel := context.WithTimeout(ctx, e.pingTimeout)
	defer cancel()

	// 基本连接测试
	pingReq := esapi.PingRequest{}
	pingRes, pingErr := pingReq.Do(timeoutCtx, e.client)

	if pingErr != nil {
		healthStatus.Status = StatusUnhealthy
		healthStatus.Details["ping_error"] = pingErr.Error()
		healthStatus.Latency = time.Since(startTime)

		e.RecordResult(healthStatus, pingErr)
		return healthStatus, errors.Wrap(pingErr, "Elasticsearch ping 失败")
	}

	defer pingRes.Body.Close()

	if pingRes.IsError() {
		healthStatus.Status = StatusUnhealthy
		healthStatus.Details["ping_status"] = fmt.Sprintf("%d", pingRes.StatusCode)
		healthStatus.Latency = time.Since(startTime)

		e.RecordResult(healthStatus, errors.New(fmt.Sprintf("Elasticsearch ping返回错误状态: %d", pingRes.StatusCode)))
		return healthStatus, errors.New(fmt.Sprintf("Elasticsearch ping返回错误状态: %d", pingRes.StatusCode))
	}

	healthStatus.Details["ping"] = "successful"

	// 检查集群健康状态
	if e.checkSettings.CheckClusterHealth {
		err := e.checkClusterHealth(timeoutCtx, &healthStatus)
		if err != nil {
			pr.Warning("Elasticsearch集群健康检查失败: %v", err)
		}
	}

	// 检查节点状态
	if e.checkSettings.CheckNodeStats {
		err := e.checkNodeStats(timeoutCtx, &healthStatus)
		if err != nil {
			pr.Warning("Elasticsearch节点状态检查失败: %v", err)
		}
	}

	// 检查索引状态
	if e.checkSettings.CheckIndices {
		err := e.checkIndices(timeoutCtx, &healthStatus)
		if err != nil {
			pr.Warning("Elasticsearch索引状态检查失败: %v", err)
		}
	}

	// 执行测试搜索
	err := e.testSearch(timeoutCtx, &healthStatus)
	if err != nil {
		pr.Warning("Elasticsearch测试搜索失败: %v", err)
	}

	// 设置检查延迟
	healthStatus.Latency = time.Since(startTime)

	// 记录检查结果
	e.RecordResult(healthStatus, nil)

	return healthStatus, nil
}

// checkClusterHealth 检查ES集群健康状态
func (e *ESChecker) checkClusterHealth(ctx context.Context, healthStatus *HealthStatus) error {
	req := esapi.ClusterHealthRequest{
		Level: "shards",
	}

	res, err := req.Do(ctx, e.client)
	if err != nil {
		healthStatus.Details["cluster_health_error"] = err.Error()
		return errors.Wrap(err, "执行集群健康检查请求失败")
	}
	defer res.Body.Close()

	if res.IsError() {
		healthStatus.Details["cluster_health_status"] = fmt.Sprintf("%d", res.StatusCode)
		return errors.New(fmt.Sprintf("集群健康检查返回错误状态: %d", res.StatusCode))
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "读取集群健康检查响应失败")
	}

	var healthResponse map[string]interface{}
	if err := json.Unmarshal(body, &healthResponse); err != nil {
		return errors.Wrap(err, "解析集群健康检查响应失败")
	}

	// 获取并记录集群状态
	if status, ok := healthResponse["status"].(string); ok {
		healthStatus.Details["cluster_status"] = status

		// 检查集群状态，并相应更新健康状态
		if status == "red" {
			healthStatus.Status = StatusUnhealthy
			healthStatus.Details["degraded_reason"] = "集群状态为red"
		} else if status == "yellow" && healthStatus.Status == StatusHealthy {
			healthStatus.Status = StatusDegraded
			healthStatus.Details["degraded_reason"] = "集群状态为yellow"
		}
	}

	// 记录活跃分片百分比
	if activeShardsPercentAsFloat, ok := healthResponse["active_shards_percent_as_number"].(float64); ok {
		healthStatus.Details["active_shards_percent"] = fmt.Sprintf("%.2f", activeShardsPercentAsFloat)

		// 如果活跃分片百分比低于阈值，标记为降级状态
		if activeShardsPercentAsFloat < e.checkSettings.MinActiveShards && healthStatus.Status == StatusHealthy {
			healthStatus.Status = StatusDegraded
			healthStatus.Details["degraded_reason"] = fmt.Sprintf(
				"活跃分片百分比低: %.2f%% < %.2f%%",
				activeShardsPercentAsFloat,
				e.checkSettings.MinActiveShards,
			)
		}
	}

	// 记录其他关键指标
	metrics := []string{
		"number_of_nodes", "number_of_data_nodes", "active_primary_shards",
		"active_shards", "relocating_shards", "initializing_shards", "unassigned_shards",
		"delayed_unassigned_shards", "number_of_pending_tasks", "task_max_waiting_in_queue_millis",
	}

	for _, metric := range metrics {
		if value, ok := healthResponse[metric]; ok {
			healthStatus.Details[metric] = fmt.Sprintf("%v", value)
		}
	}

	return nil
}

// checkNodeStats 检查ES节点状态
func (e *ESChecker) checkNodeStats(ctx context.Context, healthStatus *HealthStatus) error {
	req := esapi.NodesStatsRequest{
		Metric: []string{"jvm", "os", "fs"},
	}

	res, err := req.Do(ctx, e.client)
	if err != nil {
		healthStatus.Details["node_stats_error"] = err.Error()
		return errors.Wrap(err, "执行节点状态请求失败")
	}
	defer res.Body.Close()

	if res.IsError() {
		healthStatus.Details["node_stats_status"] = fmt.Sprintf("%d", res.StatusCode)
		return errors.New(fmt.Sprintf("节点状态检查返回错误状态: %d", res.StatusCode))
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "读取节点状态响应失败")
	}

	var nodeStatsResponse map[string]interface{}
	if err := json.Unmarshal(body, &nodeStatsResponse); err != nil {
		return errors.Wrap(err, "解析节点状态响应失败")
	}

	// 解析节点信息
	if nodes, ok := nodeStatsResponse["nodes"].(map[string]interface{}); ok {
		nodeCount := len(nodes)
		healthStatus.Details["node_count"] = fmt.Sprintf("%d", nodeCount)

		// 遍历所有节点
		maxJvmUsage := 0.0
		maxDiskUsage := 0.0

		for nodeID, nodeData := range nodes {
			if nodeInfo, ok := nodeData.(map[string]interface{}); ok {
				// 检查JVM内存使用情况
				if jvm, ok := nodeInfo["jvm"].(map[string]interface{}); ok {
					if mem, ok := jvm["mem"].(map[string]interface{}); ok {
						if heapUsedPct, ok := mem["heap_used_percent"].(float64); ok {
							healthStatus.Details[fmt.Sprintf("node_%s_jvm_usage", nodeID)] = fmt.Sprintf("%.2f", heapUsedPct)

							if heapUsedPct > maxJvmUsage {
								maxJvmUsage = heapUsedPct
							}
						}
					}
				}

				// 检查磁盘使用情况
				if fs, ok := nodeInfo["fs"].(map[string]interface{}); ok {
					if total, ok := fs["total"].(map[string]interface{}); ok {
						var totalBytes, freeBytes float64

						if totalInBytes, ok := total["total_in_bytes"].(float64); ok {
							totalBytes = totalInBytes
						}

						if freeInBytes, ok := total["free_in_bytes"].(float64); ok {
							freeBytes = freeInBytes
						}

						if totalBytes > 0 {
							diskUsagePct := (1 - (freeBytes / totalBytes)) * 100
							healthStatus.Details[fmt.Sprintf("node_%s_disk_usage", nodeID)] = fmt.Sprintf("%.2f", diskUsagePct)

							if diskUsagePct > maxDiskUsage {
								maxDiskUsage = diskUsagePct
							}
						}
					}
				}
			}
		}

		// 记录最大使用率
		healthStatus.Details["max_jvm_usage_pct"] = fmt.Sprintf("%.2f", maxJvmUsage)
		healthStatus.Details["max_disk_usage_pct"] = fmt.Sprintf("%.2f", maxDiskUsage)

		// 检查是否超过阈值
		if maxJvmUsage > e.checkSettings.MaxJvmUsagePct && healthStatus.Status == StatusHealthy {
			healthStatus.Status = StatusDegraded
			healthStatus.Details["degraded_reason"] = fmt.Sprintf(
				"JVM内存使用率高: %.2f%% > %.2f%%",
				maxJvmUsage,
				e.checkSettings.MaxJvmUsagePct,
			)
		}

		if maxDiskUsage > e.checkSettings.MaxDiskUsagePct && healthStatus.Status == StatusHealthy {
			healthStatus.Status = StatusDegraded
			healthStatus.Details["degraded_reason"] = fmt.Sprintf(
				"磁盘使用率高: %.2f%% > %.2f%%",
				maxDiskUsage,
				e.checkSettings.MaxDiskUsagePct,
			)
		}
	}

	return nil
}

// checkIndices 检查ES索引状态
func (e *ESChecker) checkIndices(ctx context.Context, healthStatus *HealthStatus) error {
	// 获取关键索引状态
	if len(e.checkSettings.VitalIndices) > 0 {
		for _, index := range e.checkSettings.VitalIndices {
			req := esapi.IndicesStatsRequest{
				Index: []string{index},
			}

			res, err := req.Do(ctx, e.client)
			if err != nil {
				healthStatus.Details[fmt.Sprintf("index_%s_error", index)] = err.Error()
				continue
			}

			if res.IsError() {
				healthStatus.Details[fmt.Sprintf("index_%s_status", index)] = fmt.Sprintf("%d", res.StatusCode)

				if res.StatusCode == 404 {
					healthStatus.Status = StatusDegraded
					healthStatus.Details["degraded_reason"] = fmt.Sprintf("关键索引不存在: %s", index)
				}

				res.Body.Close()
				continue
			}

			// 索引存在并可访问
			healthStatus.Details[fmt.Sprintf("index_%s", index)] = "accessible"
			res.Body.Close()
		}
	}

	// 获取所有索引健康状态
	req := esapi.ClusterHealthRequest{
		Level: "indices",
	}

	res, err := req.Do(ctx, e.client)
	if err != nil {
		healthStatus.Details["indices_health_error"] = err.Error()
		return errors.Wrap(err, "执行索引健康检查请求失败")
	}
	defer res.Body.Close()

	if res.IsError() {
		healthStatus.Details["indices_health_status"] = fmt.Sprintf("%d", res.StatusCode)
		return errors.New(fmt.Sprintf("索引健康检查返回错误状态: %d", res.StatusCode))
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "读取索引健康检查响应失败")
	}

	var healthResponse map[string]interface{}
	if err := json.Unmarshal(body, &healthResponse); err != nil {
		return errors.Wrap(err, "解析索引健康检查响应失败")
	}

	// 检查索引健康状态
	if indices, ok := healthResponse["indices"].(map[string]interface{}); ok {
		var redIndices, yellowIndices []string

		for indexName, indexData := range indices {
			if indexInfo, ok := indexData.(map[string]interface{}); ok {
				if status, ok := indexInfo["status"].(string); ok {
					if status == "red" {
						redIndices = append(redIndices, indexName)
					} else if status == "yellow" {
						yellowIndices = append(yellowIndices, indexName)
					}
				}
			}
		}

		healthStatus.Details["red_indices_count"] = fmt.Sprintf("%d", len(redIndices))
		healthStatus.Details["yellow_indices_count"] = fmt.Sprintf("%d", len(yellowIndices))

		if len(redIndices) > 0 {
			healthStatus.Status = StatusUnhealthy
			healthStatus.Details["degraded_reason"] = fmt.Sprintf("发现%d个red状态索引", len(redIndices))

			// 最多记录5个红色索引
			if len(redIndices) > 5 {
				redIndices = redIndices[:5]
			}
			healthStatus.Details["red_indices"] = strings.Join(redIndices, ",")
		} else if len(yellowIndices) > 0 && healthStatus.Status == StatusHealthy {
			healthStatus.Status = StatusDegraded
			healthStatus.Details["degraded_reason"] = fmt.Sprintf("发现%d个yellow状态索引", len(yellowIndices))

			// 最多记录5个黄色索引
			if len(yellowIndices) > 5 {
				yellowIndices = yellowIndices[:5]
			}
			healthStatus.Details["yellow_indices"] = strings.Join(yellowIndices, ",")
		}
	}

	return nil
}

// testSearch 执行测试搜索
func (e *ESChecker) testSearch(ctx context.Context, healthStatus *HealthStatus) error {
	var buf strings.Builder
	buf.WriteString(e.checkSettings.TestSearchQuery)

	// 记录开始时间
	startTime := time.Now()

	req := esapi.SearchRequest{
		Index: []string{e.checkSettings.TestSearchIndex},
		Body:  strings.NewReader(buf.String()),
	}

	res, err := req.Do(ctx, e.client)
	if err != nil {
		healthStatus.Details["search_error"] = err.Error()
		return errors.Wrap(err, "执行测试搜索请求失败")
	}
	defer res.Body.Close()

	// 计算搜索延迟
	searchTime := time.Since(startTime)
	healthStatus.Details["search_time_ms"] = fmt.Sprintf("%.2f", float64(searchTime.Microseconds())/1000)

	if res.IsError() {
		healthStatus.Details["search_status"] = fmt.Sprintf("%d", res.StatusCode)
		healthStatus.Status = StatusDegraded
		healthStatus.Details["degraded_reason"] = fmt.Sprintf("测试搜索失败，状态码: %d", res.StatusCode)
		return errors.New(fmt.Sprintf("测试搜索返回错误状态: %d", res.StatusCode))
	}

	// 解析响应
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "读取测试搜索响应失败")
	}

	var searchResponse map[string]interface{}
	if err := json.Unmarshal(body, &searchResponse); err != nil {
		return errors.Wrap(err, "解析测试搜索响应失败")
	}

	// 检查搜索结果
	if took, ok := searchResponse["took"].(float64); ok {
		healthStatus.Details["search_took_ms"] = fmt.Sprintf("%.0f", took)

		// 检查搜索是否超时
		if took > float64(e.checkSettings.SlowQueryMs) {
			healthStatus.Status = StatusDegraded
			healthStatus.Details["degraded_reason"] = fmt.Sprintf("搜索响应较慢: %.0fms > %dms", took, e.checkSettings.SlowQueryMs)
		}
	}

	if timedOut, ok := searchResponse["timed_out"].(bool); ok && timedOut {
		healthStatus.Status = StatusDegraded
		healthStatus.Details["search_timed_out"] = "true"
		healthStatus.Details["degraded_reason"] = "测试搜索超时"
	}

	// 记录命中数
	if hits, ok := searchResponse["hits"].(map[string]interface{}); ok {
		if total, ok := hits["total"].(map[string]interface{}); ok {
			if value, ok := total["value"].(float64); ok {
				healthStatus.Details["search_hits"] = fmt.Sprintf("%.0f", value)
			}
		} else if total, ok := hits["total"].(float64); ok {
			// 旧版本ES的兼容
			healthStatus.Details["search_hits"] = fmt.Sprintf("%.0f", total)
		}
	}

	return nil
}

// SetPingTimeout 设置Ping超时时间
func (e *ESChecker) SetPingTimeout(timeout time.Duration) {
	e.pingTimeout = timeout
}

// UpdateSettings 更新检查设置
func (e *ESChecker) UpdateSettings(settings ESCheckSettings) {
	e.checkSettings = settings
}
