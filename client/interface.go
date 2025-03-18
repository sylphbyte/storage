// Package client defines interfaces for storage clients.
package client

import (
	"time"

	"github.com/sylphbyte/sylph"
)

// StorageType 存储类型枚举
type StorageType string

const (
	// TypeMySQL MySQL存储类型
	TypeMySQL StorageType = "mysql"

	// TypeRedis Redis存储类型
	TypeRedis StorageType = "redis"

	// TypeElasticsearch Elasticsearch存储类型
	TypeElasticsearch StorageType = "elasticsearch"
)

// State 连接状态枚举
type State string

const (
	// StateConnected 已连接状态
	StateConnected State = "connected"

	// StateDisconnected 未连接状态
	StateDisconnected State = "disconnected"

	// StateReconnecting 重连中状态
	StateReconnecting State = "reconnecting"

	// StateError 错误状态
	StateError State = "error"
)

// HealthStatus 健康状态
type HealthStatus struct {
	// State 当前状态
	State State

	// ConnectedAt 连接时间
	ConnectedAt time.Time

	// LastPingAt 上次Ping时间
	LastPingAt time.Time

	// FailCount 连续失败次数
	FailCount int

	// ErrorMessage 上次错误信息
	ErrorMessage string

	// Latency 连接延迟
	Latency time.Duration
}

// Client 存储客户端接口
type Client interface {
	// Type 获取存储类型
	Type() StorageType

	// Name 获取客户端名称
	Name() string

	// Connected 检查是否已连接
	Connected() bool

	// Connect 连接到存储
	Connect(ctx sylph.Context) error

	// Disconnect 断开连接
	Disconnect(ctx sylph.Context) error

	// Reconnect 重新连接
	Reconnect(ctx sylph.Context) error

	// Ping 测试连接
	Ping(ctx sylph.Context) error

	// HealthStatus 获取健康状态
	HealthStatus() HealthStatus
}

// Config 客户端配置接口
type Config interface {
	// Type 获取存储类型
	Type() StorageType

	// Name 获取客户端名称
	Name() string

	// Validate 验证配置
	Validate() error
}

// Builder 客户端构建器接口
type Builder interface {
	// Build 构建客户端
	Build(config Config, options ...Option) (Client, error)
}
