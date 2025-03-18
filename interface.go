// Package storage 提供统一的存储服务接口和实现
package storage

import (
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/go-redis/redis/v8"
	"github.com/sylphbyte/sylph"
	"gorm.io/gorm"
)

// StorageType 存储类型枚举
type StorageType string

const (
	// StorageTypeMySQL MySQL存储类型
	StorageTypeMySQL StorageType = "mysql"
	// StorageTypeRedis Redis存储类型
	StorageTypeRedis StorageType = "redis"
	// StorageTypeES Elasticsearch存储类型
	StorageTypeES StorageType = "elasticsearch"
)

// StorageState 存储状态枚举
type StorageState string

const (
	// StorageStateConnected 已连接状态
	StorageStateConnected StorageState = "connected"
	// StorageStateDisconnected 已断开连接状态
	StorageStateDisconnected StorageState = "disconnected"
	// StorageStateReconnecting 重连中状态
	StorageStateReconnecting StorageState = "reconnecting"
	// StorageStateError 错误状态
	StorageStateError StorageState = "error"
)

// HealthStatus 健康状态
type HealthStatus struct {
	State       StorageState `json:"state"`        // 当前状态
	ConnectedAt time.Time    `json:"connected_at"` // 连接时间
	LastPingAt  time.Time    `json:"last_ping_at"` // 最后一次成功PING的时间
	FailCount   int          `json:"fail_count"`   // 连续失败次数
	ErrorMsg    string       `json:"error_msg"`    // 错误信息
	Latency     int64        `json:"latency"`      // 延迟（毫秒）
}

// StorageConfig 存储配置接口
type StorageConfig interface {
	// GetType 获取存储类型
	GetType() StorageType

	// GetName 获取存储名称
	GetName() string

	// Validate 验证配置是否有效
	Validate() error
}

// Storage 存储接口，所有类型存储的基础接口
type Storage interface {
	// 基础信息
	GetType() StorageType
	GetName() string

	// 状态管理
	IsConnected() bool
	Connect(ctx sylph.Context) error
	Disconnect(ctx sylph.Context) error
	Reconnect(ctx sylph.Context) error

	// 健康检查
	Ping(ctx sylph.Context) error
	GetHealthStatus() *HealthStatus
}

// DBStorage 数据库存储接口
type DBStorage interface {
	Storage
	GetDB() *gorm.DB
	WithTransaction(ctx sylph.Context, fn func(tx *gorm.DB) error) error
}

// RedisStorage Redis存储接口
type RedisStorage interface {
	Storage
	GetClient() *redis.Client
	WithPipeline(ctx sylph.Context, fn func(pipe redis.Pipeliner) error) error
}

// ESStorage Elasticsearch存储接口
type ESStorage interface {
	Storage
	GetClient() *elasticsearch.Client
	IndexExists(ctx sylph.Context, index string) (bool, error)
	CreateIndex(ctx sylph.Context, index string, mapping string) error
}

// StorageManager 存储管理器接口
type StorageManager interface {
	// 数据库相关
	GetDB(name ...string) (DBStorage, error)
	RegisterDB(name string, storage DBStorage) error

	// Redis相关
	GetRedis(name ...string) (RedisStorage, error)
	RegisterRedis(name string, storage RedisStorage) error

	// ES相关
	GetES(name ...string) (ESStorage, error)
	RegisterES(name string, storage ESStorage) error

	// 全局操作
	GetAllStorages() map[string]Storage
	HealthCheck(ctx sylph.Context) map[string]*HealthStatus
	CloseAll(ctx sylph.Context) error
}
