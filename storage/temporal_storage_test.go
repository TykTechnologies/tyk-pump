package storage

import (
	"context"
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

	r := TemporalStorageHandler{}
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
			err := NewTemporalStorageHandler(tc.forceReconnect, tc.config)
			assert.NoError(t, err, "Expected no error on NewTemporalStorageHandler")

			assert.NotNil(t, temporalStorageSingleton, "Expected temporalStorageSingleton not to be nil")

			assert.NotNil(t, temporalStorageSingleton.conn, "Expected connection not to be nil")
			assert.NotNil(t, temporalStorageSingleton.kv, "Expected kv not to be nil")
			assert.NotNil(t, temporalStorageSingleton.list, "Expected list not to be nil")
			assert.Equal(t, model.RedisV9Type, temporalStorageSingleton.conn.Type(), "Expected connection type to be RedisV9Type")

			if tc.expectConnection {
				assert.NoError(t, temporalStorageSingleton.conn.Ping(context.Background()), "Expected no error on ping")
			} else {
				assert.Error(t, temporalStorageSingleton.conn.Ping(context.Background()), "Expected error on ping")
			}
		})
	}
}
