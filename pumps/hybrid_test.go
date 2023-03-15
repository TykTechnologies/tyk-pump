package pumps

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/TykTechnologies/gorpc"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
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

	handshake := make([]byte, 6)
	if _, err = io.ReadFull(c, handshake); err != nil {
		return
	}

	idLenBuf := make([]byte, 1)
	if _, err = io.ReadFull(c, idLenBuf); err != nil {
		return
	}

	idLen := uint8(idLenBuf[0])
	id := make([]byte, idLen)
	if _, err = io.ReadFull(c, id); err != nil {
		return
	}

	return c, nil
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
			testName: "write aggregated data - no records",
			givenConfig: &HybridPumpConf{
				ConnectionString: "localhost:12345",
				APIKey:           "valid_credentials",
				Aggregated:       true,
			},
			givenDispatcherFuncs: map[string]interface{}{
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
				CallTimeout: 30,
				Aggregated:  false,
			},
		},
		{
			testName: "aggregated true with StoreAnalyticsPerMinute",
			givenConfig: &HybridPumpConf{
				Aggregated:              true,
				StoreAnalyticsPerMinute: true,
			},
			expectedConfig: &HybridPumpConf{
				CallTimeout:             30,
				Aggregated:              true,
				StoreAnalyticsPerMinute: true,
				aggregationTime:         1,
			},
		},

		{
			testName: "aggregated true without StoreAnalyticsPerMinute",
			givenConfig: &HybridPumpConf{
				Aggregated:              true,
				StoreAnalyticsPerMinute: false,
			},
			expectedConfig: &HybridPumpConf{
				CallTimeout:             30,
				Aggregated:              true,
				StoreAnalyticsPerMinute: false,
				aggregationTime:         60,
			},
		},
		{
			testName: "custom timeout",
			givenConfig: &HybridPumpConf{
				CallTimeout: 20,
			},
			expectedConfig: &HybridPumpConf{
				CallTimeout: 20,
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

func TestConfigParsing(t *testing.T) {
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
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			for key, env := range tc.givenEnvs {
				os.Setenv(key, env)
			}
			defer func(envs map[string]string) {
				for _, env := range envs {
					os.Unsetenv(env)
				}
			}(tc.givenEnvs)

			dispatcher := gorpc.NewDispatcher()
			dispatcher.AddFunc("Login", func(clientAddr, userKey string) bool {
				return true
			})

			server, err := startRPCMock(t, &HybridPumpConf{ConnectionString: svAddress}, dispatcher)
			assert.NoError(t, err)
			defer stopRPCMock(t, server)

			hybridPump := &HybridPump{}
			err = hybridPump.Init(tc.givenBaseConf)
			assert.NoError(t, err)

			assert.Equal(t, tc.expectedConfig, hybridPump.hybridConfig)
		})
	}
}
