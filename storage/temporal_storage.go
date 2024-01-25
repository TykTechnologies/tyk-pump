package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/TykTechnologies/storage/temporal/connector"
	keyvalue "github.com/TykTechnologies/storage/temporal/keyvalue"
	"github.com/TykTechnologies/storage/temporal/list"
	"github.com/TykTechnologies/storage/temporal/model"

	"github.com/sirupsen/logrus"

	"github.com/kelseyhightower/envconfig"
	"github.com/mitchellh/mapstructure"
)

// ------------------- TEMPORAL CLUSTER STORAGE MANAGER -------------------------------

var (
	temporalStorageSingleton *storageHandler
	logPrefix                = "temporal-storage"
	// Deprecated: use envTemporalStoragePrefix instead.
	envRedisPrefix           = "TYK_PMP_REDIS"
	envTemporalStoragePrefix = "TYK_PMP_TEMPORAL_STORAGE"
	ctx                      = context.Background()
)

type storageHandler struct {
	list list.List
	kv   keyvalue.KeyValue
	conn model.Connector
}

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

type TemporalStorageConfig struct {
	// Type is deprecated.
	Type string `json:"type" mapstructure:"type"`
	// Host value. For example: "localhost".
	Host string `json:"host" mapstructure:"host"`
	// Sentinel master name.
	MasterName string `json:"master_name" mapstructure:"master_name"`
	// Sentinel password.
	SentinelPassword string `json:"sentinel_password" mapstructure:"sentinel_password"`
	// DB username.
	Username string `json:"username" mapstructure:"username"`
	// DB password.
	Password string `json:"password" mapstructure:"password"`
	// Prefix the key names. Defaults to "analytics-".
	// Deprecated: use KeyPrefix instead.
	RedisKeyPrefix string `json:"redis_key_prefix" mapstructure:"redis_key_prefix"`
	// Prefix the key names. Defaults to "analytics-".
	KeyPrefix string `json:"key_prefix" mapstructure:"key_prefix"`
	// Path to the CA file.
	SSLCAFile string `json:"ssl_ca_file" mapstructure:"ssl_ca_file"`
	// Path to the cert file.
	SSLCertFile string `json:"ssl_cert_file" mapstructure:"ssl_cert_file"`
	// Path to the key file.
	SSLKeyFile string `json:"ssl_key_file" mapstructure:"ssl_key_file"`
	// Maximum supported TLS version. Defaults to TLS 1.3, valid values are TLS 1.0, 1.1, 1.2, 1.3.
	SSLMaxVersion string `json:"ssl_max_version" mapstructure:"ssl_max_version"`
	// Minimum supported TLS version. Defaults to TLS 1.2, valid values are TLS 1.0, 1.1, 1.2, 1.3.
	SSLMinVersion string `json:"ssl_min_version" mapstructure:"ssl_min_version"`
	// Deprecated: use Addrs instead.
	Hosts EnvMapString `json:"hosts" mapstructure:"hosts"`
	// Use instead of the host value if you're running a cluster instance with multiple instances.
	Addrs []string `json:"addrs" mapstructure:"addrs"`
	// Port value. For example: 6379.
	Port int `json:"port" mapstructure:"port"`
	// Database name.
	Database int `json:"database" mapstructure:"database"`
	// How long to allow for new connections to be established (in milliseconds). Defaults to 5sec.
	Timeout int `json:"timeout" mapstructure:"timeout"`
	// Maximum number of idle connections in the pool.
	MaxIdle int `json:"optimisation_max_idle" mapstructure:"optimisation_max_idle"`
	// Maximum number of connections allocated by the pool at a given time. When zero, there is no
	// limit on the number of connections in the pool. Defaults to 500.
	MaxActive int `json:"optimisation_max_active" mapstructure:"optimisation_max_active"`

	// Enable this option if you are using a cluster instance. Default is `false`.
	EnableCluster bool `json:"enable_cluster" mapstructure:"enable_cluster"`
	// Setting this to true to use SSL when connecting to the DB.
	// Deprecated: use UseSSL instead.
	RedisUseSSL bool `json:"redis_use_ssl" mapstructure:"redis_use_ssl"`
	// Set this to `true` to tell Pump to ignore database's cert validation.
	// Deprecated: use SSLInsecureSkipVerify instead.
	RedisSSLInsecureSkipVerify bool `json:"redis_ssl_insecure_skip_verify" mapstructure:"redis_ssl_insecure_skip_verify"`
	// Setting this to true to use SSL when connecting to the DB.
	UseSSL bool `json:"use_ssl" mapstructure:"use_ssl"`
	// Set this to `true` to tell Pump to ignore database's cert validation.
	SSLInsecureSkipVerify bool `json:"ssl_insecure_skip_verify" mapstructure:"ssl_insecure_skip_verify"`
}

