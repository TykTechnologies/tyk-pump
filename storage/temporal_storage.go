package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/TykTechnologies/storage/temporal/connector"
	keyvalue "github.com/TykTechnologies/storage/temporal/keyvalue"
	"github.com/TykTechnologies/storage/temporal/list"
	"github.com/TykTechnologies/storage/temporal/model"
	"github.com/TykTechnologies/tyk-pump/retry"

	"github.com/cenkalti/backoff/v4"

	"github.com/sirupsen/logrus"

	"github.com/kelseyhightower/envconfig"
	"github.com/mitchellh/mapstructure"
)

var (
	connectorSingleton model.Connector
	logPrefix          = "temporal-storage"
	// Deprecated: use envTemporalStoragePrefix instead.
	envRedisPrefix           = "TYK_PMP_REDIS"
	envTemporalStoragePrefix = "TYK_PMP_TEMPORAL_STORAGE"
	ctx                      = context.Background()
)

// TemporalStorageHandler is a storage manager that uses non data-persistent databases, like Redis.
type TemporalStorageHandler struct {
	Config         *TemporalStorageConfig
	kv             model.KeyValue
	list           model.List
	forceReconnect bool
}

func NewTemporalStorageHandler(config interface{}, forceReconnect bool) (*TemporalStorageHandler, error) {
	r := &TemporalStorageHandler{
		forceReconnect: forceReconnect,
	}

	switch c := config.(type) {
	case map[string]interface{}:
		err := mapstructure.Decode(config, &r.Config)
		if err != nil {
			return nil, err
		}

		return r, nil

	case *TemporalStorageConfig:
		r.Config = c

		return r, nil

	case TemporalStorageConfig:
		r.Config = &c

		return r, nil

	default:
		return nil, fmt.Errorf("unsupported config type: %T", config)
	}
}

func (r *TemporalStorageHandler) Init() error {
	if r.Config == nil {
		r.Config = &TemporalStorageConfig{}
		log.WithFields(logrus.Fields{
			"prefix": logPrefix,
		}).Debug("Config is nil, using default config")
	}

	overrideErr := envconfig.Process(envRedisPrefix, r.Config)
	if overrideErr != nil {
		return overrideErr
	}

	overrideErr = envconfig.Process(envTemporalStoragePrefix, r.Config)
	if overrideErr != nil {
		return overrideErr
	}

	switch {
	case r.Config.KeyPrefix != "":
		// Keep the KeyPrefix as is
	case r.Config.RedisKeyPrefix != "":
		r.Config.KeyPrefix = r.Config.RedisKeyPrefix
	default:
		r.Config.KeyPrefix = KeyPrefix
	}

	if r.Config.Type != "" {
		logPrefix = r.Config.Type
	}

	return r.connect()
}

// Connect will establish a connection to the r.db
func (r *TemporalStorageHandler) connect() error {
	var err error
	if connectorSingleton == nil || r.forceReconnect {
		log.WithFields(logrus.Fields{
			"prefix": logPrefix,
		}).Debug("Connecting to temporal storage")
		if r.Config.Type != "redis" && r.Config.Type != "" {
			return fmt.Errorf("unsupported database type: %s", r.Config.Type)
		}

		if err = r.resetConnection(r.Config); err != nil {
			return err
		}

		log.WithFields(logrus.Fields{"prefix": logPrefix}).Debug("Temporal Storage already INITIALISED")
	} else if r.kv == nil || r.list == nil {
		// This is the case when the connector is already created but we're instantiating a new TemporalStorageHandler
		r.kv, err = getKVFromConnector()
		if err != nil {
			return err
		}

		r.list, err = getListFromConnector()
		if err != nil {
			return err
		}
	}

	log.WithFields(logrus.Fields{
		"prefix": logPrefix,
	}).Debug("Storage Engine already initialized...")

	return nil
}

func (r *TemporalStorageHandler) resetConnection(config *TemporalStorageConfig) error {
	if connectorSingleton != nil {
		if err := connectorSingleton.Disconnect(ctx); err != nil {
			return fmt.Errorf("error disconnecting Temporal Storage: %s", err)
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

	tlsOptions := &model.TLS{
		Enable:             config.UseSSL || config.RedisUseSSL,
		InsecureSkipVerify: config.SSLInsecureSkipVerify || config.RedisSSLInsecureSkipVerify,
		CAFile:             config.SSLCAFile,
		CertFile:           config.SSLCertFile,
		KeyFile:            config.SSLKeyFile,
		MaxVersion:         config.SSLMaxVersion,
		MinVersion:         config.SSLMinVersion,
	}

	conn, kv, list, err := createConnector(opts, tlsOptions)
	if err != nil {
		return err
	}

	connectorSingleton = conn
	r.kv = kv
	r.list = list

	return nil
}

func createConnector(opts *model.RedisOptions, tlsOptions *model.TLS) (model.Connector, model.KeyValue, model.List, error) {
	conn, err := connector.NewConnector(model.RedisV9Type, model.WithRedisConfig(opts), model.WithTLS(tlsOptions))
	if err != nil {
		return nil, nil, nil, err
	}

	kv, err := keyvalue.NewKeyValue(conn)
	if err != nil {
		return nil, nil, nil, err
	}

	l, err := list.NewList(conn)
	if err != nil {
		return nil, nil, nil, err
	}

	return conn, kv, l, nil
}

func getKVFromConnector() (model.KeyValue, error) {
	kv, err := keyvalue.NewKeyValue(connectorSingleton)
	if err != nil {
		return nil, err
	}

	return kv, nil
}

func getListFromConnector() (model.List, error) {
	l, err := list.NewList(connectorSingleton)
	if err != nil {
		return nil, err
	}

	return l, nil
}

func (r *TemporalStorageHandler) GetName() string {
	if r.Config.Type != "" {
		return r.Config.Type
	}

	return "redis"
}

func (r *TemporalStorageHandler) fixKey(keyName string) string {
	setKeyName := r.Config.KeyPrefix + keyName

	log.WithFields(logrus.Fields{
		"prefix": logPrefix,
	}).Debug("Input key was: ", setKeyName)

	return setKeyName
}

func (r *TemporalStorageHandler) GetAndDeleteSet(keyName string, chunkSize int64, expire time.Duration) ([]interface{}, error) {
	log.WithFields(logrus.Fields{
		"prefix": logPrefix,
	}).Debug("Getting raw key set: ", keyName)

	err := r.ensureConnection()
	if err != nil {
		return nil, err
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

	result, err := r.list.Pop(ctx, fixedKey, chunkSize)
	if err != nil {
		return nil, err
	}

	if chunkSize != -1 {
		err = r.kv.Expire(ctx, fixedKey, expire)
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

	err := r.ensureConnection()
	if err != nil {
		return err
	}

	err = r.kv.Set(ctx, r.fixKey(keyName), session, time.Duration(timeout)*time.Second)
	if err != nil {
		log.Error("Error trying to set value: ", err)
		return err
	}
	return nil
}

func (r *TemporalStorageHandler) ensureConnection() error {
	if connectorSingleton != nil {
		return nil
	}

	log.Info("Connection dropped, reconnecting...")
	backoffStrategy := retry.GetTemporalStorageExponentialBackoff()

	operation := func() error {
		if err := r.connect(); err != nil {
			return err
		}

		if connectorSingleton == nil {
			return fmt.Errorf("connection failed")
		}
		return nil
	}

	if err := backoff.Retry(operation, backoffStrategy); err != nil {
		return fmt.Errorf("failed to reconnect after several attempts: %w", err)
	}

	return nil
}
