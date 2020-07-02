package storage

import (
	"fmt"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/go-redis/redis"
	"github.com/ory/dockertest/v3"
	"gopkg.in/vmihailenco/msgpack.v2"

	"os"
	"strconv"
	"testing"
)

const (
	redisDockerImage = "redis"
)

var (
	testPool *dockertest.Pool
)

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	teardown()
	os.Exit(code)
}
func setup() {
	var err error
	testPool, err = dockertest.NewPool("")
	if err != nil {
		log.Fatal(err)
	}

	//Redis
	if _, found := testPool.ContainerByName(redisDockerImage); !found {
		_, err = testPool.RunWithOptions(&dockertest.RunOptions{Name: redisDockerImage, Repository: redisDockerImage, Tag: "3.2", Env: nil})
		if err != nil {
			log.Fatalf("Could not start Redis resource: %s", err)
		}
	}

}

func teardown() {
	resourceRedis, _ := testPool.ContainerByName(redisDockerImage)
	testPool.Purge(resourceRedis)

}

func TestGetAndDeleteSet(t *testing.T) {
	resource, found := testPool.ContainerByName(redisDockerImage)
	if !found {
		t.Fatalf("Redis resource not found")
	}

	config := RedisStorageConfig{}
	storage := RedisClusterStorageManager{}
	config.Host = "localhost"
	var db *redis.Client
	if err := testPool.Retry(func() error {
		db = redis.NewClient(&redis.Options{
			Addr: fmt.Sprintf("localhost:%s", resource.GetPort("6379/tcp")),
		})
		intPort, _ := strconv.Atoi(resource.GetPort("6379/tcp"))
		config.Port = intPort
		return db.Ping().Err()
	}); err != nil {
		t.Fatalf("Could not connect to Redis docker: %s", err)
	}

	config.RedisKeyPrefix = "analytics-tyk-"
	storage.Init(config)

	record := analytics.AnalyticsRecord{APIID: "api-1", OrgID: "org-1"}
	saveTestRedisRecord(t, db, record)

	results := storage.GetAndDeleteSet("system-analytics")

	if len(results) != 1 {
		t.Fatal("Results len should be 1")
	}

	results = storage.GetAndDeleteSet("system-analytics")
	if len(results) != 0 {
		t.Fatal("Results len should be 0")
	}

}

func TestFixKey(t *testing.T) {
	config := RedisStorageConfig{}
	storage := RedisClusterStorageManager{}
	config.RedisKeyPrefix = "test"
	storage.Init(config)

	result := storage.fixKey("test_key")
	if result != "testtest_key" {
		t.Fatal("Incorrect fixedKey")
	}
}

func TestSetKey(t *testing.T) {
	resource, found := testPool.ContainerByName(redisDockerImage)
	if !found {
		t.Fatalf("Redis resource not found")
	}

	config := RedisStorageConfig{}
	storage := RedisClusterStorageManager{}
	config.Host = "localhost"
	var db *redis.Client
	if err := testPool.Retry(func() error {
		db = redis.NewClient(&redis.Options{
			Addr: fmt.Sprintf("localhost:%s", resource.GetPort("6379/tcp")),
		})
		intPort, _ := strconv.Atoi(resource.GetPort("6379/tcp"))
		config.Port = intPort
		return db.Ping().Err()
	}); err != nil {
		t.Fatalf("Could not connect to Redis docker: %s", err)
	}

	config.RedisKeyPrefix = "test-"
	storage.Init(config)
	storage.Connect()

	t.Run("set_key_without_exp", func(t *testing.T) {
		storage.SetKey("test-key", "test-result", 0)

		result, err := db.Get("test-test-key").Result()
		if err != nil {
			t.Fatalf("Error getting the key: %s", err)
		}

		if result != "test-result" {
			t.Fatalf("Key stored on redis should be 'test-result'")
		}
	})

	t.Run("set_key_with_exp", func(t *testing.T) {
		storage.SetKey("test-key-2", "test-result", 10)

		result, err := db.Get("test-test-key-2").Result()
		if err != nil {
			t.Fatalf("Error getting the key: %s", err)
		}

		if result != "test-result" {
			t.Fatalf("Key stored on redis should be 'test-result'")
		}
		expiration, _ := db.TTL("test-test-key-2").Result()

		if expiration == 0 {
			t.Fatal("test-test-key-2 should expires.")
		}
	})

}

