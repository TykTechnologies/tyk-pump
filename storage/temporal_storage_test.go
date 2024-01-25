package storage

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/TykTechnologies/storage/temporal/model"
	"github.com/stretchr/testify/assert"
)

var testData = []struct {
	in    []string
	chunk int64
}{
	{in: nil, chunk: int64(0)},
	{in: []string{"one"}, chunk: int64(0)},
	{in: []string{"one", "two"}, chunk: int64(0)},
	{in: []string{"one", "two", "three"}, chunk: int64(0)},
	{in: []string{"one", "two", "three", "four"}, chunk: int64(0)},
	{in: []string{"one", "two", "three", "four", "five"}, chunk: int64(0)},
	{in: nil, chunk: int64(1)},
	{in: []string{"one"}, chunk: int64(1)},
	{in: []string{"one", "two"}, chunk: int64(1)},
	{in: []string{"one", "two", "three"}, chunk: int64(1)},
	{in: []string{"one", "two", "three", "four"}, chunk: int64(1)},
	{in: []string{"one", "two", "three", "four", "five"}, chunk: int64(1)},
	{in: nil, chunk: int64(2)},
	{in: []string{"one"}, chunk: int64(2)},
	{in: []string{"one", "two"}, chunk: int64(2)},
	{in: []string{"one", "two", "three"}, chunk: int64(2)},
	{in: []string{"one", "two", "three", "four"}, chunk: int64(2)},
	{in: []string{"one", "two", "three", "four", "five"}, chunk: int64(2)},
	{in: nil, chunk: int64(3)},
	{in: []string{"one"}, chunk: int64(3)},
	{in: []string{"one", "two"}, chunk: int64(3)},
	{in: []string{"one", "two", "three"}, chunk: int64(3)},
	{in: []string{"one", "two", "three", "four"}, chunk: int64(3)},
	{in: []string{"one", "two", "three", "four", "five"}, chunk: int64(3)},
}

func TestRedisClusterStorageManager_GetAndDeleteSet(t *testing.T) {
	conf := make(map[string]interface{})
	conf["host"] = "localhost"
	conf["port"] = 6379

	r, err := NewTemporalStorageHandler(conf, false)
	if err != nil {
		t.Fatal(err)
	}

	if err := r.Init(); err != nil {
		t.Fatal("unable to connect", err.Error())
	}

	if connectorSingleton == nil {
		t.Fatal("connectorSingleton is nil")
	}

	mockKeyName := "testanalytics"

	for _, tt := range testData {
		t.Run(fmt.Sprintf("in: %v", tt), func(t *testing.T) {
			ctx := context.Background()
			if tt.in != nil {
				in := [][]byte{}
				for _, v := range tt.in {
					in = append(in, []byte(v))
				}
				err := r.list.Append(ctx, false, r.fixKey(mockKeyName), in...)
				if err != nil {
					t.Fatal(err)
				}
			}

			iterations := 1
			if tt.chunk > 0 {
				iterations = len(tt.in) / int(tt.chunk)
				if rem := len(tt.in) % int(tt.chunk); rem > 0 {
					iterations++
				}
			}

			t.Log("iterations", iterations, "tt.in", len(tt.in), "tt.chunk", tt.chunk)

			count := 0
			for i := 0; i < iterations; i++ {
				res, err := r.GetAndDeleteSet(mockKeyName, tt.chunk, 60*time.Second)
				if err != nil {
					t.Fatal(err)
				}
				count += len(res)
				t.Logf("---> %d: %v", i, res)
			}

			if count != len(tt.in) {
				t.Fatal()
			}
		})
	}
}

