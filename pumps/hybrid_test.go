package pumps

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/TykTechnologies/gorpc"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
)

func setupKeepalive(conn net.Conn) error {
	tcpConn := conn.(*net.TCPConn)
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

func startRPCMock(t *testing.T, config HybridPumpConf, dispatcher *gorpc.Dispatcher) (*gorpc.Server, error) {
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
	tcs := []struct {
		testName             string
		givenConfig          HybridPumpConf
		givenDispatcherFuncs map[string]interface{}
		expectedError        error
	}{
		{
			testName:    "Should return error if connection string is empty",
			givenConfig: HybridPumpConf{}, // empty connection string
			givenDispatcherFuncs: map[string]interface{}{
				"Login": func(clientAddr, userKey string) bool { return false },
			},
			expectedError: errors.New("empty connection_string"),
		},
		{
			testName: "Should return error if invalid credentials",
			givenConfig: HybridPumpConf{
				ConnectionString: "localhost:12345",
				APIKey:           "invalid_credentials",
			}, // empty connection string
			givenDispatcherFuncs: map[string]interface{}{
				"Login": func(clientAddr, userKey string) bool {
					return userKey == "valid_credentials"
				},
			},
			expectedError: errors.New("RPC Login incorrect"),
		},
		{
			testName: "Should init if valid credentials",
			givenConfig: HybridPumpConf{
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
			defer p.Shutdown()

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
		})
	}
}

func TestHybridPumpWriteData(t *testing.T) {
	tcs := []struct {
		testName             string
		givenConfig          HybridPumpConf
		givenDispatcherFuncs map[string]interface{}
		givenData            []interface{}
		expectedError        error
	}{
		{
			testName: "write non aggregated data",
			givenConfig: HybridPumpConf{
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
			givenConfig: HybridPumpConf{
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
			givenConfig: HybridPumpConf{
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
			defer p.Shutdown()

			err = p.WriteData(context.TODO(), tc.givenData)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestHybridPumpShutdown(t *testing.T) {
	mockConf := HybridPumpConf{
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
