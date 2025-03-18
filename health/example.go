// Package health 提供存储服务的健康检查功能
package health

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
)

// ExampleUsage 展示如何使用健康检查和告警系统
func ExampleUsage() {
	// 创建健康检查与告警服务
	healthService := NewHealthService(
		ServiceWithCheckInterval(30*time.Second),
		ServiceWithTimeout(5*time.Second),
	)

	// 注册MySQL健康检查器
	// 此处仅为示例，需要替换为实际的数据库连接
	mysqlDB, err := sql.Open("mysql", "user:password@tcp(localhost:3306)/dbname")
	if err != nil {
		pr.Error("连接MySQL数据库失败: %v", err)
		return
	}
	defer mysqlDB.Close()

	// 设置MySQL检查参数
	mysqlSettings := DefaultMySQLCheckSettings()
	mysqlSettings.CheckConnections = true
	mysqlSettings.CheckSlowQueries = true
	mysqlSettings.VitalTables = []string{"users", "orders", "products"}
	mysqlSettings.CheckVitalTables = true

	// 注册MySQL检查器
	err = healthService.RegisterMySQLChecker("main_db", mysqlDB, &mysqlSettings)
	if err != nil {
		pr.Error("注册MySQL健康检查器失败: %v", err)
		return
	}

	// 注册Redis健康检查器
	// 此处仅为示例，需要替换为实际的Redis客户端
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	defer redisClient.Close()

	// 设置Redis检查参数
	redisSettings := DefaultRedisCheckSettings()
	redisSettings.CheckMemory = true
	redisSettings.CheckClients = true
	redisSettings.MemoryHighPct = 0.8
	redisSettings.MemoryCriticalPct = 0.9

	// 注册Redis检查器
	err = healthService.RegisterRedisChecker("cache", redisClient, &redisSettings)
	if err != nil {
		pr.Error("注册Redis健康检查器失败: %v", err)
		return
	}

	// 注册Elasticsearch健康检查器
	// 此处仅为示例，需要替换为实际的ES客户端
	esClient, err := elasticsearch.NewDefaultClient()
	if err != nil {
		pr.Error("创建Elasticsearch客户端失败: %v", err)
		return
	}

	// 设置ES检查参数
	esSettings := DefaultESCheckSettings()
	esSettings.CheckClusterHealth = true
	esSettings.CheckNodeStats = true
	esSettings.VitalIndices = []string{"logs", "metrics"}
	esSettings.CheckIndices = true

	// 注册ES检查器
	err = healthService.RegisterESChecker("search", esClient, &esSettings)
	if err != nil {
		pr.Error("注册Elasticsearch健康检查器失败: %v", err)
		return
	}

	// 创建默认告警规则
	healthService.CreateDefaultAlertRules()

	// 创建自定义告警规则
	customRule := AlertRule{
		ID:           "redis_connection_limit",
		Name:         "Redis连接数接近限制",
		Description:  "Redis服务连接数接近最大限制",
		Level:        AlertLevelWarning,
		ServiceTypes: []string{"redis"},
		Condition:    "status == degraded",
		Message:      "Redis连接数高，请检查连接泄漏",
		Enabled:      true,
	}

	err = healthService.RegisterAlertRule(customRule)
	if err != nil {
		pr.Error("注册自定义告警规则失败: %v", err)
		return
	}

	// 注册文件告警处理器
	fileConfig := FileConfig{
		FilePath:     "/var/log/sylph/alerts.log",
		LevelFilters: []AlertLevel{AlertLevelError, AlertLevelCritical},
		Format:       "text",
		RotateSize:   10 * 1024 * 1024, // 10MB
		MaxFiles:     5,
	}

	err = healthService.RegisterFileHandler("file_handler", fileConfig)
	if err != nil {
		pr.Error("注册文件告警处理器失败: %v", err)
		return
	}

	// 启动健康检查服务
	err = healthService.Start()
	if err != nil {
		pr.Error("启动健康检查服务失败: %v", err)
		return
	}
	defer healthService.Stop()

	// 立即执行一次全量检查
	ctx := context.Background()
	results := healthService.CheckAllNow(ctx)

	// 输出检查结果
	for name, status := range results {
		pr.Info("服务 [%s] 状态: %s", name, status.Status)
	}

	// 获取当前告警
	activeAlerts := healthService.GetActiveAlerts()
	pr.Info("活跃告警数: %d", len(activeAlerts))

	// 查看健康摘要
	summary := healthService.GetHealthSummary()
	pr.Info("健康状态摘要: 总体状态=%s, 健康服务=%d, 降级服务=%d, 不健康服务=%d",
		summary.OverallStatus, summary.HealthyServices, summary.DegradedServices, summary.UnhealthyServices)
}

// ExampleSimpleUsage 简化版的使用示例
func ExampleSimpleUsage() {
	// 创建服务
	service := NewHealthService()

	// 注册MySQL检查器 (以下为示例，需要替换为实际的DB连接)
	db, _ := sql.Open("mysql", "user:password@tcp(127.0.0.1:3306)/test")
	service.RegisterMySQLChecker("main_db", db, nil) // 使用默认设置

	// 注册默认告警规则
	service.CreateDefaultAlertRules()

	// 启动服务
	service.Start()
	defer service.Stop()

	// 检查并处理结果
	ctx := context.Background()
	results := service.CheckAllNow(ctx)

	// 打印检查结果数量
	fmt.Printf("检查了 %d 个服务\n", len(results))

	// 查看健康摘要
	summary := service.GetHealthSummary()
	fmt.Printf("系统健康状态: %s\n", summary.OverallStatus)

	// 处理告警
	alerts := service.GetActiveAlerts()
	for _, alert := range alerts {
		fmt.Printf("告警: %s - %s\n", alert.Name, alert.Message)
		service.AcknowledgeAlert(alert.ID) // 确认告警
	}
}

// CustomCheckerExample 自定义检查器示例
func CustomCheckerExample() {
	// 创建自定义健康检查器
	customChecker := NewCustomChecker()

	// 创建服务
	service := NewHealthService()

	// 注册自定义检查器
	service.RegisterChecker("custom_service", customChecker)

	// 启动服务
	service.Start()
	defer service.Stop()

	// 等待检查完成
	time.Sleep(2 * time.Second)

	// 查看健康状态
	ctx := context.Background()
	status, _ := service.CheckNow(ctx, "custom_service")

	pr.Info("自定义服务状态: %s", status.Status)
}

// CustomChecker 自定义健康检查器示例
type CustomChecker struct {
	*Checker
}

// NewCustomChecker 创建新的自定义检查器
func NewCustomChecker() *CustomChecker {
	return &CustomChecker{
		Checker: NewChecker("custom_service", "custom"),
	}
}

// Check 实现健康检查接口
func (c *CustomChecker) Check(ctx context.Context) (HealthStatus, error) {
	// 此处实现自定义的健康检查逻辑
	// 例如检查某个文件是否存在
	_, err := os.Stat("/tmp/service.lock")

	status := NewHealthStatus(StatusHealthy)
	status = status.WithDetail("service_type", "custom")

	if err != nil {
		if os.IsNotExist(err) {
			status.Status = StatusDegraded
			status = status.WithDetail("message", "服务锁文件不存在")
			return status, nil
		}
		return status, errors.Wrap(err, "检查文件状态失败")
	}

	// 添加自定义详情
	status = status.WithDetail("message", "服务运行正常")
	status = status.WithDetail("checked_at", time.Now().Format(time.RFC3339))

	return status, nil
}
