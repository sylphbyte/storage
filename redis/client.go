// Package redis 提供Redis操作的相关功能
package redis

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
	"app/pkg/storage/reconnect"
)

// Config Redis客户端配置
type Config struct {
	// Addrs Redis地址列表，单机模式只用第一个
	Addrs []string `yaml:"addrs" json:"addrs"`

	// Password Redis密码
	Password string `yaml:"password" json:"password"`

	// DB 数据库编号（仅单机模式有效）
	DB int `yaml:"db" json:"db"`

	// ClusterMode 是否集群模式
	ClusterMode bool `yaml:"cluster_mode" json:"cluster_mode"`

	// DialTimeout 连接超时(秒)
	DialTimeout int `yaml:"dial_timeout" json:"dial_timeout"`

	// ReadTimeout 读取超时(秒)
	ReadTimeout int `yaml:"read_timeout" json:"read_timeout"`

	// WriteTimeout 写入超时(秒)
	WriteTimeout int `yaml:"write_timeout" json:"write_timeout"`

	// PoolSize 连接池大小
	PoolSize int `yaml:"pool_size" json:"pool_size"`

	// MinIdleConns 最小空闲连接数
	MinIdleConns int `yaml:"min_idle_conns" json:"min_idle_conns"`

	// MaxConnAge 连接最大生存时间(秒)
	MaxConnAge int `yaml:"max_conn_age" json:"max_conn_age"`

	// ReconnectPolicy 重连策略
	ReconnectPolicy *reconnect.ReconnectPolicy `yaml:"reconnect_policy" json:"reconnect_policy"`
}

// Client Redis客户端
type Client struct {
	redisClient redis.UniversalClient
	config      *Config
}

// NewClient 创建新的Redis客户端
func NewClient(config *Config) (*Client, error) {
	if config == nil {
		return nil, errors.New("Redis配置不能为空")
	}

	if len(config.Addrs) == 0 {
		return nil, errors.New("Redis地址不能为空")
	}

	var client redis.UniversalClient
	if config.ClusterMode {
		// 集群模式
		client = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:        config.Addrs,
			Password:     config.Password,
			DialTimeout:  time.Duration(config.DialTimeout) * time.Second,
			ReadTimeout:  time.Duration(config.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(config.WriteTimeout) * time.Second,
			PoolSize:     config.PoolSize,
			MinIdleConns: config.MinIdleConns,
			MaxConnAge:   time.Duration(config.MaxConnAge) * time.Second,
		})
	} else {
		// 单机模式
		client = redis.NewClient(&redis.Options{
			Addr:         config.Addrs[0],
			Password:     config.Password,
			DB:           config.DB,
			DialTimeout:  time.Duration(config.DialTimeout) * time.Second,
			ReadTimeout:  time.Duration(config.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(config.WriteTimeout) * time.Second,
			PoolSize:     config.PoolSize,
			MinIdleConns: config.MinIdleConns,
			MaxConnAge:   time.Duration(config.MaxConnAge) * time.Second,
		})
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, errors.Wrap(err, "Redis ping失败")
	}

	pr.Info("Redis连接成功: %s", config.Addrs)

	return &Client{
		redisClient: client,
		config:      config,
	}, nil
}

// Client 返回原生Redis客户端
func (c *Client) Client() redis.UniversalClient {
	return c.redisClient
}

// Close 关闭Redis连接
func (c *Client) Close() error {
	if c.redisClient != nil {
		err := c.redisClient.Close()
		if err != nil {
			return errors.Wrap(err, "关闭Redis连接失败")
		}
		c.redisClient = nil
		pr.Info("Redis连接已关闭")
	}
	return nil
}

// Ping 测试Redis连接
func (c *Client) Ping(ctx context.Context) error {
	if c.redisClient == nil {
		return errors.New("Redis连接未初始化")
	}

	err := c.redisClient.Ping(ctx).Err()
	if err != nil {
		return errors.Wrap(err, "Redis ping失败")
	}

	return nil
}

// Get 获取键值
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	if c.redisClient == nil {
		return "", errors.New("Redis连接未初始化")
	}

	val, err := c.redisClient.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return "", errors.Wrapf(err, "Redis获取失败: %s", key)
	}

	return val, err
}

// Set 设置键值
func (c *Client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	if c.redisClient == nil {
		return errors.New("Redis连接未初始化")
	}

	err := c.redisClient.Set(ctx, key, value, expiration).Err()
	if err != nil {
		return errors.Wrapf(err, "Redis设置失败: %s", key)
	}

	return nil
}

// Del 删除键
func (c *Client) Del(ctx context.Context, keys ...string) (int64, error) {
	if c.redisClient == nil {
		return 0, errors.New("Redis连接未初始化")
	}

	n, err := c.redisClient.Del(ctx, keys...).Result()
	if err != nil {
		return 0, errors.Wrap(err, "Redis删除失败")
	}

	return n, nil
}

// Exists 检查键是否存在
func (c *Client) Exists(ctx context.Context, keys ...string) (int64, error) {
	if c.redisClient == nil {
		return 0, errors.New("Redis连接未初始化")
	}

	n, err := c.redisClient.Exists(ctx, keys...).Result()
	if err != nil {
		return 0, errors.Wrap(err, "Redis检查失败")
	}

	return n, nil
}

// Do 执行任意Redis命令
func (c *Client) Do(ctx context.Context, cmd redis.Cmder) error {
	if c.redisClient == nil {
		return errors.New("Redis连接未初始化")
	}

	err := c.redisClient.Do(ctx, cmd).Err()
	if err != nil && err != redis.Nil {
		return errors.Wrap(err, "Redis执行命令失败")
	}

	return err
}

// Pipeline 创建一个Pipeline
func (c *Client) Pipeline(ctx context.Context) redis.Pipeliner {
	if c.redisClient == nil {
		return nil
	}

	return c.redisClient.Pipeline()
}

// TxPipeline 创建一个事务Pipeline
func (c *Client) TxPipeline(ctx context.Context) redis.Pipeliner {
	if c.redisClient == nil {
		return nil
	}

	return c.redisClient.TxPipeline()
}
