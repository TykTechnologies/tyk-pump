package storage

import (
	"strings"
	"time"

	"github.com/TykTechnologies/tyk-pump/logger"
)

type AnalyticsStorage interface {
	Init() error
	GetName() string
	GetAndDeleteSet(setName string, chunkSize int64, expire time.Duration) ([]interface{}, error)
}

const (
	KeyPrefix               string = "analytics-"
	ANALYTICS_KEYNAME       string = "tyk-system-analytics"
	UptimeAnalytics_KEYNAME string = "tyk-uptime-analytics"
)

var log = logger.GetLogger()

// nolint:govet
type TemporalStorageConfig struct {
	// Deprecated.
	Type string `json:"type" mapstructure:"type"`
	// Host value. For example: "localhost".
	Host string `json:"host" mapstructure:"host"`
	// Port value. For example: 6379.
	Port int `json:"port" mapstructure:"port"`
	// Deprecated: use Addrs instead.
	Hosts EnvMapString `json:"hosts" mapstructure:"hosts"`
	// Use instead of the host value if you're running a cluster instance with multiple instances.
	Addrs []string `json:"addrs" mapstructure:"addrs"`
	// Sentinel master name.
	MasterName string `json:"master_name" mapstructure:"master_name"`
	// Sentinel password.
	SentinelPassword string `json:"sentinel_password" mapstructure:"sentinel_password"`
	// DB username.
	Username string `json:"username" mapstructure:"username"`
	// DB password.
	Password string `json:"password" mapstructure:"password"`
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

	// Prefix the key names. Defaults to "analytics-".
	// Deprecated: use KeyPrefix instead.
	RedisKeyPrefix string `json:"redis_key_prefix" mapstructure:"redis_key_prefix"`
	// Prefix the key names. Defaults to "analytics-".
	KeyPrefix string `json:"key_prefix" mapstructure:"key_prefix"`

	// Setting this to true to use SSL when connecting to the DB.
	UseSSL bool `json:"use_ssl" mapstructure:"use_ssl"`
	// Set this to `true` to tell Pump to ignore database's cert validation.
	SSLInsecureSkipVerify bool `json:"ssl_insecure_skip_verify" mapstructure:"ssl_insecure_skip_verify"`
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
	// Setting this to true to use SSL when connecting to the DB.
	// Deprecated: use UseSSL instead.
	RedisUseSSL bool `json:"redis_use_ssl" mapstructure:"redis_use_ssl"`
	// Set this to `true` to tell Pump to ignore database's cert validation.
	// Deprecated: use SSLInsecureSkipVerify instead.
	RedisSSLInsecureSkipVerify bool `json:"redis_ssl_insecure_skip_verify" mapstructure:"redis_ssl_insecure_skip_verify"`
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
