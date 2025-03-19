package storage

import (
	"sync"

	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
	"github.com/sylphbyte/sylph"
)

// ManagerImpl 存储管理器实现
type ManagerImpl struct {
	// 存储服务映射
	dbMap    map[string]DBStorage
	redisMap map[string]RedisStorage
	esMap    map[string]ESStorage

	// 默认存储
	defaultDB    string
	defaultRedis string
	defaultES    string

	// 并发控制
	mutex sync.RWMutex
}

// NewStorageManager 创建存储管理器
func NewStorageManager() *ManagerImpl {
	return &ManagerImpl{
		dbMap:    make(map[string]DBStorage),
		redisMap: make(map[string]RedisStorage),
		esMap:    make(map[string]ESStorage),
	}
}

// GetDB 获取数据库存储
func (sm *ManagerImpl) GetDB(name ...string) (DBStorage, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	// 确定要获取的数据库名称
	var dbName string
	if len(name) > 0 && name[0] != "" {
		dbName = name[0]
	} else {
		if sm.defaultDB == "" {
			return nil, errors.New("未设置默认数据库，必须指定数据库名称")
		}
		dbName = sm.defaultDB
	}

	// 查找数据库
	db, exists := sm.dbMap[dbName]
	if !exists {
		return nil, errors.Errorf("数据库 %s 不存在", dbName)
	}

	return db, nil
}

// RegisterDB 注册数据库存储
func (sm *ManagerImpl) RegisterDB(name string, storage DBStorage) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// 检查是否已注册
	if _, exists := sm.dbMap[name]; exists {
		return errors.Errorf("数据库 %s 已经注册", name)
	}

	// 注册数据库
	sm.dbMap[name] = storage

	// 如果是第一个注册的数据库，设为默认
	if sm.defaultDB == "" {
		sm.defaultDB = name
	}

	pr.System("已注册数据库: %s", name)
	return nil
}

// GetRedis 获取Redis存储
func (sm *ManagerImpl) GetRedis(name ...string) (RedisStorage, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	// 确定要获取的Redis名称
	var redisName string
	if len(name) > 0 && name[0] != "" {
		redisName = name[0]
	} else {
		if sm.defaultRedis == "" {
			return nil, errors.New("未设置默认Redis，必须指定Redis名称")
		}
		redisName = sm.defaultRedis
	}

	// 查找Redis
	redis, exists := sm.redisMap[redisName]
	if !exists {
		return nil, errors.Errorf("Redis %s 不存在", redisName)
	}

	return redis, nil
}

// RegisterRedis 注册Redis存储
func (sm *ManagerImpl) RegisterRedis(name string, storage RedisStorage) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// 检查是否已注册
	if _, exists := sm.redisMap[name]; exists {
		return errors.Errorf("Redis %s 已经注册", name)
	}

	// 注册Redis
	sm.redisMap[name] = storage

	// 如果是第一个注册的Redis，设为默认
	if sm.defaultRedis == "" {
		sm.defaultRedis = name
	}

	pr.System("已注册Redis: %s", name)
	return nil
}

// GetES 获取ES存储
func (sm *ManagerImpl) GetES(name ...string) (ESStorage, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	// 确定要获取的ES名称
	var esName string
	if len(name) > 0 && name[0] != "" {
		esName = name[0]
	} else {
		if sm.defaultES == "" {
			return nil, errors.New("未设置默认ES，必须指定ES名称")
		}
		esName = sm.defaultES
	}

	// 查找ES
	es, exists := sm.esMap[esName]
	if !exists {
		return nil, errors.Errorf("ES %s 不存在", esName)
	}

	return es, nil
}

// RegisterES 注册ES存储
func (sm *ManagerImpl) RegisterES(name string, storage ESStorage) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// 检查是否已注册
	if _, exists := sm.esMap[name]; exists {
		return errors.Errorf("ES %s 已经注册", name)
	}

	// 注册ES
	sm.esMap[name] = storage

	// 如果是第一个注册的ES，设为默认
	if sm.defaultES == "" {
		sm.defaultES = name
	}

	pr.System("已注册Elasticsearch: %s", name)
	return nil
}

// GetAllStorages 获取所有存储
func (sm *ManagerImpl) GetAllStorages() map[string]Storage {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	result := make(map[string]Storage)

	// 添加DB存储
	for name, db := range sm.dbMap {
		result[name] = db
	}

	// 添加Redis存储
	for name, redis := range sm.redisMap {
		result[name] = redis
	}

	// 添加ES存储
	for name, es := range sm.esMap {
		result[name] = es
	}

	return result
}

// HealthCheck 健康检查
func (sm *ManagerImpl) HealthCheck(ctx sylph.Context) map[string]*HealthStatus {
	result := make(map[string]*HealthStatus)

	// 获取所有存储
	storages := sm.GetAllStorages()

	// 对每个存储进行健康检查
	for name, storage := range storages {
		// 获取当前健康状态
		status := storage.GetHealthStatus()

		// 尝试Ping
		err := storage.Ping(ctx)
		if err != nil {
			if status.State != StorageStateError {
				status.State = StorageStateError
				status.ErrorMsg = err.Error()
				status.FailCount++
				pr.Warning("存储 %s 健康检查失败: %v", name, err)
			}
		} else {
			status.State = StorageStateConnected
			status.FailCount = 0
			status.ErrorMsg = ""
		}

		result[name] = status
	}

	return result
}

// CloseAll 关闭所有存储连接
func (sm *ManagerImpl) CloseAll(ctx sylph.Context) error {
	var errs []error

	// 获取所有存储
	storages := sm.GetAllStorages()

	// 关闭每个存储连接
	for name, storage := range storages {
		if err := storage.Disconnect(ctx); err != nil {
			errs = append(errs, errors.Wrapf(err, "关闭存储 %s 失败", name))
			pr.Warning("关闭存储 %s 失败: %v", name, err)
		} else {
			pr.System("已关闭存储: %s", name)
		}
	}

	if len(errs) > 0 {
		return errors.Errorf("关闭存储时发生 %d 个错误", len(errs))
	}

	return nil
}
