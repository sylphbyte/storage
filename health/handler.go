package health

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
)

// WebhookConfig 网络钩子配置
type WebhookConfig struct {
	URL             string            // 网络钩子URL
	Method          string            // HTTP方法，默认为POST
	Headers         map[string]string // 请求头
	Timeout         time.Duration     // 请求超时时间
	RetryCount      int               // 重试次数
	RetryDelay      time.Duration     // 重试延迟
	LevelFilters    []AlertLevel      // 级别过滤
	PayloadTemplate string            // 载荷模板(如果为空则使用默认JSON结构)
}

// WebhookHandler 网络钩子告警处理器
type WebhookHandler struct {
	name      string
	config    WebhookConfig
	client    *http.Client
	bufferMux sync.Mutex
	buffer    []Alert // 失败告警缓冲区
	maxBuffer int     // 最大缓冲区大小
}

// NewWebhookHandler 创建新的网络钩子告警处理器
func NewWebhookHandler(name string, config WebhookConfig) (*WebhookHandler, error) {
	if name == "" {
		name = "webhook_handler"
	}

	if config.URL == "" {
		return nil, errors.New("webhook URL不能为空")
	}

	if config.Method == "" {
		config.Method = "POST"
	}

	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Second
	}

	client := &http.Client{
		Timeout: config.Timeout,
	}

	handler := &WebhookHandler{
		name:      name,
		config:    config,
		client:    client,
		buffer:    make([]Alert, 0),
		maxBuffer: 100, // 默认最多缓存100条告警
	}

	return handler, nil
}

// HandleAlert 处理告警
func (h *WebhookHandler) HandleAlert(alert Alert) error {
	// 级别过滤
	if len(h.config.LevelFilters) > 0 {
		matched := false
		for _, level := range h.config.LevelFilters {
			if alert.Level == level {
				matched = true
				break
			}
		}
		if !matched {
			return nil // 级别不匹配，跳过
		}
	}

	// 构建载荷
	payload, err := h.buildPayload(alert)
	if err != nil {
		return errors.Wrap(err, "构建webhook载荷失败")
	}

	// 发送请求
	err = h.sendRequest(payload)
	if err != nil {
		// 如果配置了重试，尝试重试
		if h.config.RetryCount > 0 {
			for i := 0; i < h.config.RetryCount; i++ {
				time.Sleep(h.config.RetryDelay)
				pr.Warning("Webhook告警发送失败，正在进行第%d次重试", i+1)

				err = h.sendRequest(payload)
				if err == nil {
					return nil
				}
			}
		}

		// 如果所有重试都失败，缓存告警
		h.bufferAlert(alert)
		return errors.Wrap(err, "发送webhook告警失败")
	}

	return nil
}

// Name 获取处理器名称
func (h *WebhookHandler) Name() string {
	return h.name
}

// 缓存告警
func (h *WebhookHandler) bufferAlert(alert Alert) {
	h.bufferMux.Lock()
	defer h.bufferMux.Unlock()

	// 如果缓冲区已满，删除最旧的告警
	if len(h.buffer) >= h.maxBuffer {
		h.buffer = h.buffer[1:]
	}

	h.buffer = append(h.buffer, alert)
	pr.Warning("Webhook告警发送失败，已缓存告警: %s", alert.ID)
}

// 构建载荷
func (h *WebhookHandler) buildPayload(alert Alert) ([]byte, error) {
	// 如果有自定义模板，使用模板生成载荷
	if h.config.PayloadTemplate != "" {
		// 这里可以实现模板引擎
		// 简化起见，这里直接使用json编码
	}

	// 默认使用JSON编码
	return json.Marshal(alert)
}

