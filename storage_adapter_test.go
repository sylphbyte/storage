package storage

import (
	"os"
	"testing"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/sylphbyte/sylph"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestMysqlStorageAdapter(t *testing.T) {
	// 如果没有配置测试数据库，则跳过测试
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("跳过MySQL适配器测试: 未设置TEST_MYSQL_DSN环境变量")
	}

	// 创建GORM连接
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	assert.NoError(t, err, "打开MySQL连接失败")

	// 创建适配器
	adapter := NewMysqlStorage("test_mysql", db)
	assert.NotNil(t, adapter, "创建MySQL适配器失败")

	// 测试接口实现
	var storage DBStorage = adapter
	assert.Equal(t, StorageTypeMySQL, storage.GetType())
	assert.Equal(t, "test_mysql", storage.GetName())
	assert.True(t, storage.IsConnected())

	// 测试健康状态
	status := storage.GetHealthStatus()
	assert.NotNil(t, status)
	assert.Equal(t, StorageStateConnected, status.State)

	// 创建上下文
	ctx := sylph.NewDefaultContext("test", "")

	// 测试Ping
	err = storage.Ping(ctx)
	assert.NoError(t, err, "Ping失败")

	// 测试获取DB
	assert.Equal(t, db, storage.GetDB())
}

func TestRedisStorageAdapter(t *testing.T) {
	// 如果没有配置测试Redis，则跳过测试
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("跳过Redis适配器测试: 未设置TEST_REDIS_ADDR环境变量")
	}

	// 创建Redis客户端
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	// 创建适配器
	adapter := NewRedisStorage("test_redis", client)
	assert.NotNil(t, adapter, "创建Redis适配器失败")

	// 测试接口实现
	var storage RedisStorage = adapter
	assert.Equal(t, StorageTypeRedis, storage.GetType())
	assert.Equal(t, "test_redis", storage.GetName())
	assert.True(t, storage.IsConnected())

	// 测试健康状态
	status := storage.GetHealthStatus()
	assert.NotNil(t, status)
	assert.Equal(t, StorageStateConnected, status.State)

	// 测试获取客户端
	assert.Equal(t, client, storage.GetClient())

	// 清理
	defer client.Close()
}

func TestESStorageAdapter(t *testing.T) {
	// 如果没有配置测试ES，则跳过测试
	addr := os.Getenv("TEST_ES_ADDR")
	if addr == "" {
		t.Skip("跳过ES适配器测试: 未设置TEST_ES_ADDR环境变量")
	}

	// 创建ES客户端
	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{addr},
	})
	assert.NoError(t, err, "创建ES客户端失败")

	// 创建适配器
	adapter := NewESStorage("test_es", client)
	assert.NotNil(t, adapter, "创建ES适配器失败")

	// 测试接口实现
	var storage ESStorage = adapter
	assert.Equal(t, StorageTypeES, storage.GetType())
	assert.Equal(t, "test_es", storage.GetName())
	assert.True(t, storage.IsConnected())

	// 测试健康状态
	status := storage.GetHealthStatus()
	assert.NotNil(t, status)
	assert.Equal(t, StorageStateConnected, status.State)

	// 测试获取客户端
	assert.Equal(t, client, storage.GetClient())
}
