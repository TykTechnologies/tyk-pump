package pumps

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	tcelasticsearch "github.com/testcontainers/testcontainers-go/modules/elasticsearch"
	tckafka "github.com/testcontainers/testcontainers-go/modules/kafka"
	tcmongodb "github.com/testcontainers/testcontainers-go/modules/mongodb"
	tcmysql "github.com/testcontainers/testcontainers-go/modules/mysql"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// Shared per-process containers, one-shot init for fast reuse across tests.
//
// Each helper:
//   - uses sync.Once so only ONE container of each kind starts per `go test` process
//   - prefers the smallest viable image variant (alpine/jammy slim) to minimise
//     memory + image-pull time
//   - caps memory-hungry runtimes (ES JVM heap pinned at 256MB) so a developer
//     box running the full pumps suite doesn't OOM
//   - registers a TestMain shutdown via TestMain() below so containers are
//     terminated even if individual tests fail or panic
//
// Resource budget (with all 5 containers up at the same time):
//   mongo:7-jammy      ~150MB
//   mysql:8-oracle     ~400MB
//   postgres:15-alpine ~120MB
//   confluent-local    ~600MB
//   elasticsearch (Xms=Xmx=256MB)  ~512MB
//                                ─────
//   total              ~1.8GB
//
// Previously: ES alone defaulted to a 1GB heap; the matrix could exceed 5GB.

var (
	mongoOnce sync.Once
	mongoURI  string
	mongoErr  error
	mongoC    *tcmongodb.MongoDBContainer

	mysqlOnce sync.Once
	mysqlDSN  string
	mysqlErr  error
	mysqlC    *tcmysql.MySQLContainer

	postgresOnce sync.Once
	postgresDSN  string
	postgresErr  error
	postgresC    *tcpostgres.PostgresContainer

	kafkaOnce    sync.Once
	kafkaBrokers []string
	kafkaErr     error
	kafkaC       *tckafka.KafkaContainer

	elasticOnce sync.Once
	elasticURL  string
	elasticErr  error
	elasticC    *tcelasticsearch.ElasticsearchContainer
)

func startSharedMongo(ctx context.Context) (string, error) {
	mongoOnce.Do(func() {
		if uri := testEnv("TYK_TEST_MONGO"); uri != "" {
			mongoURI = uri
			return
		}
		c, err := tcmongodb.Run(ctx, "mongo:7-jammy")
		if err != nil {
			mongoErr = err
			return
		}
		mongoC = c
		uri, err := c.ConnectionString(ctx)
		if err != nil {
			_ = c.Terminate(context.Background())
			mongoC = nil
			mongoErr = err
			return
		}
		mongoURI = uri
	})
	return mongoURI, mongoErr
}

func mongoConnectionURI(t *testing.T) string {
	t.Helper()
	if testing.Short() && testEnv("TYK_TEST_MONGO") == "" {
		t.Skip("skipping mongo testcontainer in short mode")
	}
	if testEnv("TYK_TEST_MONGO") == "" && mongoC == nil && mongoURI == "" && mongoErr == nil {
		requireTestcontainerMemory(t, "mongo")
	}
	uri, err := startSharedMongo(t.Context())
	if err != nil {
		if isDockerUnavailableErr(err) {
			t.Skipf("Docker not available for testcontainer mongo: %v", err)
		}
		t.Fatalf("failed to start testcontainer mongo: %v", err)
	}
	return uri
}

func startSharedMySQL(ctx context.Context) (string, error) {
	mysqlOnce.Do(func() {
		c, err := tcmysql.Run(ctx, "mysql:8-oracle",
			tcmysql.WithDatabase("tyk_pump_test"),
			tcmysql.WithUsername("tyk"),
			tcmysql.WithPassword("tyk"),
		)
		if err != nil {
			mysqlErr = err
			return
		}
		mysqlC = c
		dsn, err := c.ConnectionString(ctx, "parseTime=true&multiStatements=true")
		if err != nil {
			_ = c.Terminate(context.Background())
			mysqlC = nil
			mysqlErr = err
			return
		}
		mysqlDSN = dsn
	})
	return mysqlDSN, mysqlErr
}

