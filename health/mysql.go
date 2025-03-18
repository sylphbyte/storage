// Package health 提供存储服务的健康检查功能
package health

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
)

// MySQLChecker MySQL健康检查器
type MySQLChecker struct {
	*Checker
	db            *sql.DB            // 数据库连接
	pingTimeout   time.Duration      // ping超时时间
	checkSettings MySQLCheckSettings // 检查设置
}

// MySQLCheckSettings MySQL健康检查设置
type MySQLCheckSettings struct {
	CheckConnections   bool     // 是否检查连接数
	CheckSlowQueries   bool     // 是否检查慢查询
	CheckReplication   bool     // 是否检查复制状态
	CheckVitalTables   bool     // 是否检查关键表
	VitalTables        []string // 关键表列表
	MaxIdleConnPct     float64  // 最大空闲连接百分比 (0.0-1.0)
	MaxOpenConnPct     float64  // 最大打开连接百分比 (0.0-1.0)
	SlowQueryThreshold int      // 慢查询阈值（秒）
}

// DefaultMySQLCheckSettings 创建默认MySQL检查设置
func DefaultMySQLCheckSettings() MySQLCheckSettings {
	return MySQLCheckSettings{
		CheckConnections:   true,
		CheckSlowQueries:   true,
		CheckReplication:   false,
		CheckVitalTables:   false,
		VitalTables:        []string{},
		MaxIdleConnPct:     0.7,
		MaxOpenConnPct:     0.8,
		SlowQueryThreshold: 1,
	}
}

// NewMySQLChecker 创建新的MySQL健康检查器
func NewMySQLChecker(name string, db *sql.DB, settings *MySQLCheckSettings) *MySQLChecker {
	if settings == nil {
		defaultSettings := DefaultMySQLCheckSettings()
		settings = &defaultSettings
	}

	return &MySQLChecker{
		Checker:       NewChecker(name, "mysql"),
		db:            db,
		pingTimeout:   5 * time.Second,
		checkSettings: *settings,
	}
}

// Check 执行MySQL健康检查
func (m *MySQLChecker) Check(ctx context.Context) (HealthStatus, error) {
	startTime := time.Now()

	// 创建健康状态对象
	healthStatus := NewHealthStatus(StatusHealthy)

	// 添加健康检查基本信息
	healthStatus.Details["service_type"] = "mysql"
	healthStatus.Details["check_name"] = m.Name()

	// 创建带超时的上下文
	timeoutCtx, cancel := context.WithTimeout(ctx, m.pingTimeout)
	defer cancel()

	// 基本连接测试
	pingErr := m.db.PingContext(timeoutCtx)
	if pingErr != nil {
		healthStatus.Status = StatusUnhealthy
		healthStatus.Details["ping_error"] = pingErr.Error()
		healthStatus.Latency = time.Since(startTime)

		m.RecordResult(healthStatus, pingErr)
		return healthStatus, errors.Wrap(pingErr, "MySQL ping 失败")
	}

	healthStatus.Details["ping"] = "successful"

	// 连接池健康检查
	if m.checkSettings.CheckConnections {
		err := m.checkConnectionPool(&healthStatus)
		if err != nil {
			pr.Warning("MySQL连接池检查失败: %v", err)
		}
	}

	// 慢查询检查
	if m.checkSettings.CheckSlowQueries {
		err := m.checkSlowQueries(timeoutCtx, &healthStatus)
		if err != nil {
			pr.Warning("MySQL慢查询检查失败: %v", err)
		}
	}

	// 复制状态检查
	if m.checkSettings.CheckReplication {
		err := m.checkReplication(timeoutCtx, &healthStatus)
		if err != nil {
			pr.Warning("MySQL复制状态检查失败: %v", err)
		}
	}

	// 关键表检查
	if m.checkSettings.CheckVitalTables && len(m.checkSettings.VitalTables) > 0 {
		err := m.checkVitalTables(timeoutCtx, &healthStatus)
		if err != nil {
			pr.Warning("MySQL关键表检查失败: %v", err)
		}
	}

	// 设置检查延迟
	healthStatus.Latency = time.Since(startTime)

	// 记录检查结果
	m.RecordResult(healthStatus, nil)

	return healthStatus, nil
}

// checkConnectionPool 检查连接池状态
func (m *MySQLChecker) checkConnectionPool(healthStatus *HealthStatus) error {
	stats := m.db.Stats()

	// 记录连接池统计信息
	healthStatus.Details["open_connections"] = fmt.Sprintf("%d", stats.OpenConnections)
	healthStatus.Details["in_use_connections"] = fmt.Sprintf("%d", stats.InUse)
	healthStatus.Details["idle_connections"] = fmt.Sprintf("%d", stats.Idle)
	healthStatus.Details["max_open_connections"] = fmt.Sprintf("%d", stats.MaxOpenConnections)

	// 检查连接使用率
	if stats.MaxOpenConnections > 0 {
		usagePct := float64(stats.OpenConnections) / float64(stats.MaxOpenConnections)
		healthStatus.Details["connection_usage_pct"] = fmt.Sprintf("%.2f", usagePct*100)

		// 如果连接使用率超过阈值，标记为降级状态
		if usagePct > m.checkSettings.MaxOpenConnPct && healthStatus.Status == StatusHealthy {
			healthStatus.Status = StatusDegraded
			healthStatus.Details["degraded_reason"] = fmt.Sprintf(
				"高连接使用率: %.2f%% > %.2f%%",
				usagePct*100,
				m.checkSettings.MaxOpenConnPct*100,
			)
		}
	}

	// 检查空闲连接率
	if stats.OpenConnections > 0 {
		idlePct := float64(stats.Idle) / float64(stats.OpenConnections)
		healthStatus.Details["idle_connection_pct"] = fmt.Sprintf("%.2f", idlePct*100)

		// 如果空闲连接率超过阈值，标记为降级状态
		if idlePct > m.checkSettings.MaxIdleConnPct && healthStatus.Status == StatusHealthy {
			healthStatus.Status = StatusDegraded
			healthStatus.Details["degraded_reason"] = fmt.Sprintf(
				"高空闲连接率: %.2f%% > %.2f%%",
				idlePct*100,
				m.checkSettings.MaxIdleConnPct*100,
			)
		}
	}

	return nil
}

