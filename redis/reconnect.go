package redis

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
	"app/pkg/storage/reconnect"
)

// ReconnectableClient Redis可自动重连的客户端
type ReconnectableClient struct {
	*Client
	reconnector *reconnect.BaseReconnector
}

// NewReconnectableClient 创建支持自动重连的Redis客户端
func NewReconnectableClient(config *Config) (*ReconnectableClient, error) {
	// 首先创建标准客户端
	client, err := NewClient(config)
	if err != nil {
		return nil, errors.Wrap(err, "创建Redis客户端失败")
	}

	// 创建重连策略
	policy := reconnect.DefaultReconnectPolicy()
	if config.ReconnectPolicy != nil {
		policy = *config.ReconnectPolicy
	}

	// 创建重连器
	rc := &ReconnectableClient{
		Client: client,
	}

	reconnector, err := reconnect.NewBaseReconnector(
		rc.connect,
		rc.isConnected,
		rc.close,
		&policy,
	)

	if err != nil {
		client.Close()
		return nil, errors.Wrap(err, "创建重连器失败")
	}

	rc.reconnector = reconnector
	return rc, nil
}

// connect 建立连接
func (c *ReconnectableClient) connect(ctx context.Context) error {
	// 关闭现有客户端
	if c.Client.redisClient != nil {
		_ = c.Client.redisClient.Close()
	}

	// 创建新的Redis客户端
	var client redis.UniversalClient
	if c.Client.config.ClusterMode {
		// 集群模式
		client = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:        c.Client.config.Addrs,
			Password:     c.Client.config.Password,
			DialTimeout:  time.Duration(c.Client.config.DialTimeout) * time.Second,
			ReadTimeout:  time.Duration(c.Client.config.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(c.Client.config.WriteTimeout) * time.Second,
			PoolSize:     c.Client.config.PoolSize,
			MinIdleConns: c.Client.config.MinIdleConns,
			MaxConnAge:   time.Duration(c.Client.config.MaxConnAge) * time.Second,
		})
	} else {
		// 单机模式
		client = redis.NewClient(&redis.Options{
			Addr:         c.Client.config.Addrs[0],
			Password:     c.Client.config.Password,
			DB:           c.Client.config.DB,
			DialTimeout:  time.Duration(c.Client.config.DialTimeout) * time.Second,
			ReadTimeout:  time.Duration(c.Client.config.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(c.Client.config.WriteTimeout) * time.Second,
			PoolSize:     c.Client.config.PoolSize,
			MinIdleConns: c.Client.config.MinIdleConns,
			MaxConnAge:   time.Duration(c.Client.config.MaxConnAge) * time.Second,
		})
	}

	// 测试连接
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return errors.Wrap(err, "Redis ping失败")
	}

	c.Client.redisClient = client
	pr.Info("Redis连接建立成功")
	return nil
}

// isConnected 检查连接状态
func (c *ReconnectableClient) isConnected() bool {
	if c.Client.redisClient == nil {
		return false
	}

	// 使用Ping来检查连接状态，使用默认上下文
	return c.Client.redisClient.Ping(context.Background()).Err() == nil
}

// close 关闭Redis连接
func (c *ReconnectableClient) close() error {
	if c.Client.redisClient != nil {
		err := c.Client.redisClient.Close()
		c.Client.redisClient = nil
		return err
	}
	return nil
}

// Reconnect 重新连接Redis
func (c *ReconnectableClient) Reconnect(ctx context.Context) error {
	return c.reconnector.Reconnect(ctx)
}

// IsConnected 检查是否已连接
func (c *ReconnectableClient) IsConnected() bool {
	return c.reconnector.IsConnected()
}

// GetReconnectStats 获取重连统计信息
func (c *ReconnectableClient) GetReconnectStats() reconnect.ReconnectStats {
	return c.reconnector.GetReconnectStats()
}

// SetReconnectPolicy 设置重连策略
func (c *ReconnectableClient) SetReconnectPolicy(policy reconnect.ReconnectPolicy) error {
	return c.reconnector.SetReconnectPolicy(policy)
}

// DoWithReconnect 执行Redis命令，自动处理重连
func (c *ReconnectableClient) DoWithReconnect(ctx context.Context, cmd redis.Cmder) error {
	return reconnect.WithReconnect(ctx, c.reconnector, "redis", func() error {
		return c.Client.Do(ctx, cmd)
	})
}

// GetWithReconnect 获取键值，自动处理重连
func (c *ReconnectableClient) GetWithReconnect(ctx context.Context, key string) (string, error) {
	var val string
	err := reconnect.WithReconnect(ctx, c.reconnector, "redis", func() error {
		var err error
		val, err = c.Client.redisClient.Get(ctx, key).Result()
		return err
	})
	return val, err
}

// SetWithReconnect 设置键值，自动处理重连
func (c *ReconnectableClient) SetWithReconnect(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return reconnect.WithReconnect(ctx, c.reconnector, "redis", func() error {
		return c.Client.redisClient.Set(ctx, key, value, expiration).Err()
	})
}

// DelWithReconnect 删除键，自动处理重连
func (c *ReconnectableClient) DelWithReconnect(ctx context.Context, keys ...string) (int64, error) {
	var val int64
	err := reconnect.WithReconnect(ctx, c.reconnector, "redis", func() error {
		var err error
		val, err = c.Client.redisClient.Del(ctx, keys...).Result()
		return err
	})
	return val, err
}

// ExistsWithReconnect 检查键是否存在，自动处理重连
func (c *ReconnectableClient) ExistsWithReconnect(ctx context.Context, keys ...string) (int64, error) {
	var val int64
	err := reconnect.WithReconnect(ctx, c.reconnector, "redis", func() error {
		var err error
		val, err = c.Client.redisClient.Exists(ctx, keys...).Result()
		return err
	})
	return val, err
}

// PingWithReconnect Ping Redis服务器，自动处理重连
func (c *ReconnectableClient) PingWithReconnect(ctx context.Context) error {
	return reconnect.WithReconnect(ctx, c.reconnector, "redis", func() error {
		return c.Client.redisClient.Ping(ctx).Err()
	})
}
