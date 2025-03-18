package enhanced

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sylphbyte/pr"
	"app/pkg/storage/health"
	"gorm.io/gorm"
)

// ExampleUsage 展示如何使用增强型存储管理器
func ExampleUsage() {
	// 创建增强型存储管理器
	manager := NewStorageManager(
		// 设置健康检查间隔为30秒
		WithHealthCheckInterval(30*time.Second),
		// 设置健康检查超时为3秒
		WithHealthCheckTimeout(3*time.Second),
		// 添加日志告警处理器
		WithLogAlertHandler(),
		// 添加文件告警处理器
		WithFileAlertHandler("./storage_alerts.log"),
		// 添加Webhook告警处理器
		WithWebhookAlertHandler("http://example.com/alerts", map[string]string{
			"Content-Type": "application/json",
			"X-API-Key":    "my-api-key",
		}),
	)

	// 启动健康检查服务
	if err := manager.StartHealthCheck(); err != nil {
		pr.Error("启动健康检查服务失败: %v", err)
		os.Exit(1)
	}
	pr.Info("已启动健康检查服务")

	// 在程序退出时关闭所有连接
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := manager.CloseAll(ctx); err != nil {
			pr.Error("关闭存储连接失败: %v", err)
		}
		pr.Info("已关闭所有存储连接")
	}()

	// 使用管理器获取数据库连接
	db, err := manager.GetDB("main")
	if err != nil {
		pr.Error("获取数据库连接失败: %v", err)
		return
	}
	pr.Info("成功获取数据库连接")

	// 执行数据库操作
	ctx := context.Background()
	err = db.WithTransaction(ctx, func(tx *gorm.DB) error {
		// 执行数据库操作
		return nil
	})
	if err != nil {
		pr.Error("执行数据库操作失败: %v", err)
		return
	}

	// 获取Redis客户端
	redisClient, err := manager.GetRedis("cache")
	if err != nil {
		pr.Error("获取Redis客户端失败: %v", err)
		return
	}
	pr.Info("成功获取Redis客户端")

	// 执行Redis操作
	err = redisClient.WithPipeline(ctx, func(pipe redis.Pipeliner) error {
		// 执行Redis操作
		return nil
	})
	if err != nil {
		pr.Error("执行Redis操作失败: %v", err)
		return
	}

	// 获取Elasticsearch客户端
	esClient, err := manager.GetES("search")
	if err != nil {
		pr.Error("获取Elasticsearch客户端失败: %v", err)
		return
	}
	pr.Info("成功获取Elasticsearch客户端")

	// 检查索引是否存在
	exists, err := esClient.IndexExists(ctx, "products")
	if err != nil {
		pr.Error("检查索引失败: %v", err)
		return
	}
	if !exists {
		pr.Info("索引不存在，创建索引")
		err = esClient.CreateIndex(ctx, "products", `{
			"settings": {
				"number_of_shards": 1,
				"number_of_replicas": 0
			},
			"mappings": {
				"properties": {
					"name": { "type": "text" },
					"price": { "type": "float" },
					"created_at": { "type": "date" }
				}
			}
		}`)
		if err != nil {
			pr.Error("创建索引失败: %v", err)
			return
		}
	}

	// 获取健康状态摘要
	summary := manager.GetHealthSummary()
	pr.Info("存储服务健康状态摘要:")
	pr.Info("  总体状态: %s", summary.OverallStatus)
	pr.Info("  已检查服务: %d", summary.CheckedServices)
	pr.Info("  健康服务: %d", summary.HealthyServices)
	pr.Info("  降级服务: %d", summary.DegradedServices)
	pr.Info("  不健康服务: %d", summary.UnhealthyServices)

	// 获取活跃告警
	alerts := manager.GetActiveAlerts()
	if len(alerts) > 0 {
		pr.Warning("当前有 %d 个活跃告警:", len(alerts))
		for _, alert := range alerts {
			pr.Warning("  [%s] %s: %s", alert.Level, alert.Name, alert.Message)
		}
	} else {
		pr.Info("当前没有活跃告警")
	}

	// 执行健康检查
	healthStatus := manager.HealthCheck(ctx)
	for name, status := range healthStatus {
		pr.Info("服务 %s 状态: %s", name, status.State)
	}

	// 程序继续运行...
	select {}
}

// SimpleExample 简化的使用示例
func SimpleExample() {
	// 创建增强型存储管理器，使用默认设置
	manager := NewStorageManager()

	// 启动健康检查服务
	if err := manager.StartHealthCheck(); err != nil {
		log.Fatalf("启动健康检查服务失败: %v", err)
	}

	// 在程序退出时关闭所有连接
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		manager.CloseAll(ctx)
	}()

	// 程序主逻辑...
	ctx := context.Background()

	// 定期检查健康状态
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_ = manager.HealthCheck(ctx)
				summary := manager.GetHealthSummary()
				if summary.OverallStatus != health.StatusHealthy {
					pr.Warning("存储服务状态: %s", summary.OverallStatus)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// 应用主循环
	select {}
}
