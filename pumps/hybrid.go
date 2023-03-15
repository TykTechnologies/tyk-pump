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
	GlobalRPCCallTimeout = 30
	GlobalRPCPingTimeout = 60
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
	// Enable or disable the pump
	Enabled bool `mapstructure:"enabled"`
	// Use SSL to connect to RPC server
	UseSSL bool `mapstructure:"use_ssl"`
	// Skip SSL verification
	SSLInsecureSkipVerify bool `mapstructure:"ssl_insecure_skip_verify"`
	// RPC server connection string
	ConnectionString string `mapstructure:"connection_string"`
	// RPC server key
	RPCKey string `mapstructure:"rpc_key"`
	// RPC server API key
	APIKey string `mapstructure:"api_key"`
	// RPC server call timeout
	CallTimeout int `mapstructure:"call_timeout"`
	// RPC server ping timeout
	PingTimeout int `mapstructure:"ping_timeout"`
	// RPC server connection pool size
	RPCPoolSize int `mapstructure:"rpc_pool_size"`
	// Send aggregated analytics data to RPC server
	Aggregated bool `mapstructure:"aggregated"`
	// Specifies if it should store aggregated data for all the endpoints if `aggregated` is set to `true`. By default, `false`
	// which means that only store aggregated data for `tracked endpoints`.
	TrackAllPaths bool `mapstructure:"track_all_paths"`
	// Determines if the aggregations should be made per minute (true) or per hour (false) if `aggregated` is set to `true`.
	StoreAnalyticsPerMinute bool `json:"store_analytics_per_minute" mapstructure:"store_analytics_per_minute"`
	// Specifies prefixes of tags that should be ignored if `aggregated` is set to `true`.
	IgnoreTagPrefixList []string `json:"ignore_tag_prefix_list" mapstructure:"ignore_tag_prefix_list"`

	aggregationTime int
}

func (conf *HybridPumpConf) CheckDefaults() {
	if conf.CallTimeout == 0 {
		conf.CallTimeout = GlobalRPCCallTimeout
	}

	if conf.PingTimeout == 0 {
		conf.PingTimeout = GlobalRPCPingTimeout
	}

	if conf.Aggregated {
		if conf.StoreAnalyticsPerMinute {
			conf.aggregationTime = 1
		} else {
			conf.aggregationTime = 60
		}
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

	p.log.Info("connecting to MDCB rpc server")
	errConnect := p.connectRpc()
	if errConnect != nil {
		p.log.Fatal("Failed to connect to RPC server")
	}
	p.log.Info("starting rpc dispatcher")
	p.startDispatcher()

	p.log.Info("loging in to MDCB rpc server")
	logged, err := p.RPCLogin()
	if err != nil {
		p.log.Error("Failed to login to RPC server: ", err)
		return err
	} else if !logged {
		p.log.Error("RPC Login incorrect")
		return errors.New("RPC Login incorrect")
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

func (p *HybridPump) connectRpc() error {
	p.log.Info("Setting new RPC connection!")

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
	if p.clientSingleton.Conns == 0 {
		p.clientSingleton.Conns = 20
	}

	p.clientSingleton.Dial = getDialFn(connID, *p.hybridConfig)

	p.clientSingleton.Start()

	return nil
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

func getDialFn(connID string, config HybridPumpConf) func(addr string) (conn net.Conn, err error) {
	return func(addr string) (conn net.Conn, err error) {
		dialer := &net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}

		useSSL := config.UseSSL

		if useSSL {
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

		initWrite := [][]byte{[]byte("proto2"), []byte(len(connID))}, []byte(connID)}

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

	logged, err := p.RPCLogin()
	if err != nil {
		p.log.Error("Failed to login to RPC server: ", err)
		return err
	} else if !logged {
		p.log.Error("RPC Login incorrect")
		return errors.New("RPC Login incorrect")
	}

	// do RPC call to server
	if !p.hybridConfig.Aggregated { // send analytics records as is
		// turn array with analytics records into JSON payload

		p.log.Info("Sending analytics data to MDCB")
		jsonData, err := json.Marshal(data)
		if err != nil {
			p.log.WithError(err).Error("Failed to marshal analytics data")
			return err
		}

		if _, err := p.callRPCFn("PurgeAnalyticsData", string(jsonData)); err != nil {
			p.log.WithError(err).Error("Failed to call PurgeAnalyticsData")
			return err
		}
	} else {
		p.log.Info("Sending aggregated analytics data to MDCB")

		// aggregate analytics records
		aggregates := analytics.AggregateData(data, p.hybridConfig.TrackAllPaths, p.hybridConfig.IgnoreTagPrefixList, p.hybridConfig.ConnectionString, p.hybridConfig.aggregationTime)

		// turn map with analytics aggregates into JSON payload
		jsonData, err := json.Marshal(aggregates)
		if err != nil {
			p.log.WithError(err).Error("Failed to marshal analytics aggregates data")
			return err
		}

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

func (p *HybridPump) RPCLogin() (bool, error) {
	// do RPC call to server
	logged, err := p.callRPCFn("Login", p.hybridConfig.APIKey)
	if err != nil {
		p.log.WithError(err).Error("Failed to call Login")
		return false, err
	}

	return logged.(bool), nil
}
