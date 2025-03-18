package elasticsearch

import (
	"context"
	"net/http"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
	"app/pkg/storage/reconnect"
)

// ReconnectableClient Elasticsearch可自动重连的客户端
type ReconnectableClient struct {
	*Client
	reconnector *reconnect.BaseReconnector
}

// NewReconnectableClient 创建支持自动重连的Elasticsearch客户端
func NewReconnectableClient(config *Config) (*ReconnectableClient, error) {
	// 首先创建标准客户端
	client, err := NewClient(config)
	if err != nil {
		return nil, errors.Wrap(err, "创建Elasticsearch客户端失败")
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
	if c.Client.esClient != nil {
		c.Client.esClient = nil // ES客户端没有显式的Close方法
	}

	// 创建ES配置
	cfg := elasticsearch.Config{
		Addresses: c.Client.config.Addresses,
		Username:  c.Client.config.Username,
		Password:  c.Client.config.Password,
		Transport: &http.Transport{
			MaxIdleConns:        c.Client.config.MaxIdleConns,
			MaxIdleConnsPerHost: c.Client.config.MaxIdleConnsPerHost,
			MaxConnsPerHost:     c.Client.config.MaxConnsPerHost,
			IdleConnTimeout:     time.Duration(c.Client.config.IdleConnTimeout) * time.Second,
		},
	}

	// 创建新的ES客户端
	esClient, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return errors.Wrap(err, "创建ES客户端失败")
	}

	// 测试连接
	info, err := esClient.Info(
		esClient.Info.WithContext(ctx),
		esClient.Info.WithHuman(),
	)
	if err != nil {
		return errors.Wrap(err, "ES连接测试失败")
	}
	defer info.Body.Close()

	if info.IsError() {
		return errors.Errorf("ES返回错误: %s", info.String())
	}

	c.Client.esClient = esClient
	pr.Info("ES连接建立成功")
	return nil
}

// isConnected 检查连接状态
func (c *ReconnectableClient) isConnected() bool {
	if c.Client.esClient == nil {
		return false
	}

	// 使用ping来检查连接状态
	req := esapi.PingRequest{}
	res, err := req.Do(context.Background(), c.Client.esClient)
	if err != nil {
		return false
	}
	defer res.Body.Close()

	return res.StatusCode >= 200 && res.StatusCode < 300
}

// close 关闭ES连接
func (c *ReconnectableClient) close() error {
	// ES客户端没有显式的Close方法，但其使用的HTTP客户端可能需要关闭
	c.Client.esClient = nil
	return nil
}

// Reconnect 重新连接ES
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

// SearchWithReconnect 执行搜索，自动处理重连
func (c *ReconnectableClient) SearchWithReconnect(ctx context.Context, req *esapi.SearchRequest) (*esapi.Response, error) {
	var resp *esapi.Response
	err := reconnect.WithReconnect(ctx, c.reconnector, "elasticsearch", func() error {
		var err error
		resp, err = req.Do(ctx, c.Client.esClient)
		return err
	})
	return resp, err
}

// IndexWithReconnect 索引文档，自动处理重连
func (c *ReconnectableClient) IndexWithReconnect(ctx context.Context, req *esapi.IndexRequest) (*esapi.Response, error) {
	var resp *esapi.Response
	err := reconnect.WithReconnect(ctx, c.reconnector, "elasticsearch", func() error {
		var err error
		resp, err = req.Do(ctx, c.Client.esClient)
		return err
	})
	return resp, err
}

// UpdateWithReconnect 更新文档，自动处理重连
func (c *ReconnectableClient) UpdateWithReconnect(ctx context.Context, req *esapi.UpdateRequest) (*esapi.Response, error) {
	var resp *esapi.Response
	err := reconnect.WithReconnect(ctx, c.reconnector, "elasticsearch", func() error {
		var err error
		resp, err = req.Do(ctx, c.Client.esClient)
		return err
	})
	return resp, err
}

// DeleteWithReconnect 删除文档，自动处理重连
func (c *ReconnectableClient) DeleteWithReconnect(ctx context.Context, req *esapi.DeleteRequest) (*esapi.Response, error) {
	var resp *esapi.Response
	err := reconnect.WithReconnect(ctx, c.reconnector, "elasticsearch", func() error {
		var err error
		resp, err = req.Do(ctx, c.Client.esClient)
		return err
	})
	return resp, err
}

// BulkWithReconnect 批量操作，自动处理重连
func (c *ReconnectableClient) BulkWithReconnect(ctx context.Context, req *esapi.BulkRequest) (*esapi.Response, error) {
	var resp *esapi.Response
	err := reconnect.WithReconnect(ctx, c.reconnector, "elasticsearch", func() error {
		var err error
		resp, err = req.Do(ctx, c.Client.esClient)
		return err
	})
	return resp, err
}

// CountWithReconnect 计数，自动处理重连
func (c *ReconnectableClient) CountWithReconnect(ctx context.Context, req *esapi.CountRequest) (*esapi.Response, error) {
	var resp *esapi.Response
	err := reconnect.WithReconnect(ctx, c.reconnector, "elasticsearch", func() error {
		var err error
		resp, err = req.Do(ctx, c.Client.esClient)
		return err
	})
	return resp, err
}

// PingWithReconnect Ping ES服务器，自动处理重连
func (c *ReconnectableClient) PingWithReconnect(ctx context.Context) (*esapi.Response, error) {
	var resp *esapi.Response
	err := reconnect.WithReconnect(ctx, c.reconnector, "elasticsearch", func() error {
		var err error
		req := esapi.PingRequest{}
		resp, err = req.Do(ctx, c.Client.esClient)
		return err
	})
	return resp, err
}
