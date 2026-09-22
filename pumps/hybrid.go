package pumps

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net"
	"sync"
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
		"PurgeAnalyticsDataMCPAggregated": func(data string) error {
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

	// connectMu serialises connecting, reconnecting and shutting down. A WriteData call that
	// outlives the configured pump timeout is abandoned and started again on the next purge
	// cycle, so two recoveries can be in flight at once; without this lock each would build a
	// client and could stop the other's brand-new one.
	//
	// Lock order is connectMu then clientMu, never the reverse. clientMu is only ever held to
	// assign or read the pointers below, never across an RPC.
	connectMu sync.Mutex

	// clientMu guards clientSingleton and funcClientSingleton, which callRPCFn reads from the
	// WriteData goroutines while a reconnect may be replacing them.
	clientMu sync.RWMutex

	// clientGen counts published clients, so a recovery that waited for connectMu can tell
	// whether the client whose failure sent it there has already been replaced.
	clientGen atomic.Uint64

	clientSingleton   *gorpc.Client
	dispatcher        *gorpc.Dispatcher
	clientIsConnected atomic.Value

	funcClientSingleton *gorpc.DispatcherClient

	hybridConfig *HybridPumpConf
}

// @PumpConf Hybrid
type HybridPumpConf struct {
	// The prefix for the environment variables that will be used to override the configuration.
	// Defaults to `TYK_PMP_PUMPS_HYBRID_META`
	EnvPrefix string `mapstructure:"meta_env_prefix"`

	// MDCB URL connection string
	ConnectionString string `mapstructure:"connection_string"`
	// Your organization ID to connect to the MDCB installation.
	RPCKey string `mapstructure:"rpc_key"`
	// This the API key of a user used to authenticate and authorize the Hybrid Pump access through MDCB.
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
	// Controls whether MCP analytics are aggregated and sent to MDCB via a separate RPC call. If `pumps.hybrid.meta.aggregated` is set to true and `enable_mcp_aggregation` is set to false, MCP analytics are not aggregated and are completely dropped. If `pumps.hybrid.meta.aggregated` is false, this flag is ignored entirely and all analytics (including MCP) are sent as raw, unaggregated data.
	EnableMCPAggregation bool `json:"enable_mcp_aggregation" mapstructure:"enable_mcp_aggregation"`
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

		// The connect may well have succeeded and only the login failed, leaving a running
		// client behind. Nothing shuts down a pump whose Init returned an error, so its pool
		// would stay up for the lifetime of the process.
		p.connectMu.Lock()
		p.stopClient()
		p.connectMu.Unlock()

		return err
	}

	return nil
}

// startDispatcher publishes an already started client along with a dispatcher client bound to
// it. A gorpc.DispatcherClient cannot be pointed at a different gorpc.Client, so both have to be
// rebuilt for every connection.
//
// The dispatcher is built in a local and published as a whole. Assigning it to the pump first
// and filling it in afterwards let two concurrent reconnects register their functions into
// whichever dispatcher happened to win the field, which is a concurrent map write.
//
// The caller must hold connectMu.
func (p *HybridPump) startDispatcher(client *gorpc.Client) {
	dispatcher := gorpc.NewDispatcher()

	for funcName, funcBody := range dispatcherFuncs {
		dispatcher.AddFunc(funcName, funcBody)
	}

	funcClient := dispatcher.NewFuncClient(client)

	p.clientMu.Lock()
	defer p.clientMu.Unlock()

	p.dispatcher = dispatcher
	p.clientSingleton = client
	p.funcClientSingleton = funcClient

	p.clientGen.Add(1)
}

// stopClient stops the current RPC client, if there is one, and clears it.
//
// A client is published only after gorpc.Client.Start() has returned (see connectRPC), so a
// non-nil clientSingleton is always a running client and is safe to stop exactly once - gorpc
// panics both on stopping a client that was never started and on stopping one twice.
//
// The caller must hold connectMu.
func (p *HybridPump) stopClient() {
	p.clientMu.Lock()
	client := p.clientSingleton
	p.clientSingleton = nil
	p.funcClientSingleton = nil
	p.clientMu.Unlock()

	p.clientIsConnected.Store(false)

	if client == nil {
		return
	}

	// Bounded even when MDCB is unreachable: the pooled handlers return as soon as the client's
	// stop channel is closed and do not wait for an outstanding dial.
	client.Stop()
}

