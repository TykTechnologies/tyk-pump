package pumps

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TykTechnologies/gorpc"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupKeepalive(conn net.Conn) error {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return errors.New("not a tcp connection")
	}

	if err := tcpConn.SetKeepAlive(true); err != nil {
		return err
	}
	if err := tcpConn.SetKeepAlivePeriod(30 * time.Second); err != nil {
		return err
	}
	return nil
}

type testListener struct {
	L net.Listener
}

func (ln *testListener) Init(addr string) (err error) {
	ln.L, err = net.Listen("tcp", addr)
	return
}

func (ln *testListener) Accept() (conn net.Conn, err error) {
	c, err := ln.L.Accept()
	if err != nil {
		return
	}

	if err = setupKeepalive(c); err != nil {
		c.Close()
		return
	}

	if _, err = readConnID(c); err != nil {
		return nil, err
	}

	return c, nil
}

// readConnID reads the handshake the pump's dialer writes: the protocol marker, a length byte
// and then the connection id. The id is generated once per connectRPC, so it identifies the
// gorpc.Client that opened the connection.
func readConnID(c net.Conn) (string, error) {
	handshake := make([]byte, 6)
	if _, err := io.ReadFull(c, handshake); err != nil {
		return "", err
	}

	idLenBuf := make([]byte, 1)
	if _, err := io.ReadFull(c, idLenBuf); err != nil {
		return "", err
	}

	id := make([]byte, uint8(idLenBuf[0]))
	if _, err := io.ReadFull(c, id); err != nil {
		return "", err
	}

	return string(id), nil
}

func (ln *testListener) Close() error {
	return ln.L.Close()
}

func startRPCMock(t *testing.T, config *HybridPumpConf, dispatcher *gorpc.Dispatcher) (*gorpc.Server, error) {
	server := gorpc.NewTCPServer(config.ConnectionString, dispatcher.NewHandlerFunc())
	list := &testListener{}
	server.Listener = list
	server.LogError = gorpc.NilErrorLogger

	if err := server.Start(); err != nil {
		t.Fail()
		return nil, err
	}

	return server, nil
}

func stopRPCMock(t *testing.T, server *gorpc.Server) {
	t.Helper()
	if server != nil {
		server.Listener.Close()
		server.Stop()
	}
}

func TestHybridPumpInit(t *testing.T) {
	//nolint:govet
	tcs := []struct {
		testName             string
		givenDispatcherFuncs map[string]interface{}
		givenConfig          *HybridPumpConf
		expectedError        error
	}{
		{
			testName:    "Should return error if connection string is empty",
			givenConfig: &HybridPumpConf{}, // empty connection string
			givenDispatcherFuncs: map[string]interface{}{
				"Ping":  func() bool { return true },
				"Login": func(clientAddr, userKey string) bool { return false },
			},
			expectedError: errors.New("empty connection_string"),
		},
		{
			testName: "Should return error if invalid credentials",
			givenConfig: &HybridPumpConf{
				ConnectionString: "localhost:12345",
				APIKey:           "invalid_credentials",
			}, // empty connection string
			givenDispatcherFuncs: map[string]interface{}{
				"Ping": func() bool { return true },
				"Login": func(clientAddr, userKey string) bool {
					return userKey == "valid_credentials"
				},
			},
			expectedError: ErrRPCLogin,
		},
		{
			testName: "Should init if valid credentials",
			givenConfig: &HybridPumpConf{
				ConnectionString: "localhost:12345",
				APIKey:           "valid_credentials",
			},
			givenDispatcherFuncs: map[string]interface{}{
				"Ping": func() bool { return true },
				"Login": func(clientAddr, userKey string) bool {
					return userKey == "valid_credentials"
				},
			},
			expectedError: nil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			p := &HybridPump{}

			dispatcher := gorpc.NewDispatcher()
			for funcName, funcBody := range tc.givenDispatcherFuncs {
				dispatcher.AddFunc(funcName, funcBody)
			}

			mockServer, err := startRPCMock(t, tc.givenConfig, dispatcher)
			if err != nil {
				t.Fatalf("Failed to start RPC mock: %v", err)
			}
			defer stopRPCMock(t, mockServer)

			err = p.Init(tc.givenConfig)
			assert.Equal(t, tc.expectedError, err)

			if err == nil {
				assert.Nil(t, p.Shutdown())
			}
		})
	}
}

