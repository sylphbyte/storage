// Package health 提供存储服务的健康检查功能
package health

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
)

// RedisChecker Redis健康检查器
type RedisChecker struct {
	*Checker
	client        *redis.Client      // Redis客户端
	pingTimeout   time.Duration      // ping超时时间
	checkSettings RedisCheckSettings // 检查设置
}

// RedisCheckSettings Redis健康检查设置
type RedisCheckSettings struct {
	CheckMemory       bool    // 是否检查内存
	CheckClients      bool    // 是否检查客户端连接
	CheckReplication  bool    // 是否检查复制状态
	CheckCommandTime  bool    // 是否检查命令执行时间
	TestKeyPrefix     string  // 测试键前缀
	MemoryHighPct     float64 // 内存使用率高阈值
	MemoryCriticalPct float64 // 内存使用率危险阈值
	SlowCommandMs     int64   // 慢命令阈值（毫秒）
}

// DefaultRedisCheckSettings 创建默认Redis检查设置
func DefaultRedisCheckSettings() RedisCheckSettings {
	return RedisCheckSettings{
		CheckMemory:       true,
		CheckClients:      true,
		CheckReplication:  true,
		CheckCommandTime:  true,
		TestKeyPrefix:     "health_check:",
		MemoryHighPct:     80.0,
		MemoryCriticalPct: 90.0,
		SlowCommandMs:     100,
	}
}

// NewRedisChecker 创建新的Redis健康检查器
func NewRedisChecker(name string, client *redis.Client, settings *RedisCheckSettings) *RedisChecker {
	if settings == nil {
		defaultSettings := DefaultRedisCheckSettings()
		settings = &defaultSettings
	}

	return &RedisChecker{
		Checker:       NewChecker(name, "redis"),
		client:        client,
		pingTimeout:   5 * time.Second,
		checkSettings: *settings,
	}
}

// Check 执行Redis健康检查
func (r *RedisChecker) Check(ctx context.Context) (HealthStatus, error) {
	startTime := time.Now()

	// 创建健康状态对象
	healthStatus := NewHealthStatus(StatusHealthy)

	// 添加健康检查基本信息
	healthStatus.Details["service_type"] = "redis"
	healthStatus.Details["check_name"] = r.Name()

	// 创建带超时的上下文
	timeoutCtx, cancel := context.WithTimeout(ctx, r.pingTimeout)
	defer cancel()

	// 基本连接测试
	pingCmd := r.client.Ping(timeoutCtx)
	pingErr := pingCmd.Err()
	if pingErr != nil {
		healthStatus.Status = StatusUnhealthy
		healthStatus.Details["ping_error"] = pingErr.Error()
		healthStatus.Latency = time.Since(startTime)

		r.RecordResult(healthStatus, pingErr)
		return healthStatus, errors.Wrap(pingErr, "Redis ping 失败")
	}

	healthStatus.Details["ping"] = "successful"

	// 检查SET/GET功能
	testKey := r.checkSettings.TestKeyPrefix + "test_" + strconv.FormatInt(time.Now().Unix(), 10)
	testValue := fmt.Sprintf("health_check_%d", time.Now().UnixNano())

	err := r.client.Set(timeoutCtx, testKey, testValue, 10*time.Second).Err()
	if err != nil {
		healthStatus.Status = StatusDegraded
		healthStatus.Details["set_error"] = err.Error()
		healthStatus.Details["degraded_reason"] = "Redis SET操作失败"

		r.RecordResult(healthStatus, err)
		return healthStatus, errors.Wrap(err, "Redis SET操作失败")
	}

	getCmd := r.client.Get(timeoutCtx, testKey)
	if getCmd.Err() != nil {
		healthStatus.Status = StatusDegraded
		healthStatus.Details["get_error"] = getCmd.Err().Error()
		healthStatus.Details["degraded_reason"] = "Redis GET操作失败"

		r.RecordResult(healthStatus, getCmd.Err())
		return healthStatus, errors.Wrap(getCmd.Err(), "Redis GET操作失败")
	}

	if getCmd.Val() != testValue {
		healthStatus.Status = StatusDegraded
		healthStatus.Details["get_error"] = "数据不一致"
		healthStatus.Details["degraded_reason"] = "Redis GET结果与SET不一致"

		r.RecordResult(healthStatus, errors.New("Redis数据不一致"))
		return healthStatus, errors.New("Redis GET结果与SET不一致")
	}

	// 运行其他检查
	if r.checkSettings.CheckMemory {
		err := r.checkMemory(timeoutCtx, &healthStatus)
		if err != nil {
			pr.Warning("Redis内存检查失败: %v", err)
		}
	}

	if r.checkSettings.CheckClients {
		err := r.checkClients(timeoutCtx, &healthStatus)
		if err != nil {
			pr.Warning("Redis客户端检查失败: %v", err)
		}
	}

	if r.checkSettings.CheckReplication {
		err := r.checkReplication(timeoutCtx, &healthStatus)
		if err != nil {
			pr.Warning("Redis复制状态检查失败: %v", err)
		}
	}

	if r.checkSettings.CheckCommandTime {
		err := r.checkCommandTime(timeoutCtx, &healthStatus)
		if err != nil {
			pr.Warning("Redis命令执行时间检查失败: %v", err)
		}
	}

	// 设置检查延迟
	healthStatus.Latency = time.Since(startTime)

	// 记录检查结果
	r.RecordResult(healthStatus, nil)

	return healthStatus, nil
}