// 发送请求
func (h *WebhookHandler) sendRequest(payload []byte) error {
	req, err := http.NewRequest(h.config.Method, h.config.URL, bytes.NewBuffer(payload))
	if err != nil {
		return errors.Wrap(err, "创建HTTP请求失败")
	}

	// 设置默认Content-Type
	if _, exists := h.config.Headers["Content-Type"]; !exists {
		req.Header.Set("Content-Type", "application/json")
	}

	// 设置自定义请求头
	for key, value := range h.config.Headers {
		req.Header.Set(key, value)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "执行HTTP请求失败")
	}
	defer resp.Body.Close()

	// 检查HTTP状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return errors.Errorf("webhook请求返回非成功状态码: %d, 响应: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// RetryBuffered 重试发送缓冲区中的告警
func (h *WebhookHandler) RetryBuffered() int {
	h.bufferMux.Lock()
	bufferCopy := make([]Alert, len(h.buffer))
	copy(bufferCopy, h.buffer)
	h.bufferMux.Unlock()

	if len(bufferCopy) == 0 {
		return 0
	}

	pr.Info("尝试重新发送 %d 条缓存的webhook告警", len(bufferCopy))

	successCount := 0
	for i, alert := range bufferCopy {
		payload, err := h.buildPayload(alert)
		if err != nil {
			pr.Error("构建webhook载荷失败: %v", err)
			continue
		}

		err = h.sendRequest(payload)
		if err == nil {
			// 发送成功，从缓冲区中移除
			h.bufferMux.Lock()
			for j, a := range h.buffer {
				if a.ID == alert.ID {
					h.buffer = append(h.buffer[:j], h.buffer[j+1:]...)
					break
				}
			}
			h.bufferMux.Unlock()

			successCount++
		} else {
			pr.Error("重新发送webhook告警失败: %v", err)
		}

		// 每发送5条休息一下，避免过度请求
		if i > 0 && i%5 == 0 {
			time.Sleep(time.Second)
		}
	}

	pr.Info("成功重新发送 %d/%d 条缓存的webhook告警", successCount, len(bufferCopy))
	return successCount
}

// FileConfig 文件处理器配置
type FileConfig struct {
	FilePath     string       // 文件路径
	LevelFilters []AlertLevel // 级别过滤
	Format       string       // 格式(text, json, csv)
	RotateSize   int64        // 文件大小限制(字节)，超过则切割
	MaxFiles     int          // 最大保留文件数
	Permissions  os.FileMode  // 文件权限
}

// FileHandler 文件告警处理器
type FileHandler struct {
	name        string
	config      FileConfig
	fileMux     sync.Mutex
	file        *os.File
	currentSize int64
}

// NewFileHandler 创建新的文件告警处理器
func NewFileHandler(name string, config FileConfig) (*FileHandler, error) {
	if name == "" {
		name = "file_handler"
	}

	if config.FilePath == "" {
		return nil, errors.New("文件路径不能为空")
	}

	if config.Format == "" {
		config.Format = "text"
	} else if config.Format != "text" && config.Format != "json" && config.Format != "csv" {
		return nil, errors.New("不支持的文件格式: " + config.Format)
	}

	if config.Permissions == 0 {
		config.Permissions = 0644
	}

	// 创建目录
	dir := filepath.Dir(config.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, errors.Wrap(err, "创建目录失败")
	}

	// 打开文件
	file, err := os.OpenFile(config.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, config.Permissions)
	if err != nil {
		return nil, errors.Wrap(err, "打开文件失败")
	}

	// 获取当前文件大小
	fileInfo, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, errors.Wrap(err, "获取文件信息失败")
	}

	handler := &FileHandler{
		name:        name,
		config:      config,
		file:        file,
		currentSize: fileInfo.Size(),
	}

	return handler, nil
}