func TestNewTemporalClusterStorageHandler(t *testing.T) {
	testCases := []struct {
		config           *TemporalStorageConfig
		testName         string
		forceReconnect   bool
		expectConnection bool
	}{
		{
			testName:         "Connect to localhost:6379",
			config:           &TemporalStorageConfig{Host: "localhost", Port: 6379},
			expectConnection: true,
		},
		{
			testName:         "Force reconnect with existing singleton",
			forceReconnect:   true,
			config:           &TemporalStorageConfig{Host: "localhost", Port: 6379},
			expectConnection: true,
		},
		{
			testName:         "Invalid configuration",
			config:           &TemporalStorageConfig{Host: "invalid-host", Port: 6379},
			expectConnection: false,
			forceReconnect:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			tsh, err := NewTemporalStorageHandler(tc.config, tc.forceReconnect)
			assert.NoError(t, err, "Expected no error on NewTemporalStorageHandler")

			err = tsh.Init()
			assert.NoError(t, err, "Expected no error on NewTemporalStorageHandler Init method")

			assert.NotNil(t, connectorSingleton, "Expected connectorSingleton not to be nil")

			assert.NotNil(t, connectorSingleton, "Expected connectorSingleton not to be nil")
			assert.NotNil(t, tsh.kv, "Expected kv not to be nil")
			assert.NotNil(t, tsh.list, "Expected list not to be nil")
			assert.Equal(t, model.RedisV9Type, connectorSingleton.Type(), "Expected connection type to be RedisV9Type")

			if tc.expectConnection {
				assert.NoError(t, connectorSingleton.Ping(context.Background()), "Expected no error on ping")
			} else {
				assert.Error(t, connectorSingleton.Ping(context.Background()), "Expected error on ping")
			}
		})
	}
}
func TestTemporalStorageHandler_ensureConnection(t *testing.T) {
	conf := make(map[string]interface{})
	conf["host"] = "localhost"
	conf["port"] = 6379

	r, err := NewTemporalStorageHandler(conf, false)
	if err != nil {
		t.Fatal(err)
	}

	if err := r.Init(); err != nil {
		t.Fatal("unable to connect", err.Error())
	}

	if connectorSingleton == nil {
		t.Fatal("connectorSingleton is nil")
	}

	t.Run("Connection already established", func(t *testing.T) {
		err := r.ensureConnection()
		assert.NoError(t, err, "Expected no error when connection is already established")
	})

	t.Run("Connection dropped, reconnecting", func(t *testing.T) {
		connectorSingleton = nil
		err := r.ensureConnection()
		assert.NoError(t, err, "Expected no error when reconnecting")
		assert.NotNil(t, connectorSingleton, "Expected connectorSingleton not to be nil after reconnecting")
	})

	// This test timeouts because of the exponential backoff:
	// t.Run("Connection failed after several attempts", func(t *testing.T) {
	// 	connectorSingleton = nil
	// 	conf["type"] = "invalid"
	// 	r, err = NewTemporalStorageHandler(conf, true)
	// 	assert.NoError(t, err, "Expected no error when creating new TemporalStorageHandler")

	// 	err = r.ensureConnection()
	// 	assert.Error(t, err, "Expected error when reconnecting after connection failure")
	// })
}

func TestTemporalStorageHandler_SetKey(t *testing.T) {
	conf := make(map[string]interface{})
	conf["host"] = "localhost"
	conf["port"] = 6379

	r, err := NewTemporalStorageHandler(conf, false)
	if err != nil {
		t.Fatal(err)
	}

	if err := r.Init(); err != nil {
		t.Fatal("unable to connect", err.Error())
	}

	if connectorSingleton == nil {
		t.Fatal("connectorSingleton is nil")
	}

	keyName := "testKey"
	session := "testSession"
	timeout := int64(60)

	err = r.SetKey(keyName, session, timeout)
	if err != nil {
		t.Fatal(err)
	}

	// Verify that the key was set correctly
	res, err := r.kv.Get(ctx, r.fixKey(keyName))
	if err != nil {
		t.Fatal(err)
	}

	if res != session {
		t.Fatalf("Expected value %s, got %s", session, res)
	}
}
func TestTemporalStorageHandler_GetName(t *testing.T) {
	conf := make(map[string]interface{})
	conf["host"] = "localhost"
	conf["port"] = 6379

	r, err := NewTemporalStorageHandler(conf, false)
	if err != nil {
		t.Fatal(err)
	}

	if err := r.Init(); err != nil {
		t.Fatal("unable to connect", err.Error())
	}

	if connectorSingleton == nil {
		t.Fatal("connectorSingleton is nil")
	}

	expected := "redis"
	result := r.GetName()

	if result != expected {
		t.Fatalf("Expected %s, but got %s", expected, result)
	}
}
func TestTemporalStorageHandler_Init(t *testing.T) {
	testCases := []struct {
		name           string
		conf           map[string]interface{}
		forceReconnect bool
		errExpected    error
	}{
		{
			name: "Valid configuration",
			conf: map[string]interface{}{
				"host": "localhost",
				"port": 6379,
			},
		},
		{
			name: "Invalid configuration",
			conf: map[string]interface{}{
				"host": "abc",
				"port": 6379,
				"type": "invalid",
			},
			forceReconnect: true,
			errExpected:    errors.New("unsupported database type: invalid"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := NewTemporalStorageHandler(tc.conf, tc.forceReconnect)
			if err != nil {
				t.Fatal(err)
			}

			err = r.Init()
			if err != nil {
				assert.Error(t, err, tc.errExpected)
				return
			}

			assert.NotNil(t, r.Config, "Expected Config not to be nil")
			assert.NotNil(t, connectorSingleton, "Expected connectorSingleton not to be nil")
			assert.NotNil(t, r.kv, "Expected kv not to be nil")
			assert.NotNil(t, r.list, "Expected list not to be nil")
			assert.Equal(t, model.RedisV9Type, connectorSingleton.Type(), "Expected connection type to be RedisV9Type")
			assert.NoError(t, connectorSingleton.Ping(context.Background()), "Expected no error on ping")
		})
	}
}