// checkMemory 检查Redis内存使用情况
func (r *RedisChecker) checkMemory(ctx context.Context, healthStatus *HealthStatus) error {
	info, err := r.client.Info(ctx, "memory").Result()
	if err != nil {
		healthStatus.Details["memory_check_error"] = err.Error()
		return errors.Wrap(err, "获取Redis内存信息失败")
	}

	// 解析内存信息
	var usedMemory, totalMemory, maxMemory int64

	lines := strings.Split(info, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "used_memory:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				usedMemory, _ = strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
				healthStatus.Details["used_memory"] = fmt.Sprintf("%d", usedMemory)
			}
		} else if strings.HasPrefix(line, "maxmemory:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				maxMemory, _ = strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
				healthStatus.Details["max_memory"] = fmt.Sprintf("%d", maxMemory)
			}
		} else if strings.HasPrefix(line, "total_system_memory:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				totalMemory, _ = strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
				healthStatus.Details["total_system_memory"] = fmt.Sprintf("%d", totalMemory)
			}
		}
	}

	// 如果没有设置maxmemory，使用系统总内存作为参考值
	if maxMemory == 0 && totalMemory > 0 {
		maxMemory = totalMemory
	}

	// 计算内存使用率
	var memoryUsagePct float64
	if maxMemory > 0 {
		memoryUsagePct = float64(usedMemory) / float64(maxMemory) * 100
		healthStatus.Details["memory_usage_pct"] = fmt.Sprintf("%.2f", memoryUsagePct)

		// 检查内存使用率是否超过阈值
		if memoryUsagePct > r.checkSettings.MemoryCriticalPct {
			healthStatus.Status = StatusDegraded
			healthStatus.Details["degraded_reason"] = fmt.Sprintf(
				"内存使用率非常高: %.2f%% > %.2f%%",
				memoryUsagePct,
				r.checkSettings.MemoryCriticalPct,
			)
		} else if memoryUsagePct > r.checkSettings.MemoryHighPct && healthStatus.Status == StatusHealthy {
			healthStatus.Status = StatusDegraded
			healthStatus.Details["degraded_reason"] = fmt.Sprintf(
				"内存使用率较高: %.2f%% > %.2f%%",
				memoryUsagePct,
				r.checkSettings.MemoryHighPct,
			)
		}
	}

	return nil
}

// checkClients 检查Redis客户端连接
func (r *RedisChecker) checkClients(ctx context.Context, healthStatus *HealthStatus) error {
	info, err := r.client.Info(ctx, "clients").Result()
	if err != nil {
		healthStatus.Details["clients_check_error"] = err.Error()
		return errors.Wrap(err, "获取Redis客户端信息失败")
	}

	// 解析客户端信息
	lines := strings.Split(info, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "connected_clients:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				connectedClients, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
				healthStatus.Details["connected_clients"] = fmt.Sprintf("%d", connectedClients)
			}
		} else if strings.HasPrefix(line, "blocked_clients:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				blockedClients, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
				healthStatus.Details["blocked_clients"] = fmt.Sprintf("%d", blockedClients)

				// 如果有阻塞的客户端，标记为降级状态
				if blockedClients > 0 && healthStatus.Status == StatusHealthy {
					healthStatus.Status = StatusDegraded
					healthStatus.Details["degraded_reason"] = fmt.Sprintf("检测到%d个阻塞的客户端", blockedClients)
				}
			}
		} else if strings.HasPrefix(line, "maxclients:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				maxClients, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
				healthStatus.Details["max_clients"] = fmt.Sprintf("%d", maxClients)
			}
		}
	}

	return nil
}

