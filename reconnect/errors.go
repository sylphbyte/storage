package reconnect

import (
	"context"
	"database/sql/driver"
	"errors"
	"io"
	"net"
	"strings"
	"syscall"
)

// IsNetworkError 判断是否为网络相关错误
func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}

	// 检查是否为基本网络错误类型
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// 检查特定的网络错误
	if errors.Is(err, io.EOF) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ECONNABORTED) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ENETUNREACH) ||
		errors.Is(err, syscall.ENETRESET) {
		return true
	}

	// 检查错误信息（不够精确，但有些库封装了错误）
	errStr := err.Error()
	for _, s := range []string{
		"broken pipe",
		"connection reset",
		"connection refused",
		"network is unreachable",
		"connection timed out",
		"i/o timeout",
		"no route to host",
		"connection closed",
		"use of closed network connection",
	} {
		if strings.Contains(strings.ToLower(errStr), s) {
			return true
		}
	}

	return false
}

// IsMySQLReconnectableError 判断是否是需要重连的MySQL错误
func IsMySQLReconnectableError(err error) bool {
	if err == nil {
		return false
	}

	// 检查是否是网络错误
	if IsNetworkError(err) {
		return true
	}

	// 检查特定的MySQL错误
	errStr := strings.ToLower(err.Error())
	mysqlReconnectErrors := []string{
		// 常见的MySQL连接错误关键字
		"lost connection",
		"server has gone away",
		"broken pipe",
		"eof",
		"read-only connection",
		"connection was killed",
		"query execution was interrupted",
		"connection was closed",
		"communication link failure",
		"packet too large",
		"lock wait timeout",
		"1040", // Too many connections
		"1042", // Unable to connect
		"1043", // Bad handshake
		"2002", // Connection refused
		"2003", // Can't connect
		"2006", // Server has gone away
		"2013", // Lost connection
		"closed by the remote host",
	}

	for _, e := range mysqlReconnectErrors {
		if strings.Contains(errStr, e) {
			return true
		}
	}

	// 检查是否是driver.ErrBadConn错误
	return errors.Is(err, driver.ErrBadConn)
}

// IsRedisReconnectableError 判断是否是需要重连的Redis错误
func IsRedisReconnectableError(err error) bool {
	if err == nil {
		return false
	}

	// 检查是否是网络错误
	if IsNetworkError(err) {
		return true
	}

	// 检查特定的Redis错误消息
	errStr := strings.ToLower(err.Error())
	redisReconnectErrors := []string{
		"connection refused",
		"connection reset",
		"connection closed",
		"no connection",
		"connection timed out",
		"i/o timeout",
		"broken pipe",
		"eof",
		"reset by peer",
		"client is closed",
		"client sent an invalid line",
		"redis is loading",
		"master is down",
		"redis server: connection",
		"redis server: read",
		"redis server: write",
		"dial tcp",
		"pool is closed",
		"max number of connections",
		"connection pool timeout",
	}

	for _, e := range redisReconnectErrors {
		if strings.Contains(errStr, e) {
			return true
		}
	}

	return false
}

// IsElasticsearchReconnectableError 判断是否是需要重连的Elasticsearch错误
func IsElasticsearchReconnectableError(err error) bool {
	if err == nil {
		return false
	}

	// 检查是否是网络错误
	if IsNetworkError(err) {
		return true
	}

	// 检查特定的Elasticsearch错误消息
	errStr := strings.ToLower(err.Error())
	esReconnectErrors := []string{
		"connection refused",
		"connection reset",
		"cannot get connection",
		"no available connection",
		"all the connections are busy",
		"connection timed out",
		"no connection available",
		"no node available",
		"circuit breaker",
		"cluster state",
		"timeout waiting",
		"i/o timeout",
		"read timeout",
		"write timeout",
		"connection closed",
		"connection pool",
		"dial tcp",
	}

	for _, e := range esReconnectErrors {
		if strings.Contains(errStr, e) {
			return true
		}
	}

	return false
}

// IsReconnectableError 根据错误类型和存储类型判断是否需要重连
func IsReconnectableError(err error, storageType string) bool {
	if err == nil {
		return false
	}

	switch strings.ToLower(storageType) {
	case "mysql":
		return IsMySQLReconnectableError(err)
	case "redis":
		return IsRedisReconnectableError(err)
	case "elasticsearch":
		return IsElasticsearchReconnectableError(err)
	default:
		// 默认情况下仅判断是否为网络错误
		return IsNetworkError(err)
	}
}

// WithReconnect 提供一个通用的重连包装函数
// reconnector: 重连器
// storageType: 存储类型，用于错误判断
// operation: 要执行的操作
func WithReconnect(ctx context.Context, reconnector Reconnector, storageType string, operation func() error) error {
	// 执行操作
	err := operation()
	if err == nil || !IsReconnectableError(err, storageType) {
		return err
	}

	// 需要重连
	if reconnectErr := reconnector.Reconnect(ctx); reconnectErr != nil {
		// 重连失败，返回原始错误
		return err
	}

	// 重连成功，重试操作
	return operation()
}