func TestHybridPumpWriteData(t *testing.T) {
	//nolint:govet
	tcs := []struct {
		testName             string
		givenConfig          *HybridPumpConf
		givenDispatcherFuncs map[string]interface{}
		givenData            []interface{}
		expectedError        error
	}{
		{
			testName: "write non aggregated data",
			givenConfig: &HybridPumpConf{
				ConnectionString: "localhost:12345",
				APIKey:           "valid_credentials",
			},
			givenDispatcherFuncs: map[string]interface{}{
				"Ping": func() bool { return true },
				"Login": func(clientAddr, userKey string) bool {
					return userKey == "valid_credentials"
				},
				"PurgeAnalyticsData": func(clientID, data string) error {
					if data == "" {
						return errors.New("empty data")
					}
					return nil
				},
			},
			givenData: []interface{}{
				analytics.AnalyticsRecord{
					APIID:   "testAPIID",
					OrgID:   "testOrg",
					APIName: "testAPIName",
				},
				analytics.AnalyticsRecord{
					APIID:   "testAPIID2",
					OrgID:   "testOrg2",
					APIName: "testAPIName2",
				},
			},
			expectedError: nil,
		},
		{
			testName: "write aggregated data",
			givenConfig: &HybridPumpConf{
				ConnectionString: "localhost:12345",
				APIKey:           "valid_credentials",
				Aggregated:       true,
			},
			givenDispatcherFuncs: map[string]interface{}{
				"Ping": func() bool { return true },
				"Login": func(clientAddr, userKey string) bool {
					return userKey == "valid_credentials"
				},
				"PurgeAnalyticsDataAggregated": func(clientID, data string) error {
					if data == "" {
						return errors.New("empty data")
					}
					return nil
				},
			},
			givenData: []interface{}{
				analytics.AnalyticsRecord{
					APIID:   "testAPIID",
					OrgID:   "testOrg",
					APIName: "testAPIName",
				},
				analytics.AnalyticsRecord{
					APIID:   "testAPIID2",
					OrgID:   "testOrg2",
					APIName: "testAPIName2",
				},
			},
			expectedError: nil,
		},
		{
			testName: "write aggregated data with MCP records",
			givenConfig: &HybridPumpConf{
				ConnectionString:     "localhost:12345",
				APIKey:               "valid_credentials",
				Aggregated:           true,
				EnableMCPAggregation: true,
			},
			givenDispatcherFuncs: map[string]interface{}{
				"Ping": func() bool { return true },
				"Login": func(clientAddr, userKey string) bool {
					return userKey == "valid_credentials"
				},
				"PurgeAnalyticsDataAggregated": func(clientID, data string) error {
					return nil
				},
				"PurgeAnalyticsDataMCPAggregated": func(clientID, data string) error {
					return nil
				},
			},
			givenData: []interface{}{
				analytics.AnalyticsRecord{
					APIID:        "testAPIID",
					OrgID:        "testOrg",
					APIName:      "testAPIName",
					ResponseCode: 200,
					TimeStamp:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
					MCPStats: analytics.MCPStats{
						IsMCP:         true,
						JSONRPCMethod: "tools/call",
						PrimitiveType: "tool",
						PrimitiveName: "weather",
					},
				},
				analytics.AnalyticsRecord{
					APIID:        "testAPIID",
					OrgID:        "testOrg",
					APIName:      "testAPIName",
					ResponseCode: 200,
				},
			},
			expectedError: nil,
		},
		{
			testName: "write aggregated data with only non-MCP records skips MCP RPC",
			givenConfig: &HybridPumpConf{
				ConnectionString: "localhost:12345",
				APIKey:           "valid_credentials",
				Aggregated:       true,
			},
			givenDispatcherFuncs: map[string]interface{}{
				"Ping": func() bool { return true },
				"Login": func(clientAddr, userKey string) bool {
					return userKey == "valid_credentials"
				},
				"PurgeAnalyticsDataAggregated": func(clientID, data string) error {
					return nil
				},
				// PurgeAnalyticsDataMCPAggregated NOT registered - sendMCPAggregates
				// should return nil without calling it because there are no MCP records
			},
			givenData: []interface{}{
				analytics.AnalyticsRecord{
					APIID:   "testAPIID",
					OrgID:   "testOrg",
					APIName: "testAPIName",
				},
			},
			expectedError: nil,
		},
		{
			testName: "write aggregated data with MCP records but MCP aggregation disabled",
			givenConfig: &HybridPumpConf{
				ConnectionString: "localhost:12345",
				APIKey:           "valid_credentials",
				Aggregated:       true,
			},
			givenDispatcherFuncs: map[string]interface{}{
				"Ping": func() bool { return true },
				"Login": func(clientAddr, userKey string) bool {
					return userKey == "valid_credentials"
				},
				"PurgeAnalyticsDataAggregated": func(clientID, data string) error {
					return nil
				},
				// PurgeAnalyticsDataMCPAggregated NOT registered - MCP aggregation
				// is disabled so sendMCPAggregates should not be called
			},
			givenData: []interface{}{
				analytics.AnalyticsRecord{
					APIID:        "testAPIID",
					OrgID:        "testOrg",
					APIName:      "testAPIName",
					ResponseCode: 200,
					TimeStamp:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
					MCPStats: analytics.MCPStats{
						IsMCP:         true,
						JSONRPCMethod: "tools/call",
						PrimitiveType: "tool",
						PrimitiveName: "weather",
					},
				},
			},
			expectedError: nil,
		},
		{
			testName: "write aggregated data - no records",
			givenConfig: &HybridPumpConf{
				ConnectionString: "localhost:12345",
				APIKey:           "valid_credentials",
				Aggregated:       true,
			},
			givenDispatcherFuncs: map[string]interface{}{
				"Ping": func() bool { return true },
				"Login": func(clientAddr, userKey string) bool {
					return userKey == "valid_credentials"
				},
				"PurgeAnalyticsDataAggregated": func(clientID, data string) error {
					if data == "" {
						return errors.New("empty data")
					}
					return nil
				},
			},
			givenData:     []interface{}{},
			expectedError: nil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			p := &HybridPump{}
			p.New()

			dispatcher := gorpc.NewDispatcher()
			for funcName, funcBody := range tc.givenDispatcherFuncs {
				dispatcher.AddFunc(funcName, funcBody)
			}

			mockServer, err := startRPCMock(t, tc.givenConfig, dispatcher)
			if err != nil {
				t.Fatalf("Failed to start RPC mock: %v", err)
			}
			defer stopRPCMock(t, mockServer)

			err = p.Init(tc.givenConfig)
			if err != nil {
				t.Fail()
				return
			}
			defer func() {
				err := p.Shutdown()
				if err != nil {
					t.Fatalf("Failed to shutdown hybrid pump: %v", err)
				}
			}()

			err = p.WriteData(context.TODO(), tc.givenData)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestHybridPumpShutdown(t *testing.T) {
	mockConf := &HybridPumpConf{
		ConnectionString: "localhost:9092",
		RPCKey:           "testkey",
		APIKey:           "testapikey",
	}

	dispatcher := gorpc.NewDispatcher()
	dispatcher.AddFunc("Ping", func() bool { return true })
	dispatcher.AddFunc("Login", func(clientAddr, userKey string) bool {
		return userKey == mockConf.APIKey
	})

	server, err := startRPCMock(t, mockConf, dispatcher)
	assert.NoError(t, err)
	defer stopRPCMock(t, server)

	hybridPump := &HybridPump{}
	err = hybridPump.Init(mockConf)
	assert.NoError(t, err)

	err = hybridPump.Shutdown()
	assert.NoError(t, err)

	// check if the isconnected
	assert.False(t, hybridPump.clientIsConnected.Load().(bool))

	assert.Nil(t, hybridPump.clientSingleton)
}

func TestWriteLicenseExpire(t *testing.T) {
	mockConf := &HybridPumpConf{
		ConnectionString: "localhost:9092",
		RPCKey:           "testkey",
		APIKey:           "testapikey",
	}

	loginCall := 0

	dispatcher := gorpc.NewDispatcher()
	dispatcher.AddFunc("Ping", func() bool { return true })
	dispatcher.AddFunc("Login", func(clientAddr, userKey string) bool {
		loginCall++
		return loginCall <= 3
	})
	dispatcher.AddFunc("PurgeAnalyticsData", func(clientID, data string) error { return nil })

	server, err := startRPCMock(t, mockConf, dispatcher)
	assert.NoError(t, err)
	defer stopRPCMock(t, server)

	hybridPump := &HybridPump{}
	// first login - success
	err = hybridPump.Init(mockConf)
	assert.NoError(t, err)
	defer func() {
		if err := hybridPump.Shutdown(); err != nil {
			t.Fail()
		}
	}()

	// second login - success
	err = hybridPump.WriteData(context.Background(), []interface{}{analytics.AnalyticsRecord{APIKey: "testapikey"}})
	assert.Nil(t, err)

	// third login - success
	err = hybridPump.WriteData(context.Background(), []interface{}{analytics.AnalyticsRecord{APIKey: "testapikey"}})
	assert.Nil(t, err)

	// license expired, login fail - WriteData should fail
	err = hybridPump.WriteData(context.Background(), []interface{}{analytics.AnalyticsRecord{APIKey: "testapikey"}})
	assert.NotNil(t, err)
	assert.Equal(t, ErrRPCLogin, err)
}

func TestHybridConfigCheckDefaults(t *testing.T) {
	//nolint:govet
	tcs := []struct {
		testName       string
		givenConfig    *HybridPumpConf
		expectedConfig *HybridPumpConf
	}{
		{
			testName:    "default values - no aggregated",
			givenConfig: &HybridPumpConf{},
			expectedConfig: &HybridPumpConf{
				CallTimeout: DefaultRPCCallTimeout,
				Aggregated:  false,
				RPCPoolSize: 5,
			},
		},
		{
			testName: "aggregated true with StoreAnalyticsPerMinute",
			givenConfig: &HybridPumpConf{
				Aggregated:              true,
				StoreAnalyticsPerMinute: true,
			},
			expectedConfig: &HybridPumpConf{
				CallTimeout:             DefaultRPCCallTimeout,
				Aggregated:              true,
				StoreAnalyticsPerMinute: true,
				aggregationTime:         1,
				RPCPoolSize:             5,
			},
		},

		{
			testName: "aggregated true without StoreAnalyticsPerMinute",
			givenConfig: &HybridPumpConf{
				Aggregated:              true,
				StoreAnalyticsPerMinute: false,
			},
			expectedConfig: &HybridPumpConf{
				CallTimeout:             DefaultRPCCallTimeout,
				Aggregated:              true,
				StoreAnalyticsPerMinute: false,
				aggregationTime:         60,
				RPCPoolSize:             5,
			},
		},
		{
			testName: "custom timeout",
			givenConfig: &HybridPumpConf{
				CallTimeout: 20,
			},
			expectedConfig: &HybridPumpConf{
				CallTimeout: 20,
				RPCPoolSize: 5,
			},
		},

		{
			testName: "custom rpc_pool_size",
			givenConfig: &HybridPumpConf{
				CallTimeout: 20,
				RPCPoolSize: 20,
			},
			expectedConfig: &HybridPumpConf{
				CallTimeout: 20,
				RPCPoolSize: 20,
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			tc.givenConfig.CheckDefaults()

			assert.Equal(t, tc.expectedConfig, tc.givenConfig)
		})
	}
}

func TestHybridConfigParsing(t *testing.T) {
	svAddress := "localhost:9099"

	//nolint:govet
	tcs := []struct {
		testName       string
		givenEnvs      map[string]string
		givenBaseConf  map[string]interface{}
		expectedConfig *HybridPumpConf
	}{
		{
			testName: "all envs",
			givenEnvs: map[string]string{
				hybridDefaultENV + "_CONNECTIONSTRING": svAddress,
				hybridDefaultENV + "_CALLTIMEOUT":      "20",
				hybridDefaultENV + "_RPCKEY":           "testkey",
				hybridDefaultENV + "_APIKEY":           "testapikey",
				hybridDefaultENV + "_AGGREGATED":       "true",
			},
			givenBaseConf: map[string]interface{}{},
			expectedConfig: &HybridPumpConf{
				ConnectionString: svAddress,
				CallTimeout:      20,
				RPCKey:           "testkey",
				APIKey:           "testapikey",
				Aggregated:       true,
				aggregationTime:  60,
				RPCPoolSize:      5,
			},
		},
		{
			testName:  "all config",
			givenEnvs: map[string]string{},
			givenBaseConf: map[string]interface{}{
				"connection_string": svAddress,
				"call_timeout":      20,
				"rpc_key":           "testkey",
				"api_key":           "testapikey",
				"aggregated":        true,
			},
			expectedConfig: &HybridPumpConf{
				ConnectionString: svAddress,
				CallTimeout:      20,
				RPCKey:           "testkey",
				APIKey:           "testapikey",
				Aggregated:       true,
				aggregationTime:  60,
				RPCPoolSize:      5,
			},
		},

		{
			testName: "mixed config",
			givenEnvs: map[string]string{
				hybridDefaultENV + "_CONNECTIONSTRING": svAddress,
				hybridDefaultENV + "_RPCKEY":           "testkey",
				hybridDefaultENV + "_APIKEY":           "testapikey",
			},
			givenBaseConf: map[string]interface{}{
				"call_timeout":               20,
				"aggregated":                 true,
				"store_analytics_per_minute": true,
				"track_all_paths":            true,
				"rpc_pool_size":              20,
			},
			expectedConfig: &HybridPumpConf{
				ConnectionString:        svAddress,
				CallTimeout:             20,
				RPCKey:                  "testkey",
				APIKey:                  "testapikey",
				Aggregated:              true,
				StoreAnalyticsPerMinute: true,
				aggregationTime:         1,
				TrackAllPaths:           true,
				RPCPoolSize:             20,
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			for key, env := range tc.givenEnvs {
				os.Setenv(key, env)
			}
			defer func(envs map[string]string) {
				// By key. Unsetting the values left this test case's variables set for
				// every test declared after this one.
				for key := range envs {
					os.Unsetenv(key)
				}
			}(tc.givenEnvs)

			dispatcher := gorpc.NewDispatcher()
			dispatcher.AddFunc("Ping", func() bool { return true })
			dispatcher.AddFunc("Login", func(clientAddr, userKey string) bool {
				return true
			})

			server, err := startRPCMock(t, &HybridPumpConf{ConnectionString: svAddress}, dispatcher)
			assert.NoError(t, err)
			defer stopRPCMock(t, server)

			hybridPump := &HybridPump{}
			err = hybridPump.Init(tc.givenBaseConf)
			assert.NoError(t, err)
			defer func() {
				if err := hybridPump.Shutdown(); err != nil {
					t.Fail()
				}
			}()

			assert.Equal(t, tc.expectedConfig, hybridPump.hybridConfig)
		})
	}
}

func TestDispatcherFuncs(t *testing.T) {
	//nolint:govet
	tcs := []struct {
		testName       string
		function       string
		input          []interface{}
		expectedOutput interface{}
		expectedError  error
	}{
		{
			testName:       "Login",
			function:       "Login",
			input:          []interface{}{"127.0.0.1", "userKey123"},
			expectedOutput: false,
		},
		{
			testName:       "PurgeAnalyticsData",
			function:       "PurgeAnalyticsData",
			input:          []interface{}{"test data"},
			expectedOutput: nil,
			expectedError:  nil,
		},
		{
			testName:       "Ping",
			function:       "Ping",
			input:          []interface{}{},
			expectedOutput: false,
		},
		{
			testName:       "PurgeAnalyticsDataAggregated",
			function:       "PurgeAnalyticsDataAggregated",
			input:          []interface{}{"test data"},
			expectedOutput: nil,
			expectedError:  nil,
		},
		{
			testName:       "PurgeAnalyticsDataMCPAggregated",
			function:       "PurgeAnalyticsDataMCPAggregated",
			input:          []interface{}{"test data"},
			expectedOutput: nil,
			expectedError:  nil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			switch fn := dispatcherFuncs[tc.function].(type) {
			case func(string, string) bool:
				result := fn(tc.input[0].(string), tc.input[1].(string))
				if result != tc.expectedOutput {
					t.Errorf("Expected %v, got %v", tc.expectedOutput, result)
				}
			case func(string) error:
				err := fn(tc.input[0].(string))
				if !errors.Is(err, tc.expectedError) {
					t.Errorf("Expected error %v, got %v", tc.expectedError, err)
				}
			case func() bool:
				result := fn()
				if result != tc.expectedOutput {
					t.Errorf("Expected %v, got %v", tc.expectedOutput, result)
				}
			default:
				t.Errorf("Unexpected function type")
			}
		})
	}
}

func TestRetryAndLog(t *testing.T) {
	buf := bytes.Buffer{}
	testLogger := logrus.New()
	testLogger.SetOutput(&buf)

	retries := 0
	fn := func() error {
		retries++
		if retries == 3 {
			return nil
		}
		return errors.New("test error")
	}

	err := retryAndLog(fn, "retrying", testLogger.WithField("test", "test"))
	assert.Nil(t, err)
	assert.Equal(t, 3, retries)
	assert.Contains(t, buf.String(), "retrying")
}

func TestConnectAndLogin(t *testing.T) {
	//nolint:govet
	tcs := []struct {
		testName            string
		givenRetry          bool
		shouldStartSv       bool
		givenAttemptSuccess int
		expectedErr         error
	}{
		{
			testName:      "without retry - success",
			givenRetry:    false,
			shouldStartSv: true,
		},
		{
			testName:      "without retry - server down",
			givenRetry:    false,
			shouldStartSv: false,
			expectedErr:   errors.New("gorpc.Client: [localhost:9092]. Cannot obtain response during timeout=1s"),
		},
		{
			testName:      "with retry - success",
			givenRetry:    true,
			shouldStartSv: true,
		},
		{
			testName:      "with retry - server down",
			givenRetry:    true,
			shouldStartSv: false,
			expectedErr:   errors.New("gorpc.Client: [localhost:9092]. Cannot obtain response during timeout=1s"),
		},
		{
			testName:            "without retry - fail first attempt - error",
			givenRetry:          false,
			shouldStartSv:       true,
			givenAttemptSuccess: 2,
			expectedErr:         ErrRPCLogin,
		},
		{
			testName:            " retry - fail first attempt - success after",
			givenRetry:          true,
			shouldStartSv:       true,
			givenAttemptSuccess: 2,
			expectedErr:         nil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			mockConf := &HybridPumpConf{
				ConnectionString: "localhost:9092",
				RPCKey:           "testkey",
				APIKey:           "testapikey",
				CallTimeout:      1,
			}

			pump := &HybridPump{}
			pump.hybridConfig = mockConf
			pump.log = log.WithField("prefix", "hybrid-test")

			if tc.shouldStartSv {
				attempts := 0
				dispatcherFns := map[string]interface{}{
					"Ping": func() bool { return true },
					"Login": func(clientAddr, userKey string) bool {
						attempts++
						return attempts >= tc.givenAttemptSuccess
					},
				}
				dispatcher := gorpc.NewDispatcher()
				for fnName, fn := range dispatcherFns {
					dispatcher.AddFunc(fnName, fn)
				}

				server, err := startRPCMock(t, mockConf, dispatcher)
				assert.NoError(t, err)
				defer stopRPCMock(t, server)
			}

			err := pump.connectAndLogin(tc.givenRetry)
			if tc.expectedErr == nil {
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
				assert.Equal(t, err.Error(), tc.expectedErr.Error())
			}
		})
	}
}

// connectRPC used to assign a fresh gorpc.Client over the previous one without stopping it.
// Every gorpc.Client keeps rpc_pool_size clientHandler goroutines redialling MDCB until
// Client.Stop() is called, so each reconnect abandoned a whole pool of connections that neither
// side ever closed. The helpers below make that observable from the server side, which is the
// only place it shows up: nothing about the leak is visible in the pump's own behaviour.

// connCountingListener is a gorpc.Listener that tracks how many connections are currently open
// per connection id. A gorpc.Client dials rpc_pool_size connections all carrying the id its
// connectRPC generated, so a client that was replaced but never stopped shows up as a second id
// whose count never falls to zero.
//
// It owns an already-open socket on an ephemeral port, so these tests need no hardcoded port and
// cannot collide with the fixed-port tests in this file. Init is therefore a no-op: gorpc calls
// it with the address this listener is already listening on.
type connCountingListener struct {
	L net.Listener

	// live and total are both guarded by mu, which sits last only to keep the struct's pointer
	// fields together - govet's fieldalignment check objects otherwise.
	live  map[string]int
	total map[string]int

	mu sync.Mutex
}

func newConnCountingListener(t *testing.T) *connCountingListener {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	return &connCountingListener{
		L:     l,
		live:  map[string]int{},
		total: map[string]int{},
	}
}

func (ln *connCountingListener) Addr() string { return ln.L.Addr().String() }

func (ln *connCountingListener) Init(string) error { return nil }

func (ln *connCountingListener) Close() error { return ln.L.Close() }

func (ln *connCountingListener) Accept() (net.Conn, error) {
	c, err := ln.L.Accept()
	if err != nil {
		return nil, err
	}

	if err := setupKeepalive(c); err != nil {
		c.Close()
		return nil, err
	}

	connID, err := readConnID(c)
	if err != nil {
		c.Close()
		return nil, err
	}

	ln.mu.Lock()
	ln.live[connID]++
	ln.total[connID]++
	ln.mu.Unlock()

	return &countedConn{Conn: c, ln: ln, connID: connID}, nil
}

func (ln *connCountingListener) closed(connID string) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	ln.live[connID]--
}