// checkReplication 检查Redis复制状态
func (r *RedisChecker) checkReplication(ctx context.Context, healthStatus *HealthStatus) error {
	info, err := r.client.Info(ctx, "replication").Result()
	if err != nil {
		healthStatus.Details["replication_check_error"] = err.Error()
		return errors.Wrap(err, "获取Redis复制信息失败")
	}

	// 解析复制信息
	// 使用 _ 表示我们不关心主库状态
	var isSlave bool

	lines := strings.Split(info, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "role:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				role := strings.TrimSpace(parts[1])
				healthStatus.Details["role"] = role

				if role == "master" {
					healthStatus.Details["is_master"] = "true"
				} else if role == "slave" {
					isSlave = true
					healthStatus.Details["is_slave"] = "true"
				}
			}
		} else if strings.HasPrefix(line, "master_link_status:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				linkStatus := strings.TrimSpace(parts[1])
				healthStatus.Details["master_link_status"] = linkStatus

				// 如果从库连接状态不是up，标记为降级状态
				if isSlave && linkStatus != "up" && healthStatus.Status == StatusHealthy {
					healthStatus.Status = StatusDegraded
					healthStatus.Details["degraded_reason"] = "从库连接状态不正常: " + linkStatus
				}
			}
		} else if strings.HasPrefix(line, "master_last_io_seconds_ago:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				lastIoSecondsAgo, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
				healthStatus.Details["master_last_io_seconds_ago"] = fmt.Sprintf("%d", lastIoSecondsAgo)

				// 如果从库超过30秒没有与主库通信，标记为降级状态
				if isSlave && lastIoSecondsAgo > 30 && healthStatus.Status == StatusHealthy {
					healthStatus.Status = StatusDegraded
					healthStatus.Details["degraded_reason"] = fmt.Sprintf(
						"从库与主库通信滞后: %d秒",
						lastIoSecondsAgo,
					)
				}
			}
		} else if strings.HasPrefix(line, "master_sync_in_progress:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				syncInProgress := strings.TrimSpace(parts[1])
				healthStatus.Details["master_sync_in_progress"] = syncInProgress
			}
		}
	}

	return nil
}

// checkCommandTime 检查Redis命令执行时间
func (r *RedisChecker) checkCommandTime(ctx context.Context, healthStatus *HealthStatus) error {
	// 进行一系列基本操作，并测量执行时间
	testKey := r.checkSettings.TestKeyPrefix + "cmd_time_" + strconv.FormatInt(time.Now().Unix(), 10)
	testValue := fmt.Sprintf("cmd_time_%d", time.Now().UnixNano())

	// 测试SET命令
	setStart := time.Now()
	err := r.client.Set(ctx, testKey, testValue, 10*time.Second).Err()
	setTime := time.Since(setStart)

	if err != nil {
		return errors.Wrap(err, "测试SET命令失败")
	}

	healthStatus.Details["set_command_time_ms"] = fmt.Sprintf("%.2f", float64(setTime.Microseconds())/1000)

	// 测试GET命令
	getStart := time.Now()
	getCmd := r.client.Get(ctx, testKey)
	getTime := time.Since(getStart)

	if getCmd.Err() != nil {
		return errors.Wrap(getCmd.Err(), "测试GET命令失败")
	}

	healthStatus.Details["get_command_time_ms"] = fmt.Sprintf("%.2f", float64(getTime.Microseconds())/1000)

	// 测试HSET/HGET命令
	hsetStart := time.Now()
	err = r.client.HSet(ctx, testKey+"_hash", "field1", "value1").Err()
	hsetTime := time.Since(hsetStart)

	if err != nil {
		return errors.Wrap(err, "测试HSET命令失败")
	}

	healthStatus.Details["hset_command_time_ms"] = fmt.Sprintf("%.2f", float64(hsetTime.Microseconds())/1000)

	// 测试删除
	delStart := time.Now()
	err = r.client.Del(ctx, testKey, testKey+"_hash").Err()
	delTime := time.Since(delStart)

	if err != nil {
		return errors.Wrap(err, "测试DEL命令失败")
	}

	healthStatus.Details["del_command_time_ms"] = fmt.Sprintf("%.2f", float64(delTime.Microseconds())/1000)

	// 检查是否有任何操作超过慢命令阈值
	slowCommands := make([]string, 0)

	if setTime.Milliseconds() > r.checkSettings.SlowCommandMs {
		slowCommands = append(slowCommands, fmt.Sprintf("SET(%.2fms)", float64(setTime.Microseconds())/1000))
	}

	if getTime.Milliseconds() > r.checkSettings.SlowCommandMs {
		slowCommands = append(slowCommands, fmt.Sprintf("GET(%.2fms)", float64(getTime.Microseconds())/1000))
	}

	if hsetTime.Milliseconds() > r.checkSettings.SlowCommandMs {
		slowCommands = append(slowCommands, fmt.Sprintf("HSET(%.2fms)", float64(hsetTime.Microseconds())/1000))
	}

	if delTime.Milliseconds() > r.checkSettings.SlowCommandMs {
		slowCommands = append(slowCommands, fmt.Sprintf("DEL(%.2fms)", float64(delTime.Microseconds())/1000))
	}

	if len(slowCommands) > 0 && healthStatus.Status == StatusHealthy {
		healthStatus.Status = StatusDegraded
		healthStatus.Details["degraded_reason"] = fmt.Sprintf("检测到慢命令: %s", strings.Join(slowCommands, ", "))
		healthStatus.Details["slow_commands"] = strings.Join(slowCommands, ",")
	}

	return nil
}

// SetPingTimeout 设置Ping超时时间
func (r *RedisChecker) SetPingTimeout(timeout time.Duration) {
	r.pingTimeout = timeout
}

// UpdateSettings 更新检查设置
func (r *RedisChecker) UpdateSettings(settings RedisCheckSettings) {
	r.checkSettings = settings
}