func mysqlConnectionDSN(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping mysql testcontainer in short mode")
	}
	if mysqlC == nil && mysqlDSN == "" && mysqlErr == nil {
		requireTestcontainerMemory(t, "mysql")
	}
	dsn, err := startSharedMySQL(t.Context())
	if err != nil {
		if isDockerUnavailableErr(err) {
			t.Skipf("Docker not available for testcontainer mysql: %v", err)
		}
		t.Fatalf("failed to start testcontainer mysql: %v", err)
	}
	return dsn
}

func startSharedPostgres(ctx context.Context) (string, error) {
	postgresOnce.Do(func() {
		c, err := tcpostgres.Run(ctx, "postgres:15-alpine",
			tcpostgres.WithDatabase("tyk_pump_test"),
			tcpostgres.WithUsername("tyk"),
			tcpostgres.WithPassword("tyk"),
			tcpostgres.BasicWaitStrategies(),
		)
		if err != nil {
			postgresErr = err
			return
		}
		postgresC = c
		dsn, err := c.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			_ = c.Terminate(context.Background())
			postgresC = nil
			postgresErr = err
			return
		}
		postgresDSN = dsn
	})
	return postgresDSN, postgresErr
}

func postgresConnectionDSN(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping postgres testcontainer in short mode")
	}
	if postgresC == nil && postgresDSN == "" && postgresErr == nil {
		requireTestcontainerMemory(t, "postgres")
	}
	dsn, err := startSharedPostgres(t.Context())
	if err != nil {
		if isDockerUnavailableErr(err) {
			t.Skipf("Docker not available for testcontainer postgres: %v", err)
		}
		t.Fatalf("failed to start testcontainer postgres: %v", err)
	}
	return dsn
}

func startSharedKafka(ctx context.Context) ([]string, error) {
	kafkaOnce.Do(func() {
		if brokers := kafkaBrokersFromEnv(); len(brokers) > 0 {
			kafkaBrokers = brokers
			return
		}
		c, err := tckafka.Run(ctx, "confluentinc/confluent-local:7.5.0")
		if err != nil {
			kafkaErr = err
			return
		}
		kafkaC = c
		brokers, err := c.Brokers(ctx)
		if err != nil {
			_ = c.Terminate(context.Background())
			kafkaC = nil
			kafkaErr = err
			return
		}
		kafkaBrokers = brokers
	})
	return kafkaBrokers, kafkaErr
}

func kafkaBrokerAddrs(t *testing.T) []string {
	t.Helper()
	if testing.Short() && len(kafkaBrokersFromEnv()) == 0 {
		t.Skip("skipping kafka testcontainer in short mode")
	}
	if len(kafkaBrokersFromEnv()) == 0 && kafkaC == nil && len(kafkaBrokers) == 0 && kafkaErr == nil {
		requireTestcontainerMemory(t, "kafka")
	}
	brokers, err := startSharedKafka(t.Context())
	if err != nil {
		if isDockerUnavailableErr(err) {
			t.Skipf("Docker not available for testcontainer kafka: %v", err)
		}
		t.Fatalf("failed to start testcontainer kafka: %v", err)
	}
	return brokers
}

// Cap the JVM heap at 256MB (default is 1GB) — the pump test fixtures index a
// handful of documents at most; a larger heap is pure memory tax. discovery.type
// stays at single-node (the testcontainers module sets it by default).
func startSharedElastic(ctx context.Context) (string, error) {
	elasticOnce.Do(func() {
		if url := testEnv("TYK_TEST_ELASTICSEARCH_URL"); url != "" {
			elasticURL = url
			return
		}
		c, err := tcelasticsearch.Run(ctx,
			"docker.elastic.co/elasticsearch/elasticsearch:7.17.27",
			testcontainers.WithEnv(map[string]string{
				"ES_JAVA_OPTS": "-Xms256m -Xmx256m",
			}),
		)
		if err != nil {
			elasticErr = err
			return
		}
		elasticC = c
		elasticURL = c.Settings.Address
	})
	return elasticURL, elasticErr
}

