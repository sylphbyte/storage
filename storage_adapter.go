package storage

import (
	"strings"
	"sync"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
	"github.com/sylphbyte/sylph"
	"gorm.io/gorm"
)

// MysqlStorageAdapter 适配GORM连接到DBStorage接口
type MysqlStorageAdapter struct {
	name        string
	db          *gorm.DB
	connected   bool
	healthState *HealthStatus
	mutex       sync.RWMutex
}

// RedisStorageAdapter 适配Redis客户端到RedisStorage接口
type RedisStorageAdapter struct {
	name        string
	client      *redis.Client
	connected   bool
	healthState *HealthStatus
	mutex       sync.RWMutex
}

// ESStorageAdapter 适配Elasticsearch客户端到ESStorage接口
type ESStorageAdapter struct {
	name        string
	client      *elasticsearch.Client
	connected   bool
	healthState *HealthStatus
	mutex       sync.RWMutex
}

// NewMysqlStorage 创建MySQL存储适配器
func NewMysqlStorage(name string, db *gorm.DB) *MysqlStorageAdapter {
	now := time.Now()
	return &MysqlStorageAdapter{
		name:      name,
		db:        db,
		connected: true,
		healthState: &HealthStatus{
			State:       StorageStateConnected,
			ConnectedAt: now,
			LastPingAt:  now,
		},
	}
}

// NewRedisStorage 创建Redis存储适配器
func NewRedisStorage(name string, client *redis.Client) *RedisStorageAdapter {
	now := time.Now()
	return &RedisStorageAdapter{
		name:      name,
		client:    client,
		connected: true,
		healthState: &HealthStatus{
			State:       StorageStateConnected,
			ConnectedAt: now,
			LastPingAt:  now,
		},
	}
}

// NewESStorage 创建Elasticsearch存储适配器
func NewESStorage(name string, client *elasticsearch.Client) *ESStorageAdapter {
	now := time.Now()
	return &ESStorageAdapter{
		name:      name,
		client:    client,
		connected: true,
		healthState: &HealthStatus{
			State:       StorageStateConnected,
			ConnectedAt: now,
			LastPingAt:  now,
		},
	}
}

// ---- MysqlStorageAdapter 实现 DBStorage 接口 ----

func (a *MysqlStorageAdapter) GetType() StorageType {
	return StorageTypeMySQL
}

func (a *MysqlStorageAdapter) GetName() string {
	return a.name
}

func (a *MysqlStorageAdapter) IsConnected() bool {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	return a.connected
}

func (a *MysqlStorageAdapter) Connect(ctx sylph.Context) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.connected {
		return nil
	}

	// GORM不需要显式连接
	// 这里只需要更新状态
	a.connected = true
	a.healthState.State = StorageStateConnected
	a.healthState.ConnectedAt = time.Now()
	a.healthState.ErrorMsg = ""

	return nil
}

func (a *MysqlStorageAdapter) Disconnect(ctx sylph.Context) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if !a.connected {
		return nil
	}

	// 测试用例可能使用nil的db
	if a.db == nil {
		a.connected = false
		a.healthState.State = StorageStateDisconnected
		return nil
	}

	// 获取底层SQL DB连接并关闭
	sqlDB, err := a.db.DB()
	if err != nil {
		return errors.Wrap(err, "获取底层SQL DB失败")
	}

	if err := sqlDB.Close(); err != nil {
		return errors.Wrap(err, "关闭MySQL连接失败")
	}

	a.connected = false
	a.healthState.State = StorageStateDisconnected
	return nil
}

func (a *MysqlStorageAdapter) Reconnect(ctx sylph.Context) error {
	if err := a.Disconnect(ctx); err != nil {
		return errors.Wrap(err, "断开连接失败")
	}
	return a.Connect(ctx)
}