// liveFor is how many connections one client still holds, i.e. whether it is still running.
func (ln *connCountingListener) liveFor(connID string) int {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	return ln.live[connID]
}

// liveConnIDs are the clients still holding at least one connection. One entry means one live
// gorpc.Client.
func (ln *connCountingListener) liveConnIDs() []string {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	ids := make([]string, 0, len(ln.live))
	for id, n := range ln.live {
		if n > 0 {
			ids = append(ids, id)
		}
	}

	return ids
}

// totalConnIDs are all the clients ever seen, stopped ones included, which is how many times
// connectRPC actually built one.
func (ln *connCountingListener) totalConnIDs() []string {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	ids := make([]string, 0, len(ln.total))
	for id := range ln.total {
		ids = append(ids, id)
	}

	return ids
}

// totalLive is every connection still open, across every client.
func (ln *connCountingListener) totalLive() int {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	var n int
	for _, c := range ln.live {
		if c > 0 {
			n += c
		}
	}

	return n
}

// liveSnapshot is for failure messages: which client holds how many connections.
func (ln *connCountingListener) liveSnapshot() map[string]int {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	out := make(map[string]int, len(ln.live))
	for id, n := range ln.live {
		if n > 0 {
			out[id] = n
		}
	}

	return out
}

// countedConn reports its connection as closed exactly once. gorpc closes a server connection
// from more than one path, and double counting would make a live client look stopped.
type countedConn struct {
	net.Conn

	ln     *connCountingListener
	connID string
	once   sync.Once
}

