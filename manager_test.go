package storage

import (
	"testing"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/sylphbyte/sylph"
	"gorm.io/gorm"
)

func TestStorageManagerWithMockAdapters(t *testing.T) {
	// 创建存储管理器
	manager := NewStorageManager()
	assert.NotNil(t, manager, "创建存储管理器失败")

	// 创建模拟的DB、Redis和ES客户端
	var db *gorm.DB = nil                    // 模拟
	var redisClient *redis.Client = nil      // 模拟
	var esClient *elasticsearch.Client = nil // 模拟

	// 创建适配器
	dbAdapter := NewMysqlStorage("test_db", db)
	redisAdapter := NewRedisStorage("test_redis", redisClient)
	esAdapter := NewESStorage("test_es", esClient)

	// 注册到管理器
	err := manager.RegisterDB("test_db", dbAdapter)
	assert.NoError(t, err, "注册DB适配器失败")

	err = manager.RegisterRedis("test_redis", redisAdapter)
	assert.NoError(t, err, "注册Redis适配器失败")

	err = manager.RegisterES("test_es", esAdapter)
	assert.NoError(t, err, "注册ES适配器失败")

	// 获取并验证
	dbStorage, err := manager.GetDB("test_db")
	assert.NoError(t, err, "获取DB适配器失败")
	assert.Equal(t, "test_db", dbStorage.GetName())

	redisStorage, err := manager.GetRedis("test_redis")
	assert.NoError(t, err, "获取Redis适配器失败")
	assert.Equal(t, "test_redis", redisStorage.GetName())

	esStorage, err := manager.GetES("test_es")
	assert.NoError(t, err, "获取ES适配器失败")
	assert.Equal(t, "test_es", esStorage.GetName())

	// 测试默认存储
	dbStorage, err = manager.GetDB()
	assert.NoError(t, err, "获取默认DB适配器失败")
	assert.Equal(t, "test_db", dbStorage.GetName())

	redisStorage, err = manager.GetRedis()
	assert.NoError(t, err, "获取默认Redis适配器失败")
	assert.Equal(t, "test_redis", redisStorage.GetName())

	esStorage, err = manager.GetES()
	assert.NoError(t, err, "获取默认ES适配器失败")
	assert.Equal(t, "test_es", esStorage.GetName())

	// 测试获取所有存储
	allStorages := manager.GetAllStorages()
	assert.Len(t, allStorages, 3, "获取所有存储数量错误")

	// 测试健康检查 (忽略错误)
	ctx := sylph.NewDefaultContext("test", "")
	healthStatuses := manager.HealthCheck(ctx)
	assert.Len(t, healthStatuses, 3, "健康检查结果数量错误")

	// 验证健康状态都是错误状态
	for _, status := range healthStatuses {
		assert.Equal(t, StorageStateError, status.State, "客户端为nil时状态应为错误")
		assert.NotEmpty(t, status.ErrorMsg, "错误消息不应为空")
	}

	// 测试关闭所有连接
	err = manager.CloseAll(ctx)
	assert.NoError(t, err, "关闭所有连接失败")
}

func TestStorageManagerWithAdapters(t *testing.T) {
	// 创建存储管理器
	manager := NewStorageManager()
	assert.NotNil(t, manager, "创建存储管理器失败")

	// 创建模拟的DB、Redis和ES客户端
	var db *gorm.DB = nil                    // 模拟
	var redisClient *redis.Client = nil      // 模拟
	var esClient *elasticsearch.Client = nil // 模拟

	// 创建适配器
	dbAdapter := NewMysqlStorage("test_db", db)
	redisAdapter := NewRedisStorage("test_redis", redisClient)
	esAdapter := NewESStorage("test_es", esClient)

	// 注册到管理器
	err := manager.RegisterDB("test_db", dbAdapter)
	assert.NoError(t, err, "注册DB适配器失败")

	err = manager.RegisterRedis("test_redis", redisAdapter)
	assert.NoError(t, err, "注册Redis适配器失败")

	err = manager.RegisterES("test_es", esAdapter)
	assert.NoError(t, err, "注册ES适配器失败")

	// 获取并验证
	dbStorage, err := manager.GetDB("test_db")
	assert.NoError(t, err, "获取DB适配器失败")
	assert.Equal(t, "test_db", dbStorage.GetName())

	redisStorage, err := manager.GetRedis("test_redis")
	assert.NoError(t, err, "获取Redis适配器失败")
	assert.Equal(t, "test_redis", redisStorage.GetName())

	esStorage, err := manager.GetES("test_es")
	assert.NoError(t, err, "获取ES适配器失败")
	assert.Equal(t, "test_es", esStorage.GetName())

	// 测试默认存储
	dbStorage, err = manager.GetDB()
	assert.NoError(t, err, "获取默认DB适配器失败")
	assert.Equal(t, "test_db", dbStorage.GetName())

	redisStorage, err = manager.GetRedis()
	assert.NoError(t, err, "获取默认Redis适配器失败")
	assert.Equal(t, "test_redis", redisStorage.GetName())

	esStorage, err = manager.GetES()
	assert.NoError(t, err, "获取默认ES适配器失败")
	assert.Equal(t, "test_es", esStorage.GetName())

	// 测试获取所有存储
	allStorages := manager.GetAllStorages()
	assert.Len(t, allStorages, 3, "获取所有存储数量错误")

	// 测试健康检查
	ctx := sylph.NewDefaultContext("test", "")
	healthStatuses := manager.HealthCheck(ctx)
	assert.Len(t, healthStatuses, 3, "健康检查结果数量错误")

	// 测试关闭所有连接
	err = manager.CloseAll(ctx)
	assert.NoError(t, err, "关闭所有连接失败")
}
