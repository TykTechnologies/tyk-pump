package pumps

import (
	"context"
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

// Verifies: SW-REQ-034
func startSharedMongo(ctx context.Context) (string, error) {
	mongoOnce.Do(func() {
		c, err := tcmongodb.Run(ctx, "mongo:7-jammy")
		if err != nil {
			mongoErr = err
			return
		}
		mongoC = c
		uri, err := c.ConnectionString(ctx)
		if err != nil {
			mongoErr = err
			return
		}
		mongoURI = uri
	})
	return mongoURI, mongoErr
}

// Verifies: SW-REQ-034
func mongoConnectionURI(t *testing.T) string {
	t.Helper()
	uri, err := startSharedMongo(t.Context())
	if err != nil {
		if isDockerUnavailableErr(err) {
			t.Skipf("Docker not available for testcontainer mongo: %v", err)
		}
		t.Fatalf("failed to start testcontainer mongo: %v", err)
	}
	return uri
}

// Verifies: SW-REQ-019
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
			mysqlErr = err
			return
		}
		mysqlDSN = dsn
	})
	return mysqlDSN, mysqlErr
}

// Verifies: SW-REQ-019
func mysqlConnectionDSN(t *testing.T) string {
	t.Helper()
	dsn, err := startSharedMySQL(t.Context())
	if err != nil {
		if isDockerUnavailableErr(err) {
			t.Skipf("Docker not available for testcontainer mysql: %v", err)
		}
		t.Fatalf("failed to start testcontainer mysql: %v", err)
	}
	return dsn
}

// Verifies: SW-REQ-019
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
			postgresErr = err
			return
		}
		postgresDSN = dsn
	})
	return postgresDSN, postgresErr
}

// Verifies: SW-REQ-019
func postgresConnectionDSN(t *testing.T) string {
	t.Helper()
	dsn, err := startSharedPostgres(t.Context())
	if err != nil {
		if isDockerUnavailableErr(err) {
			t.Skipf("Docker not available for testcontainer postgres: %v", err)
		}
		t.Fatalf("failed to start testcontainer postgres: %v", err)
	}
	return dsn
}

// Verifies: SW-REQ-021
func startSharedKafka(ctx context.Context) ([]string, error) {
	kafkaOnce.Do(func() {
		c, err := tckafka.Run(ctx, "confluentinc/confluent-local:7.5.0")
		if err != nil {
			kafkaErr = err
			return
		}
		kafkaC = c
		brokers, err := c.Brokers(ctx)
		if err != nil {
			kafkaErr = err
			return
		}
		kafkaBrokers = brokers
	})
	return kafkaBrokers, kafkaErr
}

// Verifies: SW-REQ-021
func kafkaBrokerAddrs(t *testing.T) []string {
	t.Helper()
	brokers, err := startSharedKafka(t.Context())
	if err != nil {
		if isDockerUnavailableErr(err) {
			t.Skipf("Docker not available for testcontainer kafka: %v", err)
		}
		t.Fatalf("failed to start testcontainer kafka: %v", err)
	}
	return brokers
}

// Verifies: SW-REQ-022
//
// Cap the JVM heap at 256MB (default is 1GB) — the pump test fixtures index a
// handful of documents at most; a larger heap is pure memory tax. discovery.type
// stays at single-node (the testcontainers module sets it by default).
func startSharedElastic(ctx context.Context) (string, error) {
	elasticOnce.Do(func() {
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

// Verifies: SW-REQ-022
func elasticsearchURL(t *testing.T) string {
	t.Helper()
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
//
// Verifies: SW-REQ-034
func terminateSharedContainers() {
	ctx := context.Background()
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

// Verifies: SW-REQ-034
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

// Keep testcontainers reference alive for future helper additions.
var _ = testcontainers.WithLogger
