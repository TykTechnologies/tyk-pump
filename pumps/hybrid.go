package pumps

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net"
	"sync/atomic"
	"time"

	"github.com/TykTechnologies/gorpc"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/cenkalti/backoff/v4"
	"github.com/gofrs/uuid"
	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"
)

const hybridPrefix = "hybrid-pump"

var hybridDefaultENV = PUMPS_ENV_PREFIX + "_HYBRID" + PUMPS_ENV_META_PREFIX

var (
	dispatcherFuncs = map[string]interface{}{
		"Login": func(clientAddr, userKey string) bool {
			return false
		},
		"PurgeAnalyticsData": func(data string) error {
			return nil
		},
		"Ping": func() bool {
			return false
		},
		"PurgeAnalyticsDataAggregated": func(data string) error {
			return nil
		},
	}
	DefaultRPCCallTimeout = 10
	ErrRPCLogin           = errors.New("RPC login incorrect")
	retryAndLog           = func(fn func() error, retryMsg string, logger *logrus.Entry) error {
		return backoff.RetryNotify(fn, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 3), func(err error, t time.Duration) {
			if err != nil {
				logger.Error("Failed to connect to Tyk MDCB, retrying")
			}
		})
	}
)

// HybridPump allows to send analytics to MDCB over RPC
type HybridPump struct {
	CommonPumpConfig

	clientSingleton   *gorpc.Client
	dispatcher        *gorpc.Dispatcher
	clientIsConnected atomic.Value

	funcClientSingleton *gorpc.DispatcherClient

	hybridConfig *HybridPumpConf
}

// @PumpConf Hybrid
type HybridPumpConf struct {
	EnvPrefix string `mapstructure:"meta_env_prefix"`

	// MDCB URL connection string
	ConnectionString string `mapstructure:"connection_string"`
	// Your organisation ID to connect to the MDCB installation.
	RPCKey string `mapstructure:"rpc_key"`
	// This the API key of a user used to authenticate and authorise the Hybrid Pump access through MDCB.
	// The user should be a standard Dashboard user with minimal privileges so as to reduce any risk if the user is compromised.
	APIKey string `mapstructure:"api_key"`

	// Specifies prefixes of tags that should be ignored if `aggregated` is set to `true`.
	IgnoreTagPrefixList []string `json:"ignore_tag_prefix_list" mapstructure:"ignore_tag_prefix_list"`

	// Hybrid pump RPC calls timeout in seconds. Defaults to `10` seconds.
	CallTimeout int `mapstructure:"call_timeout"`
	// Hybrid pump connection pool size. Defaults to `5`.
	RPCPoolSize int `mapstructure:"rpc_pool_size"`
	// aggregationTime is to specify the frequency of the aggregation in minutes if `aggregated` is set to `true`.
	aggregationTime int

	// Send aggregated analytics data to Tyk MDCB
	Aggregated bool `mapstructure:"aggregated"`
	// Specifies if it should store aggregated data for all the endpoints if `aggregated` is set to `true`. By default, `false`
	// which means that only store aggregated data for `tracked endpoints`.
	TrackAllPaths bool `mapstructure:"track_all_paths"`
	// Determines if the aggregations should be made per minute (true) or per hour (false) if `aggregated` is set to `true`.
	StoreAnalyticsPerMinute bool `json:"store_analytics_per_minute" mapstructure:"store_analytics_per_minute"`

	// Use SSL to connect to Tyk MDCB
	UseSSL bool `mapstructure:"use_ssl"`
	// Skip SSL verification
	SSLInsecureSkipVerify bool `mapstructure:"ssl_insecure_skip_verify"`
}

func (conf *HybridPumpConf) CheckDefaults() {
	if conf.CallTimeout == 0 {
		conf.CallTimeout = DefaultRPCCallTimeout
	}

	if conf.Aggregated {
		conf.aggregationTime = 60
		if conf.StoreAnalyticsPerMinute {
			conf.aggregationTime = 1
		}
	}

	if conf.RPCPoolSize == 0 {
		conf.RPCPoolSize = 5
	}
}

