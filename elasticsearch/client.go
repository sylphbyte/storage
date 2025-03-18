// Package elasticsearch 提供Elasticsearch操作的相关功能
package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
	"app/pkg/storage/reconnect"
)

// Config Elasticsearch客户端配置
type Config struct {
	// Addresses ES地址列表
	Addresses []string `yaml:"addresses" json:"addresses"`

	// Username 用户名
	Username string `yaml:"username" json:"username"`

	// Password 密码
	Password string `yaml:"password" json:"password"`

	// MaxIdleConns 最大空闲连接数
	MaxIdleConns int `yaml:"max_idle_conns" json:"max_idle_conns"`

	// MaxIdleConnsPerHost 每个主机最大空闲连接数
	MaxIdleConnsPerHost int `yaml:"max_idle_conns_per_host" json:"max_idle_conns_per_host"`

	// MaxConnsPerHost 每个主机最大连接数
	MaxConnsPerHost int `yaml:"max_conns_per_host" json:"max_conns_per_host"`

	// IdleConnTimeout 空闲连接超时(秒)
	IdleConnTimeout int `yaml:"idle_conn_timeout" json:"idle_conn_timeout"`

	// ReconnectPolicy 重连策略
	ReconnectPolicy *reconnect.ReconnectPolicy `yaml:"reconnect_policy" json:"reconnect_policy"`
}

// Client Elasticsearch客户端
type Client struct {
	esClient *elasticsearch.Client
	config   *Config
}

// NewClient 创建新的Elasticsearch客户端
func NewClient(config *Config) (*Client, error) {
	if config == nil {
		return nil, errors.New("ES配置不能为空")
	}

	if len(config.Addresses) == 0 {
		return nil, errors.New("ES地址不能为空")
	}

	// 创建ES配置
	cfg := elasticsearch.Config{
		Addresses: config.Addresses,
		Username:  config.Username,
		Password:  config.Password,
		Transport: &http.Transport{
			MaxIdleConns:        config.MaxIdleConns,
			MaxIdleConnsPerHost: config.MaxIdleConnsPerHost,
			MaxConnsPerHost:     config.MaxConnsPerHost,
			IdleConnTimeout:     time.Duration(config.IdleConnTimeout) * time.Second,
		},
	}

	// 创建ES客户端
	esClient, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "创建ES客户端失败")
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := esClient.Info(
		esClient.Info.WithContext(ctx),
		esClient.Info.WithHuman(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "ES连接测试失败")
	}
	defer info.Body.Close()

	if info.IsError() {
		return nil, errors.Errorf("ES返回错误: %s", info.String())
	}

	pr.Info("ES连接成功: %s", config.Addresses)

	return &Client{
		esClient: esClient,
		config:   config,
	}, nil
}

// Client 返回原生ES客户端
func (c *Client) Client() *elasticsearch.Client {
	return c.esClient
}

// Close 关闭ES连接
func (c *Client) Close() error {
	// ES客户端没有显式的Close方法，但我们可以清空引用
	c.esClient = nil
	pr.Info("ES连接已关闭")
	return nil
}

// Ping 测试ES连接
func (c *Client) Ping(ctx context.Context) (*esapi.Response, error) {
	if c.esClient == nil {
		return nil, errors.New("ES连接未初始化")
	}

	req := esapi.PingRequest{}
	res, err := req.Do(ctx, c.esClient)
	if err != nil {
		return nil, errors.Wrap(err, "ES ping失败")
	}

	return res, nil
}

// Info 获取ES集群信息
func (c *Client) Info(ctx context.Context) (*esapi.Response, error) {
	if c.esClient == nil {
		return nil, errors.New("ES连接未初始化")
	}

	res, err := c.esClient.Info(
		c.esClient.Info.WithContext(ctx),
		c.esClient.Info.WithHuman(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "获取ES信息失败")
	}

	return res, nil
}

// Search 执行搜索
func (c *Client) Search(ctx context.Context, index string, query interface{}) (*esapi.Response, error) {
	if c.esClient == nil {
		return nil, errors.New("ES连接未初始化")
	}

	var body []byte
	var err error
	if query != nil {
		body, err = json.Marshal(query)
		if err != nil {
			return nil, errors.Wrap(err, "查询JSON编码失败")
		}
	}

	req := esapi.SearchRequest{
		Index: []string{index},
		Body:  bytes.NewReader(body),
	}

	res, err := req.Do(ctx, c.esClient)
	if err != nil {
		return nil, errors.Wrap(err, "ES搜索失败")
	}

	return res, nil
}