func (c *countedConn) Close() error {
	c.once.Do(func() { c.ln.closed(c.connID) })

	return c.Conn.Close()
}

func startRPCMockWithListener(
	t *testing.T, addr string, dispatcher *gorpc.Dispatcher, ln gorpc.Listener,
) *gorpc.Server {
	t.Helper()

	server := gorpc.NewTCPServer(addr, dispatcher.NewHandlerFunc())
	server.Listener = ln
	server.LogError = gorpc.NilErrorLogger

	require.NoError(t, server.Start())
	t.Cleanup(func() { stopRPCMock(t, server) })

	return server
}

// stopClientQuietly stops c if it is still running. gorpc.Client.Stop() panics on a client that
// was never started or was already stopped, and cleanup must not care which of those it is: the
// point is only that no test leaves a pool of clientHandler goroutines redialling for the rest
// of the package run.
func stopClientQuietly(c *gorpc.Client) {
	if c == nil {
		return
	}

	// The panic is the expected outcome for an already stopped client, and its value carries
	// nothing worth inspecting.
	defer func() { recover() }() //nolint:errcheck

	c.Stop()
}

// newLeakTestPump builds a pump against a fresh mock server on an ephemeral port. It assigns the
// config directly instead of going through Init, so there is no mapstructure and no dependency
// on the process environment.
func newLeakTestPump(
	t *testing.T, dispatcherFns map[string]interface{}, poolSize int,
) (*HybridPump, *connCountingListener) {
	t.Helper()

	ln := newConnCountingListener(t)

	dispatcher := gorpc.NewDispatcher()
	for fnName, fn := range dispatcherFns {
		dispatcher.AddFunc(fnName, fn)
	}
	startRPCMockWithListener(t, ln.Addr(), dispatcher, ln)

	pump := &HybridPump{}
	pump.hybridConfig = &HybridPumpConf{
		ConnectionString: ln.Addr(),
		RPCKey:           "testkey",
		APIKey:           "testapikey",
		CallTimeout:      1,
		RPCPoolSize:      poolSize,
	}
	pump.log = log.WithField("prefix", "hybrid-test")

	t.Cleanup(func() { stopClientQuietly(pump.clientSingleton) })

	return pump, ln
}