func (p *HybridPump) GetName() string {
	return "Hybrid pump"
}

func (p *HybridPump) New() Pump {
	return &HybridPump{}
}

func (p *HybridPump) Init(config interface{}) error {
	p.log = log.WithField("prefix", hybridPrefix)

	// Read configuration file
	p.hybridConfig = &HybridPumpConf{}
	err := mapstructure.Decode(config, &p.hybridConfig)
	if err != nil {
		p.log.Error("Failed to decode configuration: ", err)
		return err
	}

	processPumpEnvVars(p, p.log, p.hybridConfig, hybridDefaultENV)

	if p.hybridConfig.ConnectionString == "" {
		p.log.Error("Failed to decode configuration - no connection_string")
		return errors.New("empty connection_string")
	}

	p.hybridConfig.CheckDefaults()

	if err := p.connectAndLogin(true); err != nil {
		p.log.Error(err)
		return err
	}

	return nil
}

func (p *HybridPump) startDispatcher() {
	p.dispatcher = gorpc.NewDispatcher()

	for funcName, funcBody := range dispatcherFuncs {
		p.dispatcher.AddFunc(funcName, funcBody)
	}

	p.funcClientSingleton = p.dispatcher.NewFuncClient(p.clientSingleton)
}

func (p *HybridPump) connectRPC() error {
	p.log.Debug("Setting new MDCB connection!")

	connUUID, err := uuid.NewV4()
	if err != nil {
		return err
	}
	connID := connUUID.String()

	// Length should fit into 1 byte. Protection if we decide change uuid in future.
	if len(connID) > 255 {
		return errors.New("connID is too long")
	}

	if p.hybridConfig.UseSSL {
		// #nosec G402
		clientCfg := &tls.Config{
			InsecureSkipVerify: p.hybridConfig.SSLInsecureSkipVerify,
		}

		p.clientSingleton = gorpc.NewTLSClient(p.hybridConfig.ConnectionString, clientCfg)
	} else {
		p.clientSingleton = gorpc.NewTCPClient(p.hybridConfig.ConnectionString)
	}

	if p.log.Level != logrus.DebugLevel {
		p.clientSingleton.LogError = gorpc.NilErrorLogger
	}

	p.clientSingleton.OnConnect = p.onConnectFunc

	p.clientSingleton.Conns = p.hybridConfig.RPCPoolSize

	p.clientSingleton.Dial = getDialFn(connID, p.hybridConfig)

	p.clientSingleton.Start()

	p.startDispatcher()

	_, err = p.callRPCFn("Ping", nil)

	return err
}

func (p *HybridPump) onConnectFunc(conn net.Conn) (net.Conn, string, error) {
	p.clientIsConnected.Store(true)
	remoteAddr := conn.RemoteAddr().String()
	p.log.WithField("remoteAddr", remoteAddr).Debug("connected to RPC server")

	return conn, remoteAddr, nil
}

func (p *HybridPump) callRPCFn(funcName string, request interface{}) (interface{}, error) {
	return p.funcClientSingleton.CallTimeout(funcName, request, time.Duration(p.hybridConfig.CallTimeout)*time.Second)
}

func getDialFn(connID string, config *HybridPumpConf) func(addr string) (conn net.Conn, err error) {
	return func(addr string) (conn net.Conn, err error) {
		dialer := &net.Dialer{
			Timeout:   time.Duration(config.CallTimeout) * time.Second,
			KeepAlive: 30 * time.Second,
		}

		useSSL := config.UseSSL

		if useSSL {
			// #nosec G402
			cfg := &tls.Config{
				InsecureSkipVerify: config.SSLInsecureSkipVerify,
			}

			conn, err = tls.DialWithDialer(dialer, "tcp", addr, cfg)
		} else {
			conn, err = dialer.Dial("tcp", addr)
		}

		if err != nil {
			return nil, err
		}

		initWrite := [][]byte{[]byte("proto2"), {byte(len(connID))}, []byte(connID)}

		for _, data := range initWrite {
			if _, err := conn.Write(data); err != nil {
				return nil, err
			}
		}

		return conn, nil
	}
}

