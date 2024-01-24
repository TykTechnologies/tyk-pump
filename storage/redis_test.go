package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/TykTechnologies/storage/temporal/model"
	"github.com/stretchr/testify/assert"
)

func TestRedisAddressConfiguration(t *testing.T) {
	t.Run("Host but no port", func(t *testing.T) {
		cfg := RedisStorageConfig{Host: "host"}
		if len(getRedisAddrs(cfg)) != 0 {
			t.Fatal("Port is 0, there is no valid addr")
		}
	})

	t.Run("Port but no host", func(t *testing.T) {
		cfg := RedisStorageConfig{Port: 30000}

		addrs := getRedisAddrs(cfg)
		if addrs[0] != ":30000" || len(addrs) != 1 {
			t.Fatal("Port is valid, it is a valid addr")
		}
	})

	t.Run("addrs parameter should have precedence", func(t *testing.T) {
		cfg := RedisStorageConfig{Host: "host", Port: 30000}

		addrs := getRedisAddrs(cfg)
		if addrs[0] != "host:30000" || len(addrs) != 1 {
			t.Fatal("Wrong address")
		}

		cfg.Addrs = []string{"override:30000"}

		addrs = getRedisAddrs(cfg)
		if addrs[0] != "override:30000" || len(addrs) != 1 {
			t.Fatal("Wrong address")
		}
	})
}

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

	r := RedisClusterStorageManager{}
	if err := r.Init(conf); err != nil {
		t.Fatal("unable to connect", err.Error())
	}

	connected := r.Connect()
	if !connected {
		t.Fatal("failed to connect")
	}

	if r.db == nil {
		t.Fatal("db is empty")
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
				err := r.db.list.Append(ctx, false, r.fixKey(mockKeyName), in...)
				if err != nil {
					t.Fatal(err)
				}
			}

			iterations := 1
			if tt.chunk > 0 {
				iterations = len(tt.in) / int(tt.chunk)
				if rem := len(tt.in) % int(tt.chunk); rem > 0 {
					iterations += 1
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

func TestNewRedisClusterPool(t *testing.T) {
	testCases := []struct {
		config           *RedisStorageConfig
		testName         string
		forceReconnect   bool
		expectConnection bool
	}{
		{
			testName:         "Connect to localhost:6379",
			config:           &RedisStorageConfig{Host: "localhost", Port: 6379},
			expectConnection: true,
		},
		{
			testName:         "Force reconnect with existing singleton",
			forceReconnect:   true,
			config:           &RedisStorageConfig{Host: "localhost", Port: 6379},
			expectConnection: true,
		},

		{
			testName:         "Invalid configuration",
			config:           &RedisStorageConfig{Host: "invalid-host", Port: 6379},
			expectConnection: false,
			forceReconnect:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			NewRedisClusterPool(tc.forceReconnect, tc.config)

			assert.NotNil(t, redisClusterSingleton, "Expected redisClusterSingleton not to be nil")

			assert.NotNil(t, redisClusterSingleton.conn, "Expected connection not to be nil")
			assert.NotNil(t, redisClusterSingleton.kv, "Expected kv not to be nil")
			assert.NotNil(t, redisClusterSingleton.list, "Expected list not to be nil")
			assert.Equal(t, model.RedisV9Type, redisClusterSingleton.conn.Type(), "Expected connection type to be RedisV9Type")

			if tc.expectConnection {
				assert.NoError(t, redisClusterSingleton.conn.Ping(context.Background()), "Expected no error on ping")
			} else {
				assert.Error(t, redisClusterSingleton.conn.Ping(context.Background()), "Expected error on ping")
			}
		})
	}
}
