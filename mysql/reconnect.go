package mysql

import (
	"context"
	"database/sql"
	"time"

	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
	"app/pkg/storage/reconnect"
)

// ReconnectableClient MySQL可自动重连的客户端
type ReconnectableClient struct {
	*Client
	reconnector *reconnect.BaseReconnector
}

// NewReconnectableClient 创建支持自动重连的MySQL客户端
func NewReconnectableClient(config *Config) (*ReconnectableClient, error) {
	// 首先创建标准客户端
	client, err := NewClient(config)
	if err != nil {
		return nil, errors.Wrap(err, "创建MySQL客户端失败")
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
	// 如果已经有连接，先关闭
	if c.Client.db != nil {
		_ = c.Client.db.Close()
	}

	db, err := sql.Open("mysql", c.Client.dsn)
	if err != nil {
		return errors.Wrap(err, "无法打开MySQL连接")
	}

	// 设置连接池配置
	db.SetConnMaxLifetime(time.Duration(c.Client.config.ConnMaxLifetime) * time.Second)
	db.SetMaxOpenConns(c.Client.config.MaxOpenConns)
	db.SetMaxIdleConns(c.Client.config.MaxIdleConns)

	// 测试连接
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return errors.Wrap(err, "MySQL ping失败")
	}

	c.Client.db = db
	pr.Info("MySQL连接建立成功")
	return nil
}

// isConnected 检查连接状态
func (c *ReconnectableClient) isConnected() bool {
	if c.Client.db == nil {
		return false
	}
	// 使用Ping来检查连接状态
	return c.Client.db.Ping() == nil
}

// close 关闭数据库连接
func (c *ReconnectableClient) close() error {
	if c.Client.db != nil {
		err := c.Client.db.Close()
		c.Client.db = nil
		return err
	}
	return nil
}

// Reconnect 重新连接数据库
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

// ExecWithReconnect 执行SQL语句，自动处理重连
func (c *ReconnectableClient) ExecWithReconnect(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	var result sql.Result
	err := reconnect.WithReconnect(ctx, c.reconnector, "mysql", func() error {
		var err error
		result, err = c.Client.db.ExecContext(ctx, query, args...)
		return err
	})
	return result, err
}

// QueryWithReconnect 查询数据，自动处理重连
func (c *ReconnectableClient) QueryWithReconnect(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	var rows *sql.Rows
	err := reconnect.WithReconnect(ctx, c.reconnector, "mysql", func() error {
		var err error
		rows, err = c.Client.db.QueryContext(ctx, query, args...)
		return err
	})
	return rows, err
}

// QueryRowWithReconnect 查询单行数据，自动处理重连
func (c *ReconnectableClient) QueryRowWithReconnect(ctx context.Context, query string, args ...interface{}) *sql.Row {
	var row *sql.Row
	_ = reconnect.WithReconnect(ctx, c.reconnector, "mysql", func() error {
		row = c.Client.db.QueryRowContext(ctx, query, args...)
		return nil // QueryRow不会立即返回错误，而是在Scan时返回
	})
	return row
}

// BeginTxWithReconnect 开始事务，自动处理重连
func (c *ReconnectableClient) BeginTxWithReconnect(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	var tx *sql.Tx
	err := reconnect.WithReconnect(ctx, c.reconnector, "mysql", func() error {
		var err error
		tx, err = c.Client.db.BeginTx(ctx, opts)
		return err
	})
	return tx, err
}

// PingWithReconnect Ping数据库，自动处理重连
func (c *ReconnectableClient) PingWithReconnect(ctx context.Context) error {
	return reconnect.WithReconnect(ctx, c.reconnector, "mysql", func() error {
		return c.Client.db.PingContext(ctx)
	})
}