func (p *HybridPump) WriteData(ctx context.Context, data []interface{}) error {
	if len(data) == 0 {
		return nil
	}
	p.log.Debug("Attempting to write ", len(data), " records...")

	err := p.RPCLogin()
	if err != nil {
		if errors.Is(err, ErrRPCLogin) {
			p.log.Error("Failed to login to Tyk MDCB: ", err)
			return err
		}
		p.log.Error("Failed to connect to Tyk MDCB, retrying")

		// try to login again
		if err = p.connectAndLogin(false); err != nil {
			p.log.Error(err)
			return err
		}
	}

	// do RPC call to server
	if !p.hybridConfig.Aggregated {
		// send analytics records as is
		// turn array with analytics records into JSON payload
		jsonData, err := json.Marshal(data)
		if err != nil {
			p.log.WithError(err).Error("Failed to marshal analytics data")
			return err
		}

		p.log.Debug("Sending analytics data to Tyk MDCB")

		if _, err := p.callRPCFn("PurgeAnalyticsData", string(jsonData)); err != nil {
			p.log.WithError(err).Error("Failed to call PurgeAnalyticsData")
			return err
		}
	} else {
		// aggregate analytics records
		aggregates := analytics.AggregateData(data, p.hybridConfig.TrackAllPaths, p.hybridConfig.IgnoreTagPrefixList, p.hybridConfig.ConnectionString, p.hybridConfig.aggregationTime)

		// turn map with analytics aggregates into JSON payload
		jsonData, err := json.Marshal(aggregates)
		if err != nil {
			p.log.WithError(err).Error("Failed to marshal analytics aggregates data")
			return err
		}

		p.log.Debug("Sending aggregated analytics data to Tyk MDCB")

		// send aggregated data
		if _, err := p.callRPCFn("PurgeAnalyticsDataAggregated", string(jsonData)); err != nil {
			p.log.WithError(err).Error("Failed to call PurgeAnalyticsDataAggregated")
			return err
		}
	}
	p.log.Info("Purged ", len(data), " records...")

	return nil
}

func (p *HybridPump) Shutdown() error {
	p.log.Info("Shutting down...")
	p.clientSingleton.Stop()
	p.clientSingleton = nil
	p.funcClientSingleton = nil

	p.clientIsConnected.Store(false)

	p.log.Info("Pump shut down.")
	return nil
}

func (p *HybridPump) RPCLogin() error {
	if val, ok := p.clientIsConnected.Load().(bool); !ok || !val {
		p.log.Debug("Client is not connected to RPC server")
		return errors.New("client is not connected to RPC server")
	}

	// do RPC call to server
	logged, err := p.callRPCFn("Login", p.hybridConfig.APIKey)
	if err != nil {
		p.log.WithError(err).Error("Failed to call Login")
		return err
	}

	if !logged.(bool) {
		return ErrRPCLogin
	}

	return nil
}

// connectAndLogin connects to RPC server and logs in if retry is true, it will retry with retryAndLog func
func (p *HybridPump) connectAndLogin(retry bool) error {
	connectFn := p.connectRPC
	loginFn := p.RPCLogin

	if retry {
		connectFn = func() error {
			return retryAndLog(p.connectRPC, "Failed to connect to Tyk MDCB, retrying", p.log)
		}

		loginFn = func() error {
			return retryAndLog(p.RPCLogin, "Failed to login to Tyk MDCB, retrying", p.log)
		}
	}

	p.log.Info("Connecting to Tyk MDCB...")
	if err := connectFn(); err != nil {
		return err
	}

	p.log.Info("Logging in to Tyk MDCB...")
	if err := loginFn(); err != nil {
		return err
	}

	return nil
}
