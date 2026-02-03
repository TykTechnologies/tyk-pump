package pumps

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type CommonPumpConfig struct {
	filters               analytics.AnalyticsFilters
	timeout               int
	maxRecordSize         int
	OmitDetailedRecording bool
	log                   *logrus.Entry
	ignoreFields          []string
	decodeResponseBase64  bool
	decodeRequestBase64   bool
}

func (p *CommonPumpConfig) SetFilters(filters analytics.AnalyticsFilters) {
	p.filters = filters
}
func (p *CommonPumpConfig) GetFilters() analytics.AnalyticsFilters {
	return p.filters
}
func (p *CommonPumpConfig) SetTimeout(timeout int) {
	p.timeout = timeout
}

func (p *CommonPumpConfig) GetTimeout() int {
	return p.timeout
}

func (p *CommonPumpConfig) SetOmitDetailedRecording(OmitDetailedRecording bool) {
	p.OmitDetailedRecording = OmitDetailedRecording
}
func (p *CommonPumpConfig) GetOmitDetailedRecording() bool {
	return p.OmitDetailedRecording
}

func (p *CommonPumpConfig) GetEnvPrefix() string {
	return ""
}

func (p *CommonPumpConfig) Shutdown() error {
	return nil
}

func (p *CommonPumpConfig) SetMaxRecordSize(size int) {
	p.maxRecordSize = size
}

func (p *CommonPumpConfig) GetMaxRecordSize() int {
	return p.maxRecordSize
}

func (p *CommonPumpConfig) SetLogLevel(level logrus.Level) {
	p.log.Level = level
}

func (p *CommonPumpConfig) SetIgnoreFields(fields []string) {
	p.ignoreFields = fields
}

func (p *CommonPumpConfig) GetIgnoreFields() []string {
	return p.ignoreFields
}

func (p *CommonPumpConfig) SetDecodingResponse(decoding bool) {
	p.decodeResponseBase64 = decoding
}

func (p *CommonPumpConfig) SetDecodingRequest(decoding bool) {
	p.decodeRequestBase64 = decoding
}

func (p *CommonPumpConfig) GetDecodedRequest() bool {
	return p.decodeRequestBase64
}

func (p *CommonPumpConfig) GetDecodedResponse() bool {
	return p.decodeResponseBase64
}

// HandleTableMigration handles the table migration logic for SQL pumps
// It migrates either all sharded tables or just the current day's table based on configuration
func HandleTableMigration(db *gorm.DB, conf *SQLConf, tableName string, model interface{}, log *logrus.Entry, migrateAllFunc func() error) error {
	switch {
	case !conf.TableSharding:
		// Non-sharded case: migrate the main table
		if err := db.Table(tableName).AutoMigrate(model); err != nil {
			log.WithError(err).Error("error migrating table")
			return err
		}
	case conf.MigrateShardedTables:
		// Migrate all existing sharded tables
		if err := migrateAllFunc(); err != nil {
			log.WithError(err).Warn("Failed to migrate existing sharded tables")
			// Don't fail initialization, just log the warning
		}
	default:
		// Migrate current day's table to ensure it has latest schema
		currentDayTable := tableName + "_" + time.Now().Format("20060102")
		if err := db.Table(currentDayTable).AutoMigrate(model); err != nil {
			log.WithField("table", currentDayTable).WithError(err).Warn("Failed to migrate current day table")
			// Don't fail initialization, just log the warning
		} else {
			log.WithField("table", currentDayTable).Debug("Migrated current day table")
		}
	}
	return nil
}