// waitForSingleLiveConnID waits until exactly one client holds want connections and returns its
// id, so a test can pin the baseline before provoking a reconnect.
func waitForSingleLiveConnID(t *testing.T, ln *connCountingListener, want int) string {
	t.Helper()

	var id string
	require.Eventually(t, func() bool {
		ids := ln.liveConnIDs()
		if len(ids) != 1 || ln.liveFor(ids[0]) != want {
			return false
		}
		id = ids[0]

		return true
	}, 10*time.Second, 25*time.Millisecond,
		"expected one client holding %d connections, have %v", want, ln.liveSnapshot())

	return id
}

// rpcReleaseBudget is how long these tests give the mock server to let go of the connections of
// a client the pump has stopped.
//
// Measured, with no pump involved at all: after gorpc.Client.Stop() returns, the server closes
// its end of each connection either straight away or after almost exactly ten seconds, never
// later. Which of the two it is varies per connection and per run. What is under test here is
// that the pump lets go of the client at all; how long gorpc then takes to notice is not this
// package's business, so the budget only has to clear that ten seconds. An unfixed pump never
// lets go, and its connections stay up for as long as the process lives, so nothing is hidden by
// waiting a little longer.
const rpcReleaseBudget = 15 * time.Second

// waitForLive polls until cond holds, and on timeout reports the state as it is then. It exists
// because assert.Eventually evaluates its message arguments eagerly, so a snapshot passed to it
// describes the moment the wait started rather than the moment it gave up.
func waitForLive(t *testing.T, ln *connCountingListener, budget time.Duration, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(budget)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(25 * time.Millisecond)
	}

	t.Errorf("%s; connections still open per client: %v", what, ln.liveSnapshot())
}