// Index 索引文档
func (c *Client) Index(ctx context.Context, index string, id string, document interface{}) (*esapi.Response, error) {
	if c.esClient == nil {
		return nil, errors.New("ES连接未初始化")
	}

	body, err := json.Marshal(document)
	if err != nil {
		return nil, errors.Wrap(err, "文档JSON编码失败")
	}

	req := esapi.IndexRequest{
		Index:      index,
		DocumentID: id,
		Body:       bytes.NewReader(body),
		Refresh:    "false",
	}

	res, err := req.Do(ctx, c.esClient)
	if err != nil {
		return nil, errors.Wrap(err, "ES索引文档失败")
	}

	return res, nil
}

// Update 更新文档
func (c *Client) Update(ctx context.Context, index string, id string, update interface{}) (*esapi.Response, error) {
	if c.esClient == nil {
		return nil, errors.New("ES连接未初始化")
	}

	body, err := json.Marshal(map[string]interface{}{"doc": update})
	if err != nil {
		return nil, errors.Wrap(err, "更新文档JSON编码失败")
	}

	req := esapi.UpdateRequest{
		Index:      index,
		DocumentID: id,
		Body:       bytes.NewReader(body),
	}

	res, err := req.Do(ctx, c.esClient)
	if err != nil {
		return nil, errors.Wrap(err, "ES更新文档失败")
	}

	return res, nil
}

// Delete 删除文档
func (c *Client) Delete(ctx context.Context, index string, id string) (*esapi.Response, error) {
	if c.esClient == nil {
		return nil, errors.New("ES连接未初始化")
	}

	req := esapi.DeleteRequest{
		Index:      index,
		DocumentID: id,
	}

	res, err := req.Do(ctx, c.esClient)
	if err != nil {
		return nil, errors.Wrap(err, "ES删除文档失败")
	}

	return res, nil
}

// Bulk 批量操作
func (c *Client) Bulk(ctx context.Context, body []byte) (*esapi.Response, error) {
	if c.esClient == nil {
		return nil, errors.New("ES连接未初始化")
	}

	req := esapi.BulkRequest{
		Body: bytes.NewReader(body),
	}

	res, err := req.Do(ctx, c.esClient)
	if err != nil {
		return nil, errors.Wrap(err, "ES批量操作失败")
	}

	return res, nil
}

// Count 计数
func (c *Client) Count(ctx context.Context, index string, query interface{}) (*esapi.Response, error) {
	if c.esClient == nil {
		return nil, errors.New("ES连接未初始化")
	}

	var body []byte
	var err error
	if query != nil {
		body, err = json.Marshal(query)
		if err != nil {
			return nil, errors.Wrap(err, "查询JSON编码失败")
		}
	}

	req := esapi.CountRequest{
		Index: []string{index},
		Body:  bytes.NewReader(body),
	}

	res, err := req.Do(ctx, c.esClient)
	if err != nil {
		return nil, errors.Wrap(err, "ES计数失败")
	}

	return res, nil
}

// CreateIndex 创建索引
func (c *Client) CreateIndex(ctx context.Context, index string, mapping interface{}) (*esapi.Response, error) {
	if c.esClient == nil {
		return nil, errors.New("ES连接未初始化")
	}

	var body []byte
	var err error
	if mapping != nil {
		body, err = json.Marshal(mapping)
		if err != nil {
			return nil, errors.Wrap(err, "映射JSON编码失败")
		}
	}

	req := esapi.IndicesCreateRequest{
		Index: index,
		Body:  bytes.NewReader(body),
	}

	res, err := req.Do(ctx, c.esClient)
	if err != nil {
		return nil, errors.Wrap(err, "ES创建索引失败")
	}

	return res, nil
}

// DeleteIndex 删除索引
func (c *Client) DeleteIndex(ctx context.Context, index string) (*esapi.Response, error) {
	if c.esClient == nil {
		return nil, errors.New("ES连接未初始化")
	}

	req := esapi.IndicesDeleteRequest{
		Index: []string{index},
	}

	res, err := req.Do(ctx, c.esClient)
	if err != nil {
		return nil, errors.Wrap(err, "ES删除索引失败")
	}

	return res, nil
}
