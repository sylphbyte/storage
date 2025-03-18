// Package mysql 提供MySQL数据库连接和操作功能
package mysql

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL驱动
	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
	"app/pkg/storage/reconnect"
)

// Config MySQL客户端配置
type Config struct {
	// DSN 数据源名称
	DSN string `yaml:"dsn" json:"dsn"`

	// MaxOpenConns 最大打开连接数
	MaxOpenConns int `yaml:"max_open_conns" json:"max_open_conns"`

	// MaxIdleConns 最大空闲连接数
	MaxIdleConns int `yaml:"max_idle_conns" json:"max_idle_conns"`

	// ConnMaxLifetime 连接最大生命周期(秒)
	ConnMaxLifetime int `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`

	// ReconnectPolicy 重连策略
	ReconnectPolicy *reconnect.ReconnectPolicy `yaml:"reconnect_policy" json:"reconnect_policy"`
}

// Client MySQL客户端
type Client struct {
	db     *sql.DB
	dsn    string
	config *Config
}

// NewClient 创建新的MySQL客户端
func NewClient(config *Config) (*Client, error) {
	if config == nil {
		return nil, errors.New("MySQL配置不能为空")
	}

	if config.DSN == "" {
		return nil, errors.New("MySQL DSN不能为空")
	}

	// 打开数据库连接
	db, err := sql.Open("mysql", config.DSN)
	if err != nil {
		return nil, errors.Wrap(err, "无法打开MySQL连接")
	}

	// 设置连接池参数
	if config.MaxOpenConns > 0 {
		db.SetMaxOpenConns(config.MaxOpenConns)
	} else {
		db.SetMaxOpenConns(10) // 默认值
	}

	if config.MaxIdleConns > 0 {
		db.SetMaxIdleConns(config.MaxIdleConns)
	} else {
		db.SetMaxIdleConns(5) // 默认值
	}

	if config.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(time.Duration(config.ConnMaxLifetime) * time.Second)
	} else {
		db.SetConnMaxLifetime(300 * time.Second) // 默认5分钟
	}

	// 测试连接
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, errors.Wrap(err, "MySQL ping失败")
	}

	pr.Info("MySQL连接成功: %s", config.DSN)

	return &Client{
		db:     db,
		dsn:    config.DSN,
		config: config,
	}, nil
}

// DB 返回原生SQL.DB对象
func (c *Client) DB() *sql.DB {
	return c.db
}

// Close 关闭数据库连接
func (c *Client) Close() error {
	if c.db != nil {
		err := c.db.Close()
		if err != nil {
			return errors.Wrap(err, "关闭MySQL连接失败")
		}
		c.db = nil
		pr.Info("MySQL连接已关闭")
	}
	return nil
}

// Ping 检查数据库连接是否正常
func (c *Client) Ping() error {
	if c.db == nil {
		return errors.New("MySQL连接未初始化")
	}

	err := c.db.Ping()
	if err != nil {
		return errors.Wrap(err, "MySQL ping失败")
	}

	return nil
}

// PingContext 带上下文的Ping
func (c *Client) PingContext(ctx context.Context) error {
	if c.db == nil {
		return errors.New("MySQL连接未初始化")
	}

	err := c.db.PingContext(ctx)
	if err != nil {
		return errors.Wrap(err, "MySQL ping失败")
	}

	return nil
}

// Exec 执行SQL语句
func (c *Client) Exec(query string, args ...interface{}) (sql.Result, error) {
	if c.db == nil {
		return nil, errors.New("MySQL连接未初始化")
	}

	result, err := c.db.Exec(query, args...)
	if err != nil {
		return nil, errors.Wrapf(err, "执行SQL失败: %s", query)
	}

	return result, nil
}

// ExecContext 带上下文的Exec
func (c *Client) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if c.db == nil {
		return nil, errors.New("MySQL连接未初始化")
	}

	result, err := c.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrapf(err, "执行SQL失败: %s", query)
	}

	return result, nil
}

// Query 查询多行数据
func (c *Client) Query(query string, args ...interface{}) (*sql.Rows, error) {
	if c.db == nil {
		return nil, errors.New("MySQL连接未初始化")
	}

	rows, err := c.db.Query(query, args...)
	if err != nil {
		return nil, errors.Wrapf(err, "查询失败: %s", query)
	}

	return rows, nil
}

// QueryContext 带上下文的Query
func (c *Client) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if c.db == nil {
		return nil, errors.New("MySQL连接未初始化")
	}

	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrapf(err, "查询失败: %s", query)
	}

	return rows, nil
}

// QueryRow 查询单行数据
func (c *Client) QueryRow(query string, args ...interface{}) *sql.Row {
	if c.db == nil {
		return nil // 不会立即返回错误，而是在Scan时返回
	}

	return c.db.QueryRow(query, args...)
}

// QueryRowContext 带上下文的QueryRow
func (c *Client) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if c.db == nil {
		return nil // 不会立即返回错误，而是在Scan时返回
	}

	return c.db.QueryRowContext(ctx, query, args...)
}

// Begin 开始一个事务
func (c *Client) Begin() (*sql.Tx, error) {
	if c.db == nil {
		return nil, errors.New("MySQL连接未初始化")
	}

	tx, err := c.db.Begin()
	if err != nil {
		return nil, errors.Wrap(err, "开始事务失败")
	}

	return tx, nil
}

// BeginTx 带上下文和选项的Begin
func (c *Client) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	if c.db == nil {
		return nil, errors.New("MySQL连接未初始化")
	}

	tx, err := c.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, "开始事务失败")
	}

	return tx, nil
}