func TestConnectRPCStopsPreviousClient(t *testing.T) {
	const poolSize = 3

	pump, ln := newLeakTestPump(t, map[string]interface{}{
		"Ping":  func() bool { return true },
		"Login": func(_, _ string) bool { return true },
	}, poolSize)

	require.NoError(t, pump.connectRPC())

	first := pump.clientSingleton
	require.NotNil(t, first)
	t.Cleanup(func() { stopClientQuietly(first) })

	firstID := waitForSingleLiveConnID(t, ln, poolSize)

	require.NoError(t, pump.connectRPC())

	second := pump.clientSingleton
	require.NotNil(t, second)
	require.NotSame(t, first, second)
	t.Cleanup(func() { stopClientQuietly(second) })

	// Without a Stop() the replaced client keeps its pool redialling for the lifetime of the
	// process, and MDCB keeps paying for every one of those connections.
	waitForLive(t, ln, rpcReleaseBudget, "the replaced client still holds connections", func() bool {
		return ln.liveFor(firstID) == 0 && ln.totalLive() == poolSize
	})

	// Direct proof that connectRPC stopped it, rather than its connections merely having gone
	// away: gorpc.Client.Stop() panics unless the client is running. Asserted last, because it
	// stops the client itself and would otherwise satisfy the check above.
	assert.Panics(t, func() { first.Stop() },
		"connectRPC left the previous gorpc client running")
}

