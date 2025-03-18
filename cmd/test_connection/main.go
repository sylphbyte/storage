package main

import (
	"fmt"
	"os"

	"github.com/sylphbyte/pr"
	"github.com/sylphbyte/storage"
	"github.com/sylphbyte/sylph"
)

func main() {
	configPath := "/Users/lifeng/CodeV2/sylph_app/etc/debug/storage.yaml"
	pr.System("加载配置文件: %s", configPath)

	// 检查文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		pr.Error("配置文件不存在: %s", configPath)
		os.Exit(1)
	}

	// 启用所有存储
	enabledMysql := map[string]bool{"main": true}
	enabledRedis := map[string]bool{"main": true}
	enabledES := map[string]bool{"main": true}

	// 初始化存储管理器
	storageManager, err := storage.InitializeStorage(configPath, enabledMysql, enabledRedis, enabledES)
	if err != nil {
		pr.Error("初始化存储管理器失败: %v", err)
		os.Exit(1)
	}
	pr.System("存储管理器初始化成功")

	// 创建上下文
	ctx := sylph.NewDefaultContext("test", "")

	// 健康检查
	pr.System("执行健康检查...")
	healthStatuses := storageManager.HealthCheck(ctx)

	// 打印健康状态
	fmt.Println("\n--- 健康检查结果 ---")
	for name, status := range healthStatuses {
		fmt.Printf("存储 %s:\n", name)
		fmt.Printf("  状态: %s\n", status.State)
		fmt.Printf("  延迟: %dms\n", status.Latency)
		fmt.Printf("  连接时间: %v\n", status.ConnectedAt)
		fmt.Printf("  最后Ping: %v\n", status.LastPingAt)
		fmt.Printf("  失败次数: %d\n", status.FailCount)
		if status.ErrorMsg != "" {
			fmt.Printf("  错误: %s\n", status.ErrorMsg)
		}
		fmt.Println()
	}

	// 尝试获取默认MySQL连接
	pr.System("尝试获取MySQL连接...")
	dbStorage, err := storageManager.GetDB()
	if err != nil {
		pr.Error("获取MySQL连接失败: %v", err)
	} else {
		pr.System("获取MySQL连接成功: %s", dbStorage.GetName())
		db := dbStorage.GetDB()
		if db != nil {
			// 执行简单查询
			var result int
			row := db.Raw("SELECT 1").Row()
			if err := row.Scan(&result); err != nil {
				pr.Error("MySQL查询失败: %v", err)
			} else {
				pr.System("MySQL测试查询成功: %d", result)
			}
		}
	}

	// 尝试获取Redis连接
	pr.System("尝试获取Redis连接...")
	redisStorage, err := storageManager.GetRedis()
	if err != nil {
		pr.Error("获取Redis连接失败: %v", err)
	} else {
		pr.System("获取Redis连接成功: %s", redisStorage.GetName())
		redis := redisStorage.GetClient()
		if redis != nil {
			// 执行简单操作 - Ping
			if pong, err := redis.Ping(ctx).Result(); err != nil {
				pr.Error("Redis Ping失败: %v", err)
			} else {
				pr.System("Redis Ping成功: %s", pong)
			}
		}
	}

	// 尝试获取ES连接
	pr.System("尝试获取ES连接...")
	esStorage, err := storageManager.GetES()
	if err != nil {
		pr.Error("获取ES连接失败: %v", err)
	} else {
		pr.System("获取ES连接成功: %s", esStorage.GetName())
		es := esStorage.GetClient()
		if es != nil {
			// 获取ES信息
			info, err := es.Info()
			if err != nil {
				pr.Error("ES Info失败: %v", err)
			} else {
				defer info.Body.Close()
				pr.System("ES Info成功: 状态码 %d", info.StatusCode)
			}
		}
	}

	// 关闭所有连接
	pr.System("关闭所有连接...")
	if err := storageManager.CloseAll(ctx); err != nil {
		pr.Error("关闭连接失败: %v", err)
	} else {
		pr.System("所有连接已关闭")
	}
}
