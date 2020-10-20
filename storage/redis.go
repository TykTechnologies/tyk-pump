package storage

import (
	"context"
	"crypto/tls"
	"strconv"
	"strings"
	"time"

	"github.com/TykTechnologies/logrus"
	"github.com/go-redis/redis/v8"

	"github.com/kelseyhightower/envconfig"
	"github.com/mitchellh/mapstructure"
)

// ------------------- REDIS CLUSTER STORAGE MANAGER -------------------------------

var redisClusterSingleton redis.UniversalClient
var redisLogPrefix = "redis"
var ENV_REDIS_PREFIX = "TYK_PMP_REDIS"
var ctx = context.Background()

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
	Hosts                      EnvMapString `mapstructure:"hosts"` // Deprecated: Use Addrs instead.
	Addrs                      []string     `mapstructure:"addrs"`
	MasterName                 string       `mapstructure:"master_name" json:"master_name"`
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
	db        redis.UniversalClient
	KeyPrefix string
	HashKeys  bool
	Config    RedisStorageConfig
}

func NewRedisClusterPool(forceReconnect bool, config RedisStorageConfig) redis.UniversalClient {
	if !forceReconnect {
		if redisClusterSingleton != nil {
			log.WithFields(logrus.Fields{
				"prefix": redisLogPrefix,
			}).Debug("Redis pool already INITIALISED")
			return redisClusterSingleton
		}
	} else {
		if redisClusterSingleton != nil {
			redisClusterSingleton.Close()
		}
	}

	log.WithFields(logrus.Fields{
		"prefix": redisLogPrefix,
	}).Debug("Creating new Redis connection pool")

	maxActive := 500
	if config.MaxActive > 0 {
		maxActive = config.MaxActive
	}

	timeout := 5 * time.Second

	if config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}

	var tlsConfig *tls.Config
	if config.RedisUseSSL {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: config.RedisSSLInsecureSkipVerify,
		}
	}

	var client redis.UniversalClient
	opts := &redis.UniversalOptions{
		MasterName:   config.MasterName,
		Addrs:        getRedisAddrs(config),
		DB:           config.Database,
		Password:     config.Password,
		PoolSize:     maxActive,
		IdleTimeout:  240 * time.Second,
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
		DialTimeout:  timeout,
		TLSConfig:    tlsConfig,
	}

	if opts.MasterName != "" {
		log.Info("--> [REDIS] Creating sentinel-backed failover client")
		client = redis.NewFailoverClient(opts.Failover())
	} else if config.EnableCluster {
		log.Info("--> [REDIS] Creating cluster client")
		client = redis.NewClusterClient(opts.Cluster())
	} else {
		log.Info("--> [REDIS] Creating single-node client")
		client = redis.NewClient(opts.Simple())
	}

	redisClusterSingleton = client

	return client
}

func getRedisAddrs(config RedisStorageConfig) (addrs []string) {
	if len(config.Addrs) != 0 {
		addrs = config.Addrs
	} else {
		for h, p := range config.Hosts {
			addr := h + ":" + p
			addrs = append(addrs, addr)
		}
	}

	if len(addrs) == 0 && config.Port != 0 {
		addr := config.Host + ":" + strconv.Itoa(config.Port)
		addrs = append(addrs, addr)
	}

	return addrs
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

	var lrange *redis.StringSliceCmd
	_, err := r.db.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		lrange = pipe.LRange(ctx, fixedKey, 0, -1)
		pipe.Del(ctx, fixedKey)

		return nil
	})

	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": redisLogPrefix,
		}).Error("Multi command failed: ", err)
		r.Connect()
	}

	vals := lrange.Val()

	result := make([]interface{}, len(vals))
	for i, v := range vals {
		result[i] = v
	}

	log.WithFields(logrus.Fields{
		"prefix": redisLogPrefix,
	}).Debug("Unpacked vals: ", len(result))

	return result
}

// SetKey will create (or update) a key value in the store
func (r *RedisClusterStorageManager) SetKey(keyName, session string, timeout int64) error {
	log.Debug("[STORE] SET Raw key is: ", keyName)
	log.Debug("[STORE] Setting key: ", r.fixKey(keyName))

	r.ensureConnection()
	err := r.db.Set(ctx, r.fixKey(keyName), session, 0).Err()
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
	err := r.db.Expire(ctx, r.fixKey(keyName), time.Duration(timeout)*time.Second).Err()
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