// MigrateAllShardedTables is a generic function that migrates all existing sharded tables
// matching the given table prefix and model type
func MigrateAllShardedTables(db *gorm.DB, tablePrefix, logPrefix string, model interface{}, log *logrus.Entry) error {
	log.Info("Scanning for existing sharded " + logPrefix + " tables to migrate...")

	// Get all tables in the database
	var tables []string
	var err error

	// Use database-specific queries for better compatibility
	switch db.Dialector.Name() {
	case "sqlite":
		// For SQLite, use the mock information_schema.tables table (created in tests)
		err = db.Raw(`SELECT table_name FROM "information_schema.tables" WHERE table_schema = 'public'`).Scan(&tables).Error
	case "mysql":
		// For MySQL, use the database name as schema
		err = db.Raw(`SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE()`).Scan(&tables).Error
	case "postgres":
		// For PostgreSQL, use 'public' schema
		err = db.Raw(`SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'`).Scan(&tables).Error
	default:
		// Unknown database type
		log.WithField("dialector", db.Dialector.Name()).Error("Unsupported database type for table migration")
		return fmt.Errorf("unsupported database type: %s", db.Dialector.Name())
	}
	if err != nil {
		log.WithError(err).Warn("Failed to get list of tables, skipping migration scan")
		return nil
	}

	// Find tables matching our sharded pattern
	shardedTables := make([]string, 0)
	fullTablePrefix := tablePrefix + "_"

	for _, table := range tables {
		if strings.HasPrefix(table, fullTablePrefix) {
			// Check if it matches the date pattern (YYYYMMDD)
			suffix := strings.TrimPrefix(table, fullTablePrefix)
			if len(suffix) == 8 {
				// Try to parse as date to validate format
				if _, err := time.Parse("20060102", suffix); err == nil {
					shardedTables = append(shardedTables, table)
				}
			}
		}
	}

	log.WithField("count", len(shardedTables)).Info("Found sharded " + logPrefix + " tables to migrate")

	// Migrate each sharded table
	for _, tableName := range shardedTables {
		log.WithField("table", tableName).Debug("Migrating sharded " + logPrefix + " table")

		if err := db.Table(tableName).AutoMigrate(model); err != nil {
			log.WithField("table", tableName).WithError(err).Warn("Failed to migrate sharded " + logPrefix + " table")
			// Continue with other tables even if one fails
		} else {
			log.WithField("table", tableName).Debug("Successfully migrated sharded " + logPrefix + " table")
		}
	}

	log.Info("Completed migration of sharded " + logPrefix + " tables")
	return nil
}

type TLSConfig struct {
	CertFile           string
	KeyFile            string
	CAFile             string
	ServerName         string
	InsecureSkipVerify bool
}

// NewTLSConfig creates a TLS configuration from the provided settings.
func NewTLSConfig(cfg TLSConfig, log *logrus.Entry) (*tls.Config, error) {
	if log == nil {
		return nil, errors.New("logger cannot be nil")
	}

	// Backward compatibility: Some pumps are logging this configuration mismatch instead of returning an error.
	// The TLS config will still be created with available settings (e.g., CA cert only).
	if (cfg.CertFile == "") != (cfg.KeyFile == "") {
		log.Warn("Only one of ssl_cert_file and ssl_key_file configuration option is set, you should set both to enable mTLS.")
	}

	// #nosec G402
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		ServerName:         cfg.ServerName,
	}

	if tlsConfig.InsecureSkipVerify {
		log.Warn("ssl_insecure_skip_verify is set to true. Server certificate validation will be skipped.")
	}

	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load cert/key pair(cert: %q, key: %q): %w", cfg.CertFile, cfg.KeyFile, err)
		}

		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if cfg.CAFile != "" {
		caPem, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate file %q: %w", cfg.CAFile, err)
		}

		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(caPem) {
			return nil, fmt.Errorf("failed to parse CA certificate from file %q: invalid PEM data", cfg.CAFile)
		}

		tlsConfig.RootCAs = certPool

		if tlsConfig.InsecureSkipVerify {
			log.Warn("ssl_ca_file is set but ssl_insecure_skip_verify is true - server certificate will not be verified against the provided CA")
		}
	}

	return tlsConfig, nil
}
