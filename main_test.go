package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/TykTechnologies/tyk-pump/config"
	"github.com/TykTechnologies/tyk-pump/instrumentation"
	"github.com/go-redis/redis"
	"gopkg.in/mgo.v2"
	"gopkg.in/vmihailenco/msgpack.v2"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/pumps"
	"github.com/ory/dockertest/v3"
)

const (
	mongoDockerImage = "mongo"
	redisDockerImage = "redis"
)

var (
	testPool *dockertest.Pool
)

type MockedPump struct {
	CounterRequest  int
	filters         analytics.AnalyticsFilters
	timeout         int
	activateTimeout bool
	hangingTime     int
	name            string
	shouldErr       bool
}

func (p *MockedPump) GetName() string {
	return p.name
}

func (p *MockedPump) New() pumps.Pump {
	return &MockedPump{}
}
func (p *MockedPump) Init(config interface{}) error {
	return nil
}
func (p *MockedPump) WriteData(ctx context.Context, keys []interface{}) error {
	if p.shouldErr {
		return errors.New("Pump error")
	} else if p.activateTimeout {
		time.Sleep(time.Duration(p.timeout+1) * time.Second)
		return errors.New("timeout")
	} else if p.hangingTime > 0 {
		time.Sleep(time.Duration(p.hangingTime) * time.Second)
	}
	for _, key := range keys {
		if key != nil {
			p.CounterRequest++
		}
	}
	return nil
}

func (p *MockedPump) SetFilters(filters analytics.AnalyticsFilters) {
	p.filters = filters
}
func (p *MockedPump) GetFilters() analytics.AnalyticsFilters {
	return p.filters
}
func (p *MockedPump) SetTimeout(timeout int) {
	p.timeout = timeout
}
func (p *MockedPump) GetTimeout() int {
	return p.timeout
}

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

	//Mongo resource
	if _, found := testPool.ContainerByName(mongoDockerImage); !found {
		_, err = testPool.RunWithOptions(&dockertest.RunOptions{Name: mongoDockerImage, Repository: mongoDockerImage, Tag: "3.0", Env: nil})
		if err != nil {
			log.Fatalf("Could not start resource: %s", err)
		}
	}

	fmt.Printf("\033[1;36m%s\033[0m", "> Setup completed\n")
}

func teardown() {
	resourceRedis, _ := testPool.ContainerByName(redisDockerImage)
	testPool.Purge(resourceRedis)

	resourceMongo, _ := testPool.ContainerByName(mongoDockerImage)
	testPool.Purge(resourceMongo)
	fmt.Printf("\033[1;36m%s\033[0m", "> Teardown completed")
	fmt.Printf("\n")
}

//Tests for filterData
func TestFilterData(t *testing.T) {

	mockedPump := &MockedPump{}

	mockedPump.SetFilters(
		analytics.AnalyticsFilters{
			APIIDs: []string{"api123"},
		},
	)

	keys := make([]interface{}, 3)
	keys[0] = analytics.AnalyticsRecord{APIID: "api111"}
	keys[1] = analytics.AnalyticsRecord{APIID: "api123"}
	keys[2] = analytics.AnalyticsRecord{APIID: "api321"}

	filteredKeys := filterData(mockedPump, keys)
	if len(keys) == len(filteredKeys) {
		t.Fatal("keys and filtered keys have the  same lenght")
	}

}