// TemporalStorageHandler is a storage manager that uses non data-persistent databases, like Redis.
type TemporalStorageHandler struct {
	KeyPrefix string
	db        *storageHandler
	Config    TemporalStorageConfig
	HashKeys  bool
}

func NewTemporalStorageHandler(forceReconnect bool, config *TemporalStorageConfig) error {
	switch config.Type {
	case "redis", "":
		if !forceReconnect {
			if temporalStorageSingleton != nil {
				log.WithFields(logrus.Fields{
					"prefix": logPrefix,
				}).Debug("Redis pool already INITIALISED")
				return nil
			}
		} else {
			if temporalStorageSingleton != nil {
				err := temporalStorageSingleton.conn.Disconnect(ctx)
				if err != nil {
					return fmt.Errorf("error disconnecting Redis: %s", err.Error())
				}
			}
		}

		log.WithFields(logrus.Fields{
			"prefix": logPrefix,
		}).Debug("Creating new Redis connection pool")

		maxActive := 500
		if config.MaxActive > 0 {
			maxActive = config.MaxActive
		}

		timeout := 5

		if config.Timeout > 0 {
			timeout = config.Timeout
		}

		opts := &model.RedisOptions{
			MasterName:       config.MasterName,
			SentinelPassword: config.SentinelPassword,
			Addrs:            config.Addrs,
			Database:         config.Database,
			Username:         config.Username,
			Password:         config.Password,
			MaxActive:        maxActive,
			Timeout:          timeout,
			EnableCluster:    config.EnableCluster,
			Host:             config.Host,
			Port:             config.Port,
			Hosts:            config.Hosts,
		}

		enableTLS := config.UseSSL || config.RedisUseSSL

		insecureSkipVerify := config.SSLInsecureSkipVerify || config.RedisSSLInsecureSkipVerify

		tlsOptions := &model.TLS{
			Enable:             enableTLS,
			InsecureSkipVerify: insecureSkipVerify,
			CAFile:             config.SSLCAFile,
			CertFile:           config.SSLCertFile,
			KeyFile:            config.SSLKeyFile,
			MaxVersion:         config.SSLMaxVersion,
			MinVersion:         config.SSLMinVersion,
		}

		conn, err := connector.NewConnector(model.RedisV9Type, model.WithRedisConfig(opts), model.WithTLS(tlsOptions))
		if err != nil {
			return err
		}

		kv, err := keyvalue.NewKeyValue(conn)
		if err != nil {
			return err
		}

		l, err := list.NewList(conn)
		if err != nil {
			return err
		}

		temporalStorageSingleton = &storageHandler{}
		temporalStorageSingleton.kv = kv
		temporalStorageSingleton.list = l
		temporalStorageSingleton.conn = conn

		return nil
	default:
		return fmt.Errorf("unsupported database type: %s", config.Type)
	}
}

func (r *TemporalStorageHandler) GetName() string {
	if r.Config.Type != "" {
		return r.Config.Type
	}

	return "redis"
}

