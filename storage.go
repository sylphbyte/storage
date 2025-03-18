package storage

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/sylphbyte/pr"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// MysqlConfig MySQL配置结构体
type MysqlConfig struct {
	Debug       bool   `yaml:"debug"`
	LogMode     int    `yaml:"log_mode"`
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	Database    string `yaml:"database"`
	Charset     string `yaml:"charset"`
	MaxIdleConn int    `yaml:"max_idle_conn"`
	MaxOpenConn int    `yaml:"max_open_conn"`
	MaxLifeTime int    `yaml:"max_life_time"`
}

// RedisConfig Redis配置结构体
type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	Database int    `yaml:"database"`
}

// ESConfig Elasticsearch配置结构体
type ESConfig struct {
	Addresses    []string `yaml:"addresses"`     // ES服务器地址，如 ["http://localhost:9200"]
	Username     string   `yaml:"username"`      // 用户名
	Password     string   `yaml:"password"`      // 密码
	CloudID      string   `yaml:"cloud_id"`      // 云ID，用于Elastic Cloud
	APIKey       string   `yaml:"api_key"`       // API密钥
	EnableHTTPS  bool     `yaml:"enable_https"`  // 是否启用HTTPS
	SkipVerify   bool     `yaml:"skip_verify"`   // 是否跳过证书验证
	MaxRetries   int      `yaml:"max_retries"`   // 最大重试次数
	RetryTimeout int      `yaml:"retry_timeout"` // 重试超时时间(秒)
}

// StorageConfigs 存储配置结构体
type StorageConfigs struct {
	MysqlGroup map[string]MysqlConfig `yaml:"mysql_group"`
	RedisGroup map[string]RedisConfig `yaml:"redis_group"`
	ESGroup    map[string]ESConfig    `yaml:"es_group"`
}

// InitializeStorage 初始化存储服务
func InitializeStorage(configPath string, enabledMysql, enabledRedis map[string]bool, enabledES map[string]bool) (*StorageManagerImpl, error) {
	pr.System("初始化存储服务，配置文件: %s", configPath)

	// 直接使用viper解析配置
	v := viper.New()
	v.SetConfigFile(configPath)

	if err := v.ReadInConfig(); err != nil {
		return nil, errors.Wrapf(err, "读取配置文件失败: %s", configPath)
	}

	// 创建存储管理器
	storageManager := NewStorageManager()

	// 初始化MySQL连接
	mysqlConfigs := v.GetStringMap("mysql_group")
	for name, _ := range mysqlConfigs {
		// 检查是否启用
		if enabled, exists := enabledMysql[name]; !exists || !enabled {
			pr.Warning("MySQL数据库 %s 未启用", name)
			continue
		}

		// 获取MySQL配置
		mysqlConfig := MysqlConfig{
			Debug:       v.GetBool(fmt.Sprintf("mysql_group.%s.debug", name)),
			LogMode:     v.GetInt(fmt.Sprintf("mysql_group.%s.log_mode", name)),
			Host:        v.GetString(fmt.Sprintf("mysql_group.%s.host", name)),
			Port:        v.GetInt(fmt.Sprintf("mysql_group.%s.port", name)),
			Username:    v.GetString(fmt.Sprintf("mysql_group.%s.username", name)),
			Password:    v.GetString(fmt.Sprintf("mysql_group.%s.password", name)),
			Database:    v.GetString(fmt.Sprintf("mysql_group.%s.database", name)),
			Charset:     v.GetString(fmt.Sprintf("mysql_group.%s.charset", name)),
			MaxIdleConn: v.GetInt(fmt.Sprintf("mysql_group.%s.max_idle_conn", name)),
			MaxOpenConn: v.GetInt(fmt.Sprintf("mysql_group.%s.max_open_conn", name)),
			MaxLifeTime: v.GetInt(fmt.Sprintf("mysql_group.%s.max_life_time", name)),
		}

		// 初始化MySQL连接
		db, err := InitMysql(mysqlConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "初始化MySQL连接失败: %s", name)
		}

		// 注册到存储管理器
		storageManager.RegisterDB(name, db)
		pr.System("MySQL数据库 %s 已注册到存储管理器", name)
	}

	// 初始化Redis连接
	redisConfigs := v.GetStringMap("redis_group")
	for name, _ := range redisConfigs {
		// 检查是否启用
		if enabled, exists := enabledRedis[name]; !exists || !enabled {
			pr.Warning("Redis %s 未启用", name)
			continue
		}

		// 获取Redis配置
		redisConfig := RedisConfig{
			Host:     v.GetString(fmt.Sprintf("redis_group.%s.host", name)),
			Port:     v.GetInt(fmt.Sprintf("redis_group.%s.port", name)),
			Password: v.GetString(fmt.Sprintf("redis_group.%s.password", name)),
			Database: v.GetInt(fmt.Sprintf("redis_group.%s.database", name)),
		}

		// 初始化Redis连接
		redis, err := InitRedis(redisConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "初始化Redis连接失败: %s", name)
		}

		// 注册到存储管理器
		storageManager.RegisterRedis(name, redis)
		pr.System("Redis %s 已注册到存储管理器", name)
	}

	// 初始化Elasticsearch连接
	esConfigs := v.GetStringMap("es_group")
	for name, _ := range esConfigs {
		// 检查是否启用
		if enabled, exists := enabledES[name]; !exists || !enabled {
			pr.Warning("Elasticsearch %s 未启用", name)
			continue
		}

		// 获取ES配置
		var addresses []string
		if addrs := v.GetStringSlice(fmt.Sprintf("es_group.%s.addresses", name)); len(addrs) > 0 {
			addresses = addrs
		} else {
			// 兼容单个地址的配置
			if addr := v.GetString(fmt.Sprintf("es_group.%s.address", name)); addr != "" {
				addresses = []string{addr}
			}
		}

		esConfig := ESConfig{
			Addresses:    addresses,
			Username:     v.GetString(fmt.Sprintf("es_group.%s.username", name)),
			Password:     v.GetString(fmt.Sprintf("es_group.%s.password", name)),
			CloudID:      v.GetString(fmt.Sprintf("es_group.%s.cloud_id", name)),
			APIKey:       v.GetString(fmt.Sprintf("es_group.%s.api_key", name)),
			EnableHTTPS:  v.GetBool(fmt.Sprintf("es_group.%s.enable_https", name)),
			SkipVerify:   v.GetBool(fmt.Sprintf("es_group.%s.skip_verify", name)),
			MaxRetries:   v.GetInt(fmt.Sprintf("es_group.%s.max_retries", name)),
			RetryTimeout: v.GetInt(fmt.Sprintf("es_group.%s.retry_timeout", name)),
		}

		// 初始化ES连接
		es, err := InitES(esConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "初始化Elasticsearch连接失败: %s", name)
		}

		// 注册到存储管理器
		storageManager.RegisterES(name, es)
		pr.System("Elasticsearch %s 已注册到存储管理器", name)
	}

	return storageManager, nil
}