// HandleAlert 处理告警
func (h *FileHandler) HandleAlert(alert Alert) error {
	// 级别过滤
	if len(h.config.LevelFilters) > 0 {
		matched := false
		for _, level := range h.config.LevelFilters {
			if alert.Level == level {
				matched = true
				break
			}
		}
		if !matched {
			return nil // 级别不匹配，跳过
		}
	}

	// 格式化告警
	var content []byte
	var err error

	switch h.config.Format {
	case "json":
		content, err = json.Marshal(alert)
		if err != nil {
			return errors.Wrap(err, "JSON编码告警失败")
		}
		content = append(content, '\n')

	case "csv":
		// 简单的CSV格式
		row := fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s\n",
			alert.ID,
			alert.Timestamp.Format(time.RFC3339),
			alert.Level,
			alert.ServiceType,
			alert.ServiceName,
			alert.Name,
			strings.ReplaceAll(alert.Message, ",", "\\,"), // 转义逗号
		)
		content = []byte(row)

	default: // text
		// 人类可读的格式
		text := fmt.Sprintf("[%s] %s - %s/%s: %s - %s\n",
			alert.Timestamp.Format("2006-01-02 15:04:05"),
			alert.Level,
			alert.ServiceType,
			alert.ServiceName,
			alert.Name,
			alert.Message,
		)
		content = []byte(text)
	}

	// 写入文件
	h.fileMux.Lock()
	defer h.fileMux.Unlock()

	// 检查是否需要轮转
	if h.config.RotateSize > 0 && h.currentSize+int64(len(content)) > h.config.RotateSize {
		if err := h.rotateFile(); err != nil {
			return errors.Wrap(err, "轮转文件失败")
		}
	}

	n, err := h.file.Write(content)
	if err != nil {
		return errors.Wrap(err, "写入文件失败")
	}

	h.currentSize += int64(n)
	return nil
}

// Name 获取处理器名称
func (h *FileHandler) Name() string {
	return h.name
}

// 轮转文件
func (h *FileHandler) rotateFile() error {
	// 关闭当前文件
	if err := h.file.Close(); err != nil {
		return errors.Wrap(err, "关闭文件失败")
	}

	// 获取当前时间戳
	timestamp := time.Now().Format("20060102150405")

	// 重命名当前文件
	rotatedPath := fmt.Sprintf("%s.%s", h.config.FilePath, timestamp)
	if err := os.Rename(h.config.FilePath, rotatedPath); err != nil {
		return errors.Wrap(err, "重命名文件失败")
	}

	// 清理旧文件
	if h.config.MaxFiles > 0 {
		if err := h.cleanOldFiles(); err != nil {
			pr.Warning("清理旧告警文件失败: %v", err)
		}
	}

	// 创建新文件
	file, err := os.OpenFile(h.config.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, h.config.Permissions)
	if err != nil {
		return errors.Wrap(err, "创建新文件失败")
	}

	h.file = file
	h.currentSize = 0

	return nil
}

// 清理旧文件
func (h *FileHandler) cleanOldFiles() error {
	dir := filepath.Dir(h.config.FilePath)
	base := filepath.Base(h.config.FilePath)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return errors.Wrap(err, "读取目录失败")
	}

	// 收集所有轮转的文件
	var rotatedFiles []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, base+".") && !entry.IsDir() {
			rotatedFiles = append(rotatedFiles, filepath.Join(dir, name))
		}
	}

	// 如果文件数超过最大值，删除最旧的文件
	if len(rotatedFiles) > h.config.MaxFiles-1 {
		// 按名称排序（由于时间戳格式，这实际上是按时间排序）
		sort.Strings(rotatedFiles)

		// 删除超出限制的最旧文件
		filesToRemove := rotatedFiles[:len(rotatedFiles)-(h.config.MaxFiles-1)]
		for _, file := range filesToRemove {
			if err := os.Remove(file); err != nil {
				pr.Warning("删除旧告警文件失败: %s - %v", file, err)
			} else {
				pr.Info("已删除旧告警文件: %s", file)
			}
		}
	}

	return nil
}

// Close 关闭处理器
func (h *FileHandler) Close() error {
	h.fileMux.Lock()
	defer h.fileMux.Unlock()

	if h.file != nil {
		return h.file.Close()
	}
	return nil
}