func TestSetExp(t *testing.T) {
	resource, found := testPool.ContainerByName(redisDockerImage)
	if !found {
		t.Fatalf("Redis resource not found")
	}

	config := RedisStorageConfig{}
	storage := RedisClusterStorageManager{}
	config.Host = "localhost"
	var db *redis.Client
	if err := testPool.Retry(func() error {
		db = redis.NewClient(&redis.Options{
			Addr: fmt.Sprintf("localhost:%s", resource.GetPort("6379/tcp")),
		})
		intPort, _ := strconv.Atoi(resource.GetPort("6379/tcp"))
		config.Port = intPort
		return db.Ping().Err()
	}); err != nil {
		t.Fatalf("Could not connect to Redis docker: %s", err)
	}

	config.RedisKeyPrefix = "test-"
	storage.Init(config)
	storage.Connect()

	storage.SetKey("test-key", "test-result", 0)

	storage.SetExp("test-key", 10)
	expiration, _ := db.TTL("test-test-key").Result()

	if expiration == 0 {
		t.Fatal("test-test-key should expire.")
	}

}
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

	t.Run("Default addresses", func(t *testing.T) {
		opts := &RedisOpts{}
		simpleOpts := opts.simple()

		if simpleOpts.Addr != "127.0.0.1:6379" {
			t.Fatal("Wrong default single node address")
		}

		opts.Addrs = []string{}
		clusterOpts := opts.cluster()

		if clusterOpts.Addrs[0] != "127.0.0.1:6379" || len(clusterOpts.Addrs) != 1 {
			t.Fatal("Wrong default cluster mode address")
		}

		opts.Addrs = []string{}
		failoverOpts := opts.failover()

		if failoverOpts.SentinelAddrs[0] != "127.0.0.1:26379" || len(failoverOpts.SentinelAddrs) != 1 {
			t.Fatal("Wrong default sentinel mode address")
		}
	})
}

func TestRedisConnect(t *testing.T) {
	resource, found := testPool.ContainerByName(redisDockerImage)
	if !found {
		t.Fatalf("Redis resource not found")
	}

	config := RedisStorageConfig{}
	storage := RedisClusterStorageManager{}
	config.Host = "localhost"
	var db *redis.Client
	if err := testPool.Retry(func() error {
		db = redis.NewClient(&redis.Options{
			Addr: fmt.Sprintf("localhost:%s", resource.GetPort("6379/tcp")),
		})
		intPort, _ := strconv.Atoi(resource.GetPort("6379/tcp"))
		config.Port = intPort
		return db.Ping().Err()
	}); err != nil {
		t.Fatalf("Could not connect to Redis docker: %s", err)
	}

	storage.Init(config)
	storage.Connect()

	if storage.db == nil {
		t.Fatal("Redis should connect")
	}
	dbAddr := &storage.db
	storage.Connect()
	if &storage.db != dbAddr {
		t.Fatal("It should be the same Redis db")
	}
}

func TestRedisInit(t *testing.T) {
	config := RedisStorageConfig{}
	storage := RedisClusterStorageManager{}

	storage.Init(config)

	if storage.KeyPrefix != RedisKeyPrefix {
		t.Fatal("if not keyPrefix is specified, it must be ", RedisKeyPrefix)
	}
	config.RedisKeyPrefix = "test"
	storage.Init(config)
	if storage.KeyPrefix != "test" {
		t.Fatal("it should be the KeyPrefix specified")
	}

	os.Setenv(ENV_REDIS_PREFIX+"_REDISKEYPREFIX", "test_new")
	defer os.Setenv(ENV_REDIS_PREFIX+"_KEYPREFIX", "")

	storage.Init(config)

	if storage.KeyPrefix != "test_new" {
		t.Fatal("Env variables should have more priority than config.", storage.KeyPrefix)
	}
}

func saveTestRedisRecord(t *testing.T, db *redis.Client, record interface{}) {
	recordsBuffer := make([][]byte, 0, 10000)

	encodedRecord, errMarshal := msgpack.Marshal(record)
	if errMarshal != nil {
		fmt.Println("errMarshal:", errMarshal)
	}
	recordsBuffer = append(recordsBuffer, encodedRecord)

	pipe := db.Pipeline()
	for _, val := range recordsBuffer {
		pipe.RPush("analytics-tyk-system-analytics", val)
	}
	if _, errExec := pipe.Exec(); errExec != nil {
		t.Fatal("There was a problem saving analytic record in redis.")
	}
}