func elasticsearchURL(t *testing.T) string {
	t.Helper()
	if testing.Short() && testEnv("TYK_TEST_ELASTICSEARCH_URL") == "" {
		t.Skip("skipping elasticsearch testcontainer in short mode")
	}
	if testEnv("TYK_TEST_ELASTICSEARCH_URL") == "" && elasticC == nil && elasticURL == "" && elasticErr == nil {
		requireTestcontainerMemory(t, "elasticsearch")
	}
	url, err := startSharedElastic(t.Context())
	if err != nil {
		if isDockerUnavailableErr(err) {
			t.Skipf("Docker not available for testcontainer elasticsearch: %v", err)
		}
		t.Fatalf("failed to start testcontainer elasticsearch: %v", err)
	}
	return url
}

// terminateSharedContainers shuts down every spun-up container exactly once.
// Wired from TestMain so containers die even on panic / test failure /
// SIGINT mid-run; testcontainers' Reaper handles the daemon-survives-crash
// case but TestMain handles the in-process clean-exit case far faster.
func terminateSharedContainers() {
	ctx := context.Background()
	terminateReusableDedicatedMongo()
	if mongoC != nil {
		_ = mongoC.Terminate(ctx)
	}
	if mysqlC != nil {
		_ = mysqlC.Terminate(ctx)
	}
	if postgresC != nil {
		_ = postgresC.Terminate(ctx)
	}
	if kafkaC != nil {
		_ = kafkaC.Terminate(ctx)
	}
	if elasticC != nil {
		_ = elasticC.Terminate(ctx)
	}
}

func isDockerUnavailableErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Cannot connect to the Docker daemon") ||
		strings.Contains(msg, "Cannot connect to docker") ||
		strings.Contains(msg, "docker daemon") ||
		strings.Contains(msg, "rootless Docker not found")
}

func testEnv(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

func kafkaBrokersFromEnv() []string {
	value := testEnv("TYK_TEST_KAFKA_BROKERS")
	if value == "" {
		return nil
	}
	var brokers []string
	for _, broker := range strings.Split(value, ",") {
		if broker = strings.TrimSpace(broker); broker != "" {
			brokers = append(brokers, broker)
		}
	}
	return brokers
}

func requireTestcontainerMemory(t *testing.T, name string) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		return
	}
	minMiB := testcontainerMinFreeMiB()
	if minMiB <= 0 {
		return
	}
	freeMiB, err := macFreePlusSpeculativeMiB()
	if err != nil || freeMiB >= minMiB {
		return
	}
	t.Skipf("skipping %s testcontainer: macOS free+speculative memory is %d MiB, below %d MiB; set TYK_TESTCONTAINERS_MIN_FREE_MIB=0 to override", name, freeMiB, minMiB)
}

func testcontainerMinFreeMiB() int {
	const defaultMiB = 1024
	value := strings.TrimSpace(os.Getenv("TYK_TESTCONTAINERS_MIN_FREE_MIB"))
	if value == "" {
		return defaultMiB
	}
	minMiB, err := strconv.Atoi(value)
	if err != nil {
		return defaultMiB
	}
	return minMiB
}

func macFreePlusSpeculativeMiB() (int, error) {
	out, err := exec.Command("vm_stat").Output()
	if err != nil {
		return 0, err
	}
	pageSize := int64(4096)
	var freePages, speculativePages int64
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if strings.Contains(line, "page size of") {
			for i, field := range fields {
				if field == "of" && i+1 < len(fields) {
					if parsed, err := strconv.ParseInt(strings.Trim(fields[i+1], ".)"), 10, 64); err == nil {
						pageSize = parsed
					}
					break
				}
			}
			continue
		}
		if len(fields) < 3 {
			continue
		}
		pages, err := strconv.ParseInt(strings.TrimRight(fields[len(fields)-1], "."), 10, 64)
		if err != nil {
			continue
		}
		switch {
		case strings.HasPrefix(line, "Pages free:"):
			freePages = pages
		case strings.HasPrefix(line, "Pages speculative:"):
			speculativePages = pages
		}
	}
	return int((freePages + speculativePages) * pageSize / 1048576), nil
}

// Keep testcontainers reference alive for future helper additions.
var _ = testcontainers.WithLogger