// connectRPC replaces the current RPC client with a freshly connected one.
//
// The caller must hold connectMu.
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

	var client *gorpc.Client

	if p.hybridConfig.UseSSL {
		// #nosec G402
		clientCfg := &tls.Config{
			InsecureSkipVerify: p.hybridConfig.SSLInsecureSkipVerify,
		}

		client = gorpc.NewTLSClient(p.hybridConfig.ConnectionString, clientCfg)
	} else {
		client = gorpc.NewTCPClient(p.hybridConfig.ConnectionString)
	}

	if p.log.Level != logrus.DebugLevel {
		client.LogError = gorpc.NilErrorLogger
	}

	client.OnConnect = p.onConnectFunc

	client.Conns = p.hybridConfig.RPCPoolSize

	client.Dial = getDialFn(connID, p.hybridConfig)

	// Release the client being replaced before starting its successor (TT-14423). Every
	// gorpc.Client keeps rpc_pool_size goroutines redialling MDCB until it is stopped, so simply
	// overwriting the pointer abandoned the whole pool - goroutines here, connections and memory
	// on MDCB - on every single reconnect, and nothing ever closed them. Stopping first also
	// keeps the pump within rpc_pool_size connections at every instant, including while the
	// retrying connect below is working through its attempts.
	p.stopClient()

	client.Start()

	// Published only now: until Start() has returned there is nothing to stop, and callers
	// reading the pointer must never see a client that is not running.
	p.startDispatcher(client)

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
	p.clientMu.RLock()
	funcClient := p.funcClientSingleton
	p.clientMu.RUnlock()

	// Nil between a stop and the next successful connect, and for the whole life of a pump whose
	// Init failed. Reporting it beats both dereferencing nil and burning a full call timeout on a
	// client that has been stopped.
	if funcClient == nil {
		return nil, errors.New("not connected to Tyk MDCB")
	}

	return funcClient.CallTimeout(funcName, request, time.Duration(p.hybridConfig.CallTimeout)*time.Second)
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

		// send MCP aggregates (if any MCP records exist)
		if p.hybridConfig.EnableMCPAggregation {
			if err := p.sendMCPAggregates(data); err != nil {
				return err
			}
		}
	}
	p.log.Info("Purged ", len(data), " records...")

	return nil
}

func (p *HybridPump) Shutdown() error {
	p.log.Info("Shutting down...")

	// Waits for a reconnect that is already under way rather than stopping a client from under
	// it. Nil-safe and repeatable, both of which matter: a pump whose Init failed never got a
	// client and is shut down along with the others.
	p.connectMu.Lock()
	p.stopClient()
	p.connectMu.Unlock()

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

// sendMCPAggregates aggregates MCP analytics from data and sends them to MDCB via RPC.
// Returns nil without making an RPC call when there are no MCP records.
func (p *HybridPump) sendMCPAggregates(data []interface{}) error {
	mcpAggregates := analytics.AggregateMCPData(data, p.hybridConfig.ConnectionString, p.hybridConfig.aggregationTime)
	if len(mcpAggregates) == 0 {
		return nil
	}

	mcpJsonData, err := json.Marshal(mcpAggregates)
	if err != nil {
		p.log.WithError(err).Error("Failed to marshal MCP analytics aggregates data")
		return err
	}

	if _, err := p.callRPCFn("PurgeAnalyticsDataMCPAggregated", string(mcpJsonData)); err != nil {
		p.log.WithError(err).Error("Failed to call PurgeAnalyticsDataMCPAggregated")
		return err
	}

	return nil
}

// connectAndLogin connects to RPC server and logs in if retry is true, it will retry with retryAndLog func
func (p *HybridPump) connectAndLogin(retry bool) error {
	// Which client this recovery is trying to replace. Read before queueing on the lock, so it
	// can be compared with whatever is current once the lock is held.
	observedGen := p.clientGen.Load()

	// One recovery at a time, covering the retry loop and the login that follows it. Locking
	// only around a single connect attempt would let two recoveries interleave, each stopping
	// the client the other had just published.
	p.connectMu.Lock()
	defer p.connectMu.Unlock()

	// When several purge cycles overlap they all fail together and all end up here, queued on
	// the lock. Only the first of them needs to reconnect: rebuilding again would stop a client
	// that is already working and that another purge may be part way through writing through,
	// and a purge that fails loses its records for good, because the purge loop deletes them
	// from the temporal store before handing them over.
	if p.clientGen.Load() != observedGen {
		err := p.RPCLogin()
		if err == nil {
			p.log.Debug("Reusing the MDCB connection another purge just established")

			return nil
		}

		// Credentials are wrong rather than the connection being broken. Reconnecting cannot
		// fix that, and doing so would tear down a usable client for nothing.
		if errors.Is(err, ErrRPCLogin) {
			return err
		}
	}

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