//Tests for writeToPumps and execPumpWriting
func TestWriteData(t *testing.T) {
	mockedPump := &MockedPump{}
	Pumps = []pumps.Pump{mockedPump}

	keys := make([]interface{}, 3)
	keys[0] = analytics.AnalyticsRecord{APIID: "api111"}
	keys[1] = analytics.AnalyticsRecord{APIID: "api123"}
	keys[2] = analytics.AnalyticsRecord{APIID: "api321"}

	job := instrumentation.Instrument.NewJob("TestJob")

	writeToPumps(keys, job, time.Now(), 2)

	mockedPump = Pumps[0].(*MockedPump)

	if mockedPump.CounterRequest != 3 {
		t.Fatal("MockedPump should have 3 requests")
	}

}
func TestWriteDataWithTimeout(t *testing.T) {
	var writer bytes.Buffer
	log.Out = &writer

	mockedPumpTimeouting := &MockedPump{}
	mockedPumpTimeouting.name = "MockedPumpTimeouting"
	mockedPumpTimeouting.timeout = 1
	mockedPumpTimeouting.activateTimeout = true

	mockedPumpSlow := &MockedPump{}
	mockedPumpSlow.name = "MockedPumpSlow"
	mockedPumpSlow.hangingTime = 2

	mockedPumpSlowWithTimeout := &MockedPump{}
	mockedPumpSlowWithTimeout.name = "mockedPumpSlowWithTimeout"
	mockedPumpSlowWithTimeout.timeout = 3
	mockedPumpSlowWithTimeout.hangingTime = 2

	mockedPumpErr := &MockedPump{}
	mockedPumpErr.name = "mockedPumpErr"
	mockedPumpErr.shouldErr = true

	Pumps = []pumps.Pump{mockedPumpTimeouting, mockedPumpSlow, mockedPumpSlowWithTimeout, mockedPumpErr}

	keys := make([]interface{}, 3)
	keys[0] = analytics.AnalyticsRecord{APIID: "api111"}
	keys[1] = analytics.AnalyticsRecord{APIID: "api123"}
	keys[2] = analytics.AnalyticsRecord{APIID: "api321"}

	job := instrumentation.Instrument.NewJob("TestJob")

	writeToPumps(keys, job, time.Now(), 1)

	//Test mockedPumpTimeouting
	if mockedPumpTimeouting.CounterRequest != 0 {
		t.Fatal("MockedPumpTimeouting should have 0 records")
	}
	if !findInOutput(writer, "Timeout Writing to: MockedPumpTimeouting") {
		t.Fatal("MockedPumpTimeouting should throw 'Timeout Writing to: MockedPumpTimeouting' log.")
	}

	//Test mockedPumpSlow
	if mockedPumpSlow.CounterRequest != 3 {
		t.Fatal("MockedPumpSlow  should have 3 records")
	}
	if !findInOutput(writer, "Pump  MockedPumpSlow is taking more time than the value configured of purge_delay. You should try to set a timeout for this pump.") {
		t.Fatal("MockedPumpSlow should throw 'Pump  MockedPumpSlow is taking more time than the value configured of purge_delay. You should try to set a timeout for this pump.' log.")
	}

	//Test mockedPumpSlowWithTimeout
	if mockedPumpSlowWithTimeout.CounterRequest != 3 {
		t.Fatal("mockedPumpSlowWithTimeout  should have 3 records")
	}
	if !findInOutput(writer, "Pump  mockedPumpSlowWithTimeout is taking more time than the value configured of purge_delay. You should try lowering the timeout configured for this pump") {
		t.Fatal("mockedPumpSlowWithTimeout should throw 'Pump  mockedPumpSlowWithTimeout is taking more time than the value configured of purge_delay. You should try lowering the timeout configured for this pump.' log.")
	}

	if mockedPumpErr.CounterRequest != 0 {
		t.Fatal("mockedPumpErr should have 0 records")
	}
	if !findInOutput(writer, "Error Writing to: mockedPumpErr - Error:Pump error") {
		t.Fatal("mockedPumpErr shoudl throw 'Error Writing to: mockedPumpErr - Error:Pump error' log.")
	}
}
func TestWriteDataWithFilters(t *testing.T) {
	mockedPump := &MockedPump{}
	mockedPump.SetFilters(
		analytics.AnalyticsFilters{
			APIIDs: []string{"api123"},
		},
	)

	Pumps = []pumps.Pump{mockedPump}

	keys := make([]interface{}, 3)
	keys[0] = analytics.AnalyticsRecord{APIID: "api111"}
	keys[1] = analytics.AnalyticsRecord{APIID: "api123"}
	keys[2] = analytics.AnalyticsRecord{APIID: "api321"}

	job := instrumentation.Instrument.NewJob("TestJob")

	writeToPumps(keys, job, time.Now(), 2)

	mockedPump = Pumps[0].(*MockedPump)

	if mockedPump.CounterRequest != 1 {
		fmt.Println(mockedPump.CounterRequest)
		t.Fatal("MockedPump with filter should have 3 requests")
	}
}
func TestWriteDataWithoutPumps(t *testing.T) {
	var writer bytes.Buffer
	log.Out = &writer
	Pumps = nil
	keys := make([]interface{}, 3)
	keys[0] = analytics.AnalyticsRecord{APIID: "api111"}
	keys[1] = analytics.AnalyticsRecord{APIID: "api123"}
	keys[2] = analytics.AnalyticsRecord{APIID: "api321"}

	job := instrumentation.Instrument.NewJob("TestJob")

	writeToPumps(keys, job, time.Now(), 1)

	if !findInOutput(writer, "No pumps defined") {
		t.Fatal("It should log 'No pumps defined'.")
	}
}

