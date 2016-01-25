package storage

import (
	"github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
	"github.com/lonelycode/redigocluster/rediscluster"
	"github.com/mitchellh/mapstructure"
	"strconv"
	"time"
)

// ------------------- REDIS CLUSTER STORAGE MANAGER -------------------------------

var redisClusterSingleton *rediscluster.RedisCluster
var redisLogPrefix = "redis"

type RedisStorageConfig struct {
	Type          string            `json:"type",mapstructure:"type"`
	Host          string            `json:"host",mapstructure:"host"`
	Port          int               `json:"port",mapstructure:"port"`
	Hosts         map[string]string `json:"hosts",mapstructure:"hosts"`
	Username      string            `json:"username",mapstructure:"username"`
	Password      string            `json:"password",mapstructure:"password"`
	Database      int               `json:"database",mapstructure:"database"`
	MaxIdle       int               `json:"optimisation_max_idle",mapstructure:"optimisation_max_idle"`
	MaxActive     int               `json:"optimisation_max_active",mapstructure:"optimisation_max_active"`
	EnableCluster bool              `json:"enable_cluster",mapstructure:"enable_cluster"`
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

	if config.EnableCluster {
		log.WithFields(logrus.Fields{
			"prefix": redisLogPrefix,
		}).Info("--> Using clustered mode")
	}

	thisPoolConf := rediscluster.PoolConfig{
		MaxIdle:     maxIdle,
		MaxActive:   maxActive,
		IdleTimeout: 240 * time.Second,
		Database:    config.Database,
		Password:    config.Password,
		IsCluster:   config.EnableCluster,
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

	r.KeyPrefix = RedisKeyPrefix
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
		r.GetAndDeleteSet(keyName)
	} else {
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
	return []interface{}{}
}