// checkSlowQueries 检查慢查询
func (m *MySQLChecker) checkSlowQueries(ctx context.Context, healthStatus *HealthStatus) error {
	query := `SELECT COUNT(*) FROM information_schema.processlist 
	          WHERE time > ? AND command != 'Sleep'`

	var slowQueryCount int
	err := m.db.QueryRowContext(ctx, query, m.checkSettings.SlowQueryThreshold).Scan(&slowQueryCount)
	if err != nil {
		healthStatus.Details["slow_queries_check_error"] = err.Error()
		return errors.Wrap(err, "查询慢查询数量失败")
	}

	healthStatus.Details["slow_queries_count"] = fmt.Sprintf("%d", slowQueryCount)

	// 如果存在慢查询，标记为降级状态
	if slowQueryCount > 0 && healthStatus.Status == StatusHealthy {
		healthStatus.Status = StatusDegraded
		healthStatus.Details["degraded_reason"] = fmt.Sprintf("检测到%d个慢查询", slowQueryCount)
	}

	return nil
}

// checkReplication 检查复制状态
func (m *MySQLChecker) checkReplication(ctx context.Context, healthStatus *HealthStatus) error {
	query := "SHOW SLAVE STATUS"

	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		healthStatus.Details["replication_check_error"] = err.Error()
		return errors.Wrap(err, "查询复制状态失败")
	}
	defer rows.Close()

	// 检查是否有返回行，如果没有，说明这个实例不是从库
	if !rows.Next() {
		healthStatus.Details["is_slave"] = "false"
		return nil
	}

	// 是从库，尝试获取复制状态
	healthStatus.Details["is_slave"] = "true"

	// 我们需要根据列名获取值，这里简化处理
	columns, err := rows.Columns()
	if err != nil {
		return errors.Wrap(err, "获取复制状态列名失败")
	}

	// 预先准备好容器
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range columns {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return errors.Wrap(err, "扫描复制状态结果失败")
	}

	// 将结果转换为map
	resultMap := make(map[string]string)
	for i, col := range columns {
		var value string
		switch v := values[i].(type) {
		case []byte:
			value = string(v)
		case int64:
			value = fmt.Sprintf("%d", v)
		case float64:
			value = fmt.Sprintf("%f", v)
		case time.Time:
			value = v.Format(time.RFC3339)
		case nil:
			value = "NULL"
		default:
			value = fmt.Sprintf("%v", v)
		}
		resultMap[col] = value
	}

	// 检查关键指标
	if ioRunning, ok := resultMap["Slave_IO_Running"]; ok {
		healthStatus.Details["slave_io_running"] = ioRunning
		if ioRunning != "Yes" {
			healthStatus.Status = StatusDegraded
			healthStatus.Details["degraded_reason"] = "Slave IO线程未运行"
		}
	}

	if sqlRunning, ok := resultMap["Slave_SQL_Running"]; ok {
		healthStatus.Details["slave_sql_running"] = sqlRunning
		if sqlRunning != "Yes" {
			healthStatus.Status = StatusDegraded
			healthStatus.Details["degraded_reason"] = "Slave SQL线程未运行"
		}
	}

	if lastError, ok := resultMap["Last_Error"]; ok && lastError != "" {
		healthStatus.Details["replication_last_error"] = lastError
		healthStatus.Status = StatusDegraded
		healthStatus.Details["degraded_reason"] = "复制有错误: " + lastError
	}

	if secondsBehind, ok := resultMap["Seconds_Behind_Master"]; ok {
		healthStatus.Details["seconds_behind_master"] = secondsBehind
		if secondsBehind != "0" && secondsBehind != "NULL" {
			healthStatus.Status = StatusDegraded
			healthStatus.Details["degraded_reason"] = "复制延迟: " + secondsBehind + " 秒"
		}
	}

	return nil
}

// checkVitalTables 检查关键表
func (m *MySQLChecker) checkVitalTables(ctx context.Context, healthStatus *HealthStatus) error {
	for _, table := range m.checkSettings.VitalTables {
		// 简单测试表是否可访问
		query := fmt.Sprintf("SELECT 1 FROM %s LIMIT 1", table)
		_, err := m.db.QueryContext(ctx, query)

		if err != nil {
			healthStatus.Status = StatusDegraded
			healthStatus.Details["degraded_reason"] = fmt.Sprintf("无法访问关键表: %s", table)
			healthStatus.Details[fmt.Sprintf("table_%s_error", table)] = err.Error()
			return errors.Wrapf(err, "无法访问关键表: %s", table)
		}

		healthStatus.Details[fmt.Sprintf("table_%s", table)] = "accessible"
	}

	return nil
}

// SetPingTimeout 设置Ping超时时间
func (m *MySQLChecker) SetPingTimeout(timeout time.Duration) {
	m.pingTimeout = timeout
}

// UpdateSettings 更新检查设置
func (m *MySQLChecker) UpdateSettings(settings MySQLCheckSettings) {
	m.checkSettings = settings
}