//Tests for initialisePumps
func TestInitialisePumps(t *testing.T) {
	var writer bytes.Buffer
	log.Out = &writer

	SystemConfig = config.TykPumpConfiguration{}
	SystemConfig.DontPurgeUptimeData = true
	SystemConfig.Pumps = make(map[string]config.PumpConfig, 4)
	SystemConfig.Pumps["DummyPump"] = config.PumpConfig{Name: "DummyPump", Type: "dummy"}
	SystemConfig.Pumps["InvalidPump"] = config.PumpConfig{Name: "InvalidPump", Type: "InvalidPump"}
	SystemConfig.Pumps["dummy"] = config.PumpConfig{Name: "DummyWithoutType"}
	SystemConfig.Pumps["SplunkWithoutMeta"] = config.PumpConfig{Name: "SplunkWithoutMeta", Type: "splunk"}

	initialisePumps()

	if len(Pumps) != 2 {
		t.Fatal("Should be only 1 pump initialised.")
	}
	if !findInOutput(writer, "Pump init error") {
		t.Fatal("It should log 'Pump init error' for splunk pump.")
	}
	if !findInOutput(writer, "Pump load error") {
		t.Fatal("It should log 'Pump load error' for invalid pump.")
	}
}
func TestInitialisePumpsWithUptimeData(t *testing.T) {
	var db *mgo.Session
	var writer bytes.Buffer
	log.Out = &writer

	SystemConfig = config.TykPumpConfiguration{}
	SystemConfig.DontPurgeUptimeData = false
	SystemConfig.UptimePumpConfig = make(map[string]interface{})
	SystemConfig.UptimePumpConfig["collection_name"] = "tyk_uptime_analytics"

	resource, found := testPool.ContainerByName(mongoDockerImage)
	if !found {
		log.Fatalf("Mongo resource not found")
	}

	if err := testPool.Retry(func() error {
		var err error
		db, err = mgo.Dial(fmt.Sprintf("localhost:%s", resource.GetPort("27017/tcp")))
		SystemConfig.UptimePumpConfig["mongo_url"] = fmt.Sprintf("localhost:%s", resource.GetPort("27017/tcp"))
		if err != nil {
			return err
		}
		return db.Ping()
	}); err != nil {
		log.Fatalf("Could not connect to Mongo docker: %s", err)
	}

	initialisePumps()

	if !findInOutput(writer, "Init Uptime Pump") {
		t.Fatal("Uptime pump should be initialised.")
	}
}

//Testing for store version
func TestStoreVersion(t *testing.T) {
	resource, found := testPool.ContainerByName(redisDockerImage)
	if !found {
		log.Fatalf("Redis resource not found")
	}

	SystemConfig = config.TykPumpConfiguration{}
	SystemConfig.AnalyticsStorageType = "redis"
	SystemConfig.AnalyticsStorageConfig.Host = "localhost"
	var db *redis.Client
	if err := testPool.Retry(func() error {
		db = redis.NewClient(&redis.Options{
			Addr: fmt.Sprintf("localhost:%s", resource.GetPort("6379/tcp")),
		})
		intPort, _ := strconv.Atoi(resource.GetPort("6379/tcp"))
		SystemConfig.AnalyticsStorageConfig.Port = intPort
		return db.Ping().Err()
	}); err != nil {
		log.Fatalf("Could not connect to Redis docker: %s", err)
	}

	storeVersion()

	result, err := db.Get("version-check-pump").Result()
	if err != nil {
		t.Fatalf("Error getting version key: %s", err)
	}
	if result != VERSION {
		t.Fatalf("Version stored on redis should be %s", VERSION)
	}
}