func (r *TemporalStorageHandler) Init(config interface{}) error {
	r.Config = TemporalStorageConfig{}
	err := mapstructure.Decode(config, &r.Config)
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": logPrefix,
		}).Fatal("Failed to decode configuration: ", err)
	}

	overrideErr := envconfig.Process(envRedisPrefix, &r.Config)
	if overrideErr != nil {
		log.Error("Failed to process environment variables from redis prefix: ", overrideErr)
	}

	overrideErr = envconfig.Process(envTemporalStoragePrefix, &r.Config)
	if overrideErr != nil {
		log.Error("Failed to process environment variables from temporal storage prefix: ", overrideErr)
	}

	switch {
	case r.Config.KeyPrefix != "":
		r.KeyPrefix = r.Config.KeyPrefix
	case r.Config.RedisKeyPrefix != "":
		r.KeyPrefix = r.Config.RedisKeyPrefix
	default:
		r.KeyPrefix = KeyPrefix
	}

	if r.Config.Type != "" {
		logPrefix = r.Config.Type
	}

	return nil
}

// Connect will establish a connection to the r.db
func (r *TemporalStorageHandler) Connect() bool {
	if r.db == nil {
		log.WithFields(logrus.Fields{
			"prefix": logPrefix,
		}).Debug("Connecting to temporal storage")
		err := NewTemporalStorageHandler(false, &r.Config)
		if err != nil {
			log.WithFields(logrus.Fields{
				"prefix": logPrefix,
			}).Error("Error connecting to temporal storage: ", err)
			return false
		}
		r.db = temporalStorageSingleton
		return true
	}

	log.WithFields(logrus.Fields{
		"prefix": logPrefix,
	}).Debug("Storage Engine already initialized...")

	// Reset it just in case
	r.db = temporalStorageSingleton
	return true
}

func (r *TemporalStorageHandler) hashKey(in string) string {
	return in
}

func (r *TemporalStorageHandler) fixKey(keyName string) string {
	setKeyName := r.KeyPrefix + r.hashKey(keyName)

	log.WithFields(logrus.Fields{
		"prefix": logPrefix,
	}).Debug("Input key was: ", setKeyName)

	return setKeyName
}

func (r *TemporalStorageHandler) GetAndDeleteSet(keyName string, chunkSize int64, expire time.Duration) ([]interface{}, error) {
	log.WithFields(logrus.Fields{
		"prefix": logPrefix,
	}).Debug("Getting raw key set: ", keyName)

	if r.db == nil {
		log.WithFields(logrus.Fields{
			"prefix": logPrefix,
		}).Warning("Connection dropped, connecting..")
		r.Connect()
		return r.GetAndDeleteSet(keyName, chunkSize, expire)
	}

	log.WithFields(logrus.Fields{
		"prefix": logPrefix,
	}).Debug("keyName is: ", keyName)

	fixedKey := r.fixKey(keyName)

	log.WithFields(logrus.Fields{
		"prefix": logPrefix,
	}).Debug("Fixed keyname is: ", fixedKey)

	// In Pump, we used to delete a key when chunkSize was 0.
	// This is not the case with Storage Library. So we need to check if chunkSize is 0 and set it to -1.
	if chunkSize == 0 {
		chunkSize = -1
	}

	result, err := r.db.list.Pop(ctx, fixedKey, chunkSize)
	if err != nil {
		return nil, err
	}

	if chunkSize != -1 {
		err = r.db.kv.Expire(ctx, fixedKey, expire)
		if err != nil {
			return nil, err
		}
	}

	intResult := []interface{}{}
	for _, v := range result {
		intResult = append(intResult, v)
	}

	return intResult, nil
}

// SetKey will create (or update) a key value in the store
func (r *TemporalStorageHandler) SetKey(keyName, session string, timeout int64) error {
	log.Debug("[STORE] SET Raw key is: ", keyName)
	log.Debug("[STORE] Setting key: ", r.fixKey(keyName))

	r.ensureConnection()
	err := r.db.kv.Set(ctx, r.fixKey(keyName), session, 0)
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

func (r *TemporalStorageHandler) SetExp(keyName string, timeout int64) error {
	err := r.db.kv.Expire(ctx, r.fixKey(keyName), time.Duration(timeout)*time.Second)
	if err != nil {
		log.Error("Could not EXPIRE key: ", err)
	}
	return err
}

func (r *TemporalStorageHandler) ensureConnection() {
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