func TestConnectAndLoginRetryDoesNotOrphanClients(t *testing.T) {
	const poolSize = 2

	var pings int64

	pump, ln := newLeakTestPump(t, map[string]interface{}{
		// The first two answers arrive after call_timeout, so the first two connectRPC attempts
		// fail and the retry builds another client for each of them.
		"Ping": func() bool {
			if atomic.AddInt64(&pings, 1) <= 2 {
				time.Sleep(1500 * time.Millisecond)
			}

			return true
		},
		"Login": func(_, _ string) bool { return true },
	}, poolSize)

	require.NoError(t, pump.connectAndLogin(true))
	t.Cleanup(func() { stopClientQuietly(pump.clientSingleton) })

	// Guard the premise: with no retries there is nothing to orphan and the assertion below
	// would pass for the wrong reason.
	require.GreaterOrEqual(t, len(ln.totalConnIDs()), 3, "the retry path did not run")

	waitForLive(t, ln, rpcReleaseBudget, "the retried connect attempts left clients behind", func() bool {
		return len(ln.liveConnIDs()) == 1 && ln.totalLive() == poolSize
	})
}

func TestConcurrentReconnectKeepsSingleClient(t *testing.T) {
	const (
		poolSize = 2
		callers  = 8
	)

	pump, ln := newLeakTestPump(t, map[string]interface{}{
		"Ping":  func() bool { return true },
		"Login": func(_, _ string) bool { return true },
	}, poolSize)

	require.NoError(t, pump.connectRPC())
	waitForSingleLiveConnID(t, ln, poolSize)

	// A WriteData call that outlives the pump's configured timeout is abandoned and started
	// again on the next purge cycle, so several recoveries can be in flight at once.
	var (
		mu     sync.Mutex
		errs   []error
		panics []interface{}
		wg     sync.WaitGroup
	)

	start := make(chan struct{})

	for i := 0; i < callers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					mu.Lock()
					panics = append(panics, r)
					mu.Unlock()
				}
			}()

			<-start

			err := pump.connectAndLogin(false)

			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
		}()
	}

	close(start)
	wg.Wait()

	t.Cleanup(func() { stopClientQuietly(pump.clientSingleton) })

	// Two recoveries must not stop the same client, which gorpc answers with a panic.
	assert.Empty(t, panics, "concurrent reconnects panicked")

	for _, err := range errs {
		assert.NoError(t, err)
	}

	// And only one client may survive them.
	waitForLive(t, ln, rpcReleaseBudget, "more than one client survived concurrent reconnects", func() bool {
		return len(ln.liveConnIDs()) == 1 && ln.totalLive() == poolSize
	})

	// The survivor must be the published one, i.e. the pump's state is not torn.
	_, err := pump.callRPCFn("Ping", nil)
	assert.NoError(t, err)
}

