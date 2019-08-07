package storage

import (
	"strconv"
	"strings"
	"time"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/redigocluster/rediscluster"

	"github.com/garyburd/redigo/redis"
	"github.com/kelseyhightower/envconfig"
	"github.com/mitchellh/mapstructure"
)

// ------------------- REDIS CLUSTER STORAGE MANAGER -------------------------------

var redisClusterSingleton *rediscluster.RedisCluster
var redisLogPrefix = "redis"
var ENV_REDIS_PREFIX = "TYK_PMP_REDIS"

type EnvMapString map[string]string

func (e *EnvMapString) Decode(value string) error {
	units := strings.Split(value, ",")
	m := make(map[string]string)
	for _, unit := range units {
		kvArr := strings.Split(unit, ":")
		if len(kvArr) > 1 {
			m[kvArr[0]] = kvArr[1]
		}
	}

	*e = m

	return nil
}

type RedisStorageConfig struct {
	Type                       string       `mapstructure:"type"`
	Host                       string       `mapstructure:"host"`
	Port                       int          `mapstructure:"port"`
	Hosts                      EnvMapString `mapstructure:"hosts"`
	Username                   string       `mapstructure:"username"`
	Password                   string       `mapstructure:"password"`
	Database                   int          `mapstructure:"database"`
	Timeout                    int          `mapstructure:"timeout"`
	MaxIdle                    int          `mapstructure:"optimisation_max_idle" json:"optimisation_max_idle"`
	MaxActive                  int          `mapstructure:"optimisation_max_active" json:"optimisation_max_active"`
	EnableCluster              bool         `mapstructure:"enable_cluster" json:"enable_cluster"`
	RedisKeyPrefix             string       `mapstructure:"redis_key_prefix" json:"redis_key_prefix"`
	RedisUseSSL                bool         `mapstructure:"redis_use_ssl" json:"redis_use_ssl"`
	RedisSSLInsecureSkipVerify bool         `mapstructure:"redis_ssl_insecure_skip_verify" json:"redis_ssl_insecure_skip_verify"`
}

// RedisClusterStorageManager is a storage manager that uses the redis database.
type RedisClusterStorageManager struct {
	db        *rediscluster.RedisCluster
	KeyPrefix string
	HashKeys  bool
	Config    RedisStorageConfig
}

func NewRedisClusterPool(forceReconnect bool, config RedisStorageConfig) *rediscluster.RedisCluster {
	if !forceReconnect {
		if redisClusterSingleton != nil {
			log.WithFields(logrus.Fields{
				"prefix": redisLogPrefix,
			}).Debug("Redis pool already INITIALISED")
			return redisClusterSingleton
		}
	} else {
		if redisClusterSingleton != nil {
			redisClusterSingleton.CloseConnection()
		}
	}

	log.WithFields(logrus.Fields{
		"prefix": redisLogPrefix,
	}).Debug("Creating new Redis connection pool")

	maxIdle := 100
	if config.MaxIdle > 0 {
		maxIdle = config.MaxIdle
	}

	maxActive := 500
	if config.MaxActive > 0 {
		maxActive = config.MaxActive
	}

	timeout := 5 * time.Second

	if config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}

	if config.EnableCluster {
		log.WithFields(logrus.Fields{
			"prefix": redisLogPrefix,
		}).Info("--> Using clustered mode")
	}

	thisPoolConf := rediscluster.PoolConfig{
		MaxIdle:        maxIdle,
		MaxActive:      maxActive,
		IdleTimeout:    240 * time.Second,
		ConnectTimeout: timeout,
		ReadTimeout:    timeout,
		WriteTimeout:   timeout,
		Database:       config.Database,
		Password:       config.Password,
		IsCluster:      config.EnableCluster,
		UseTLS:         config.RedisUseSSL,
		TLSSkipVerify:  config.RedisSSLInsecureSkipVerify,
	}

	seed_redii := []map[string]string{}

	if len(config.Hosts) > 0 {
		for h, p := range config.Hosts {
			seed_redii = append(seed_redii, map[string]string{h: p})
		}
	} else {
		seed_redii = append(seed_redii, map[string]string{config.Host: strconv.Itoa(config.Port)})
	}

	thisInstance := rediscluster.NewRedisCluster(seed_redii, thisPoolConf, false)

	redisClusterSingleton = &thisInstance

	return &thisInstance
}

func (r *RedisClusterStorageManager) GetName() string {
	return "redis"
}