func (a *MysqlStorageAdapter) Ping(ctx sylph.Context) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if !a.connected {
		return errors.New("未连接到MySQL")
	}

	// 测试用例可能使用nil的db
	if a.db == nil {
		a.healthState.State = StorageStateError
		a.healthState.ErrorMsg = "数据库连接为空"
		a.healthState.FailCount++
		return errors.New("数据库连接为空")
	}

	// 获取底层SQL DB连接并Ping
	sqlDB, err := a.db.DB()
	if err != nil {
		a.healthState.State = StorageStateError
		a.healthState.ErrorMsg = err.Error()
		a.healthState.FailCount++
		return errors.Wrap(err, "获取底层SQL DB失败")
	}

	start := time.Now()
	if err := sqlDB.Ping(); err != nil {
		a.healthState.State = StorageStateError
		a.healthState.ErrorMsg = err.Error()
		a.healthState.FailCount++
		return errors.Wrap(err, "Ping MySQL失败")
	}

	a.healthState.Latency = time.Since(start).Milliseconds()
	a.healthState.LastPingAt = time.Now()
	a.healthState.State = StorageStateConnected
	a.healthState.FailCount = 0
	a.healthState.ErrorMsg = ""

	return nil
}

func (a *MysqlStorageAdapter) GetHealthStatus() *HealthStatus {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	return a.healthState
}

func (a *MysqlStorageAdapter) GetDB() *gorm.DB {
	return a.db
}

func (a *MysqlStorageAdapter) WithTransaction(ctx sylph.Context, fn func(tx *gorm.DB) error) error {
	if a.db == nil {
		return errors.New("数据库连接为空")
	}
	return a.db.Transaction(fn)
}

// ---- RedisStorageAdapter 实现 RedisStorage 接口 ----

func (a *RedisStorageAdapter) GetType() StorageType {
	return StorageTypeRedis
}

func (a *RedisStorageAdapter) GetName() string {
	return a.name
}

func (a *RedisStorageAdapter) IsConnected() bool {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	return a.connected
}

func (a *RedisStorageAdapter) Connect(ctx sylph.Context) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.connected {
		return nil
	}

	// Redis客户端不需要显式连接
	// 通过Ping测试连接
	if a.client == nil {
		return errors.New("Redis客户端为空")
	}

	_, err := a.client.Ping(ctx).Result()
	if err != nil {
		return errors.Wrap(err, "连接Redis失败")
	}

	a.connected = true
	a.healthState.State = StorageStateConnected
	a.healthState.ConnectedAt = time.Now()
	a.healthState.ErrorMsg = ""

	return nil
}

func (a *RedisStorageAdapter) Disconnect(ctx sylph.Context) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if !a.connected {
		return nil
	}

	// 测试用例可能使用nil的client
	if a.client == nil {
		a.connected = false
		a.healthState.State = StorageStateDisconnected
		return nil
	}

	if err := a.client.Close(); err != nil {
		return errors.Wrap(err, "关闭Redis连接失败")
	}

	a.connected = false
	a.healthState.State = StorageStateDisconnected
	return nil
}

func (a *RedisStorageAdapter) Reconnect(ctx sylph.Context) error {
	if err := a.Disconnect(ctx); err != nil {
		return errors.Wrap(err, "断开连接失败")
	}
	return a.Connect(ctx)
}

func (a *RedisStorageAdapter) Ping(ctx sylph.Context) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if !a.connected {
		return errors.New("未连接到Redis")
	}

	// 测试用例可能使用nil的client
	if a.client == nil {
		a.healthState.State = StorageStateError
		a.healthState.ErrorMsg = "Redis客户端为空"
		a.healthState.FailCount++
		return errors.New("Redis客户端为空")
	}

	start := time.Now()
	_, err := a.client.Ping(ctx).Result()
	if err != nil {
		a.healthState.State = StorageStateError
		a.healthState.ErrorMsg = err.Error()
		a.healthState.FailCount++
		return errors.Wrap(err, "Ping Redis失败")
	}

	a.healthState.Latency = time.Since(start).Milliseconds()
	a.healthState.LastPingAt = time.Now()
	a.healthState.State = StorageStateConnected
	a.healthState.FailCount = 0
	a.healthState.ErrorMsg = ""

	return nil
}

func (a *RedisStorageAdapter) GetHealthStatus() *HealthStatus {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	return a.healthState
}