func TestShutdownWithoutClient(t *testing.T) {
	// Init returns before connectRPC for an invalid configuration, and a pump whose Init failed
	// is still shut down along with the others.
	pump := &HybridPump{}
	pump.hybridConfig = &HybridPumpConf{}
	pump.log = log.WithField("prefix", "hybrid-test")

	assert.NotPanics(t, func() {
		assert.NoError(t, pump.Shutdown())
	})
}

func TestShutdownIsIdempotent(t *testing.T) {
	pump, _ := newLeakTestPump(t, map[string]interface{}{
		"Ping":  func() bool { return true },
		"Login": func(_, _ string) bool { return true },
	}, 2)

	require.NoError(t, pump.connectRPC())
	require.NoError(t, pump.Shutdown())

	assert.NotPanics(t, func() {
		assert.NoError(t, pump.Shutdown())
	})
	assert.Nil(t, pump.clientSingleton)

	connected, ok := pump.clientIsConnected.Load().(bool)
	require.True(t, ok, "clientIsConnected was never set")
	assert.False(t, connected)
}

func TestInitFailureReleasesRPCClient(t *testing.T) {
	const poolSize = 2

	ln := newConnCountingListener(t)

	dispatcher := gorpc.NewDispatcher()
	dispatcher.AddFunc("Ping", func() bool { return true })
	dispatcher.AddFunc("Login", func(_, _ string) bool { return false })
	startRPCMockWithListener(t, ln.Addr(), dispatcher, ln)

	// Init reads the pump's env namespace, so pin the address this test needs instead of
	// inheriting whatever the ambient environment points the hybrid pump at.
	t.Setenv("TYK_PMP_PUMPS_HYBRID_META_CONNECTIONSTRING", ln.Addr())

	pump := &HybridPump{}
	err := pump.Init(&HybridPumpConf{
		ConnectionString: ln.Addr(),
		APIKey:           "wrong",
		CallTimeout:      1,
		RPCPoolSize:      poolSize,
	})
	t.Cleanup(func() { stopClientQuietly(pump.clientSingleton) })

	require.ErrorIs(t, err, ErrRPCLogin)

	// Nothing shuts down a pump whose Init failed, so Init must not leave a client behind when
	// it returns an error. Releasing it also clears the pointer, and that part is immediate.
	assert.Nil(t, pump.clientSingleton, "Init returned an error but kept its RPC client")

	// And the connections have to go - see rpcReleaseBudget for why that is not immediate.
	waitForLive(t, ln, rpcReleaseBudget, "Init left an RPC client connected", func() bool {
		return ln.totalLive() == 0
	})
}