func (r *RedisClusterStorageManager) Init(config interface{}) error {
	r.Config = RedisStorageConfig{}
	err := mapstructure.Decode(config, &r.Config)
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": redisLogPrefix,
		}).Fatal("Failed to decode configuration: ", err)
	}

	overrideErr := envconfig.Process(ENV_REDIS_PREFIX, &r.Config)
	if overrideErr != nil {
		log.Error("Failed to process environment variables for redis: ", overrideErr)
	}

	if r.Config.RedisKeyPrefix == "" {
		r.KeyPrefix = RedisKeyPrefix
	} else {
		r.KeyPrefix = r.Config.RedisKeyPrefix
	}
	return nil
}

// Connect will establish a connection to the r.db
func (r *RedisClusterStorageManager) Connect() bool {
	if r.db == nil {
		log.WithFields(logrus.Fields{
			"prefix": redisLogPrefix,
		}).Debug("Connecting to redis cluster")
		r.db = NewRedisClusterPool(false, r.Config)
		return true
	}

	log.WithFields(logrus.Fields{
		"prefix": redisLogPrefix,
	}).Debug("Storage Engine already initialized...")
	log.WithFields(logrus.Fields{
		"prefix": redisLogPrefix,
	}).Debug("Redis handles: ", len(r.db.Handles))

	// Reset it just in case
	r.db = redisClusterSingleton
	return true
}

func (r *RedisClusterStorageManager) hashKey(in string) string {
	return in
}

func (r *RedisClusterStorageManager) fixKey(keyName string) string {
	setKeyName := r.KeyPrefix + r.hashKey(keyName)

	log.WithFields(logrus.Fields{
		"prefix": redisLogPrefix,
	}).Debug("Input key was: ", setKeyName)

	return setKeyName
}

func (r *RedisClusterStorageManager) GetAndDeleteSet(keyName string) []interface{} {
	log.WithFields(logrus.Fields{
		"prefix": redisLogPrefix,
	}).Debug("Getting raw key set: ", keyName)

	if r.db == nil {
		log.WithFields(logrus.Fields{
			"prefix": redisLogPrefix,
		}).Warning("Connection dropped, connecting..")
		r.Connect()
		return r.GetAndDeleteSet(keyName)
	}

	log.WithFields(logrus.Fields{
		"prefix": redisLogPrefix,
	}).Debug("keyName is: ", keyName)

	fixedKey := r.fixKey(keyName)

	log.WithFields(logrus.Fields{
		"prefix": redisLogPrefix,
	}).Debug("Fixed keyname is: ", fixedKey)

	lrange := rediscluster.ClusterTransaction{}
	lrange.Cmd = "LRANGE"
	lrange.Args = []interface{}{fixedKey, 0, -1}

	delCmd := rediscluster.ClusterTransaction{}
	delCmd.Cmd = "DEL"
	delCmd.Args = []interface{}{fixedKey}

	redVal, err := redis.Values(r.db.DoTransaction([]rediscluster.ClusterTransaction{lrange, delCmd}))
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": redisLogPrefix,
		}).Error("Multi command failed: ", err)
		r.Connect()
	}

	log.WithFields(logrus.Fields{
		"prefix": redisLogPrefix,
	}).Debug("Analytics returned: ", redVal)
	if len(redVal) == 0 {
		return []interface{}{}
	}

	vals := redVal[0].([]interface{})

	log.WithFields(logrus.Fields{
		"prefix": redisLogPrefix,
	}).Debug("Unpacked vals: ", vals)

	return vals
}

// SetKey will create (or update) a key value in the store
func (r *RedisClusterStorageManager) SetKey(keyName, session string, timeout int64) error {
	log.Debug("[STORE] SET Raw key is: ", keyName)
	log.Debug("[STORE] Setting key: ", r.fixKey(keyName))

	r.ensureConnection()
	_, err := r.db.Do("SET", r.fixKey(keyName), session)
	if timeout > 0 {
		if err := r.SetExp(keyName, timeout); err != nil {
			return err
		}
	}
	if err != nil {
		log.Error("Error trying to set value: ", err)
		return err
	}
	return nil
}

func (r *RedisClusterStorageManager) SetExp(keyName string, timeout int64) error {
	_, err := r.db.Do("EXPIRE", r.fixKey(keyName), timeout)
	if err != nil {
		log.Error("Could not EXPIRE key: ", err)
	}
	return err
}

func (r *RedisClusterStorageManager) ensureConnection() {
	if r.db != nil {
		// already connected
		return
	}
	log.Info("Connection dropped, reconnecting...")
	for {
		r.Connect()
		if r.db != nil {
			// reconnection worked
			return
		}
		log.Info("Reconnecting again...")
	}
}