// InitMysql 初始化MySQL连接
func InitMysql(config MysqlConfig) (*gorm.DB, error) {
	pr.System("初始化MySQL连接: %s@%s:%d/%s", config.Username, config.Host, config.Port, config.Database)

	// 构建DSN
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		config.Username, config.Password, config.Host, config.Port, config.Database, config.Charset)

	// 日志级别设置
	var logLevel logger.LogLevel
	switch config.LogMode {
	case 1:
		logLevel = logger.Silent
	case 2:
		logLevel = logger.Error
	case 3:
		logLevel = logger.Warn
	case 4:
		logLevel = logger.Info
	default:
		logLevel = logger.Error
	}

	// GORM配置
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true, // 使用单数表名
		},
	}

	// 连接数据库
	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "连接MySQL数据库失败: %s", dsn)
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, errors.Wrap(err, "获取底层SQL DB失败")
	}

	// 设置连接池参数
	sqlDB.SetMaxIdleConns(config.MaxIdleConn)
	sqlDB.SetMaxOpenConns(config.MaxOpenConn)
	sqlDB.SetConnMaxLifetime(time.Duration(config.MaxLifeTime) * time.Second)

	pr.System("MySQL连接初始化成功: %s", config.Database)
	return db, nil
}

// InitRedis 初始化Redis连接
func InitRedis(config RedisConfig) (*redis.Client, error) {
	pr.System("初始化Redis连接: %s:%d/%d", config.Host, config.Port, config.Database)

	// 创建Redis客户端
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password: config.Password,
		DB:       config.Database,
	})

	// 测试连接
	ctx := context.Background()
	if _, err := redisClient.Ping(ctx).Result(); err != nil {
		return nil, errors.Wrapf(err, "连接Redis失败: %s:%d", config.Host, config.Port)
	}

	pr.System("Redis连接初始化成功: %s:%d/%d", config.Host, config.Port, config.Database)
	return redisClient, nil
}

// InitES 初始化Elasticsearch连接
func InitES(config ESConfig) (*elasticsearch.Client, error) {
	// 打印连接信息
	addressesStr := strings.Join(config.Addresses, ", ")
	pr.System("初始化Elasticsearch连接: %s", addressesStr)

	// 创建ES配置
	esConfig := elasticsearch.Config{
		Addresses: config.Addresses,
	}

	// 设置基本认证
	if config.Username != "" && config.Password != "" {
		esConfig.Username = config.Username
		esConfig.Password = config.Password
	}

	// 设置Cloud ID
	if config.CloudID != "" {
		esConfig.CloudID = config.CloudID
	}

	// 设置API Key
	if config.APIKey != "" {
		esConfig.APIKey = config.APIKey
	}

	// 配置HTTP传输
	if config.EnableHTTPS || config.SkipVerify {
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: config.SkipVerify,
			},
		}
		esConfig.Transport = transport
	}

	// 设置重试选项
	if config.MaxRetries > 0 {
		esConfig.MaxRetries = config.MaxRetries
	}

	if config.RetryTimeout > 0 {
		esConfig.RetryBackoff = func(attempt int) time.Duration {
			return time.Duration(attempt*config.RetryTimeout) * time.Second
		}
	}

	// 创建ES客户端
	client, err := elasticsearch.NewClient(esConfig)
	if err != nil {
		return nil, errors.Wrap(err, "创建Elasticsearch客户端失败")
	}

	// 测试连接
	info, err := client.Info()
	if err != nil {
		return nil, errors.Wrap(err, "连接Elasticsearch失败")
	}
	defer info.Body.Close()

	pr.System("Elasticsearch连接初始化成功: %s", addressesStr)
	return client, nil
}