//Testing setupAnalyticsStore
func TestSetupAnalyticsStoreTypeRedis(t *testing.T) {

	resource, found := testPool.ContainerByName(redisDockerImage)
	if !found {
		log.Fatalf("Redis resource not found")
	}

	SystemConfig = config.TykPumpConfiguration{}
	SystemConfig.AnalyticsStorageType = "redis"
	SystemConfig.AnalyticsStorageConfig.Host = "localhost"
	var db *redis.Client
	if err := testPool.Retry(func() error {
		db = redis.NewClient(&redis.Options{
			Addr: fmt.Sprintf("localhost:%s", resource.GetPort("6379/tcp")),
		})
		intPort, _ := strconv.Atoi(resource.GetPort("6379/tcp"))
		SystemConfig.AnalyticsStorageConfig.Port = intPort
		return db.Ping().Err()
	}); err != nil {
		log.Fatalf("Could not connect to Redis docker: %s", err)
	}

	setupAnalyticsStore()

	if AnalyticsStore.GetName() != "redis" || AnalyticsStore.Connect() != true {
		t.Fatal("AnalyticStore should be redis and be already connected")
	}
	if UptimeStorage.GetName() != "redis" || UptimeStorage.Connect() != true {
		t.Fatal("UptimeStorage should be redis and be already connected")
	}

	//default options
	SystemConfig.AnalyticsStorageType = ""
	setupAnalyticsStore()
	if AnalyticsStore.GetName() != "redis" || AnalyticsStore.Connect() != true {
		t.Fatal("AnalyticStore should be redis and be already connected")
	}
	if UptimeStorage.GetName() != "redis" || UptimeStorage.Connect() != true {
		t.Fatal("UptimeStorage should be redis and be already connected")
	}

}

//Testing Init
func TestInit(t *testing.T) {

}

//Testing PurgeLoop
func TestPurgeLoop(t *testing.T) {
	resource, found := testPool.ContainerByName(redisDockerImage)
	if !found {
		log.Fatalf("Redis resource not found")
	}

	SystemConfig = config.TykPumpConfiguration{}
	SystemConfig.AnalyticsStorageType = "redis"
	SystemConfig.AnalyticsStorageConfig.Host = "localhost"
	SystemConfig.DontPurgeUptimeData = true

	var db *redis.Client
	if err := testPool.Retry(func() error {
		db = redis.NewClient(&redis.Options{
			Addr: fmt.Sprintf("localhost:%s", resource.GetPort("6379/tcp")),
		})
		intPort, _ := strconv.Atoi(resource.GetPort("6379/tcp"))
		SystemConfig.AnalyticsStorageConfig.Port = intPort
		return db.Ping().Err()
	}); err != nil {
		log.Fatalf("Could not connect to Redis docker: %s", err)
	}

	if AnalyticsStore == nil {
		setupAnalyticsStore()
	}

	record := analytics.AnalyticsRecord{APIID: "api-1", OrgID: "org-1"}
	saveTestRedisRecord(t, db, record)

	mockedPump := &MockedPump{}

	Pumps = []pumps.Pump{mockedPump}
	purgeLoop(1)

	if mockedPump.CounterRequest != 1 {
		t.Fatal("mockedPump should have 1 request after purgeLoop and have:", mockedPump.CounterRequest)
	}

	recordMalformed := "{record-malformed.."
	saveTestRedisRecord(t, db, recordMalformed)
	purgeLoop(1)

	if mockedPump.CounterRequest != 1 {
		t.Fatal("mockedPump should still have 1 request after the second purgeLoop")
	}
}

func findInOutput(buffer bytes.Buffer, toFind string) bool {
	if strings.Contains(buffer.String(), toFind) {
		return true
	}
	return false
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