func (a *RedisStorageAdapter) GetClient() *redis.Client {
	return a.client
}

func (a *RedisStorageAdapter) WithPipeline(ctx sylph.Context, fn func(pipe redis.Pipeliner) error) error {
	if a.client == nil {
		return errors.New("Redis客户端为空")
	}
	pipe := a.client.Pipeline()
	err := fn(pipe)
	if err != nil {
		return err
	}
	_, err = pipe.Exec(ctx)
	return err
}

// ---- ESStorageAdapter 实现 ESStorage 接口 ----

func (a *ESStorageAdapter) GetType() StorageType {
	return StorageTypeES
}

func (a *ESStorageAdapter) GetName() string {
	return a.name
}

func (a *ESStorageAdapter) IsConnected() bool {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	return a.connected
}

func (a *ESStorageAdapter) Connect(ctx sylph.Context) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.connected {
		return nil
	}

	// ES客户端不需要显式连接
	// 通过Info测试连接
	if a.client == nil {
		return errors.New("Elasticsearch客户端为空")
	}

	info, err := a.client.Info()
	if err != nil {
		return errors.Wrap(err, "连接Elasticsearch失败")
	}
	defer info.Body.Close()

	a.connected = true
	a.healthState.State = StorageStateConnected
	a.healthState.ConnectedAt = time.Now()
	a.healthState.ErrorMsg = ""

	return nil
}

func (a *ESStorageAdapter) Disconnect(ctx sylph.Context) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// ES客户端不需要显式关闭
	// 更新状态
	a.connected = false
	a.healthState.State = StorageStateDisconnected
	return nil
}

func (a *ESStorageAdapter) Reconnect(ctx sylph.Context) error {
	return a.Connect(ctx)
}

func (a *ESStorageAdapter) Ping(ctx sylph.Context) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if !a.connected {
		return errors.New("未连接到Elasticsearch")
	}

	// 测试用例可能使用nil的client
	if a.client == nil {
		a.healthState.State = StorageStateError
		a.healthState.ErrorMsg = "Elasticsearch客户端为空"
		a.healthState.FailCount++
		return errors.New("Elasticsearch客户端为空")
	}

	start := time.Now()
	info, err := a.client.Info()
	if err != nil {
		a.healthState.State = StorageStateError
		a.healthState.ErrorMsg = err.Error()
		a.healthState.FailCount++
		return errors.Wrap(err, "Ping Elasticsearch失败")
	}
	defer info.Body.Close()

	a.healthState.Latency = time.Since(start).Milliseconds()
	a.healthState.LastPingAt = time.Now()
	a.healthState.State = StorageStateConnected
	a.healthState.FailCount = 0
	a.healthState.ErrorMsg = ""

	return nil
}

func (a *ESStorageAdapter) GetHealthStatus() *HealthStatus {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	return a.healthState
}

func (a *ESStorageAdapter) GetClient() *elasticsearch.Client {
	return a.client
}

func (a *ESStorageAdapter) IndexExists(ctx sylph.Context, index string) (bool, error) {
	if a.client == nil {
		return false, errors.New("Elasticsearch客户端为空")
	}

	resp, err := a.client.Indices.Exists([]string{index})
	if err != nil {
		return false, errors.Wrapf(err, "检查索引 %s 存在性失败", index)
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200, nil
}

func (a *ESStorageAdapter) CreateIndex(ctx sylph.Context, index string, mapping string) error {
	if a.client == nil {
		return errors.New("Elasticsearch客户端为空")
	}

	exists, err := a.IndexExists(ctx, index)
	if err != nil {
		return err
	}

	if exists {
		pr.Warning("索引 %s 已存在，跳过创建", index)
		return nil
	}

	resp, err := a.client.Indices.Create(
		index,
		a.client.Indices.Create.WithBody(strings.NewReader(mapping)),
	)
	if err != nil {
		return errors.Wrapf(err, "创建索引 %s 失败", index)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return errors.Errorf("创建索引 %s 失败: %s", index, resp.String())
	}

	pr.System("成功创建索引: %s", index)
	return nil
}
