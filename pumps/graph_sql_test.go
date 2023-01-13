package pumps

import (
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGraphSQLPump_Init(t *testing.T) {
	pump := &GraphSQLPump{}
	t.Run("successful", func(t *testing.T) {
		conf := SQLConf{
			Type:             "sqlite",
			ConnectionString: "",
			TableName:        "rand-table",
		}
		assert.NoError(t, pump.Init(conf))
		t.Cleanup(func() {
			pump.db.Migrator().DropTable("rand-table")
		})
		assert.True(t, pump.db.Migrator().HasTable(conf.TableName))
	})

	t.Run("invalid connection details", func(t *testing.T) {
		conf := SQLConf{
			Type:             "postgres",
			ConnectionString: "host=localhost user=gorm password=gorm DB.name=gorm port=9920 sslmode=disable",
		}
		assert.Error(t, pump.Init(conf))
	})

	t.Run("should fail", func(t *testing.T) {
		conf := SQLConf{ConnectionString: "random"}
		assert.ErrorContains(t, pump.Init(conf), "Unsupported `config_storage.type` value:")
	})

	t.Run("invalid config", func(t *testing.T) {
		conf := map[string]interface{}{
			"type": 1,
		}
		assert.ErrorContains(t, pump.Init(conf), "error decoding con")
	})

	t.Run("sharded table", func(t *testing.T) {
		conf := SQLConf{
			Type:             "sqlite",
			ConnectionString: "",
			TableName:        "test-table",
			TableSharding:    true,
		}
		assert.NoError(t, pump.Init(conf))
		assert.False(t, pump.db.Migrator().HasTable(conf.TableName))
	})
}

func TestGraphSQLPump_WriteData(t *testing.T) {
	type customRecord struct {
		isHttp       bool
		tags         []string
		responseCode int
	}
	testCases := []struct {
		name           string
		recordModifier func(record analytics.AnalyticsRecord) analytics.AnalyticsRecord
		records        []customRecord
	}{
		{
			name: "default case",
			records: []customRecord{
				{
					isHttp:       false,
					tags:         []string{analytics.PredefinedTagGraphAnalytics},
					responseCode: 200,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			records := make([]interface{}, 0)
			for _, cr := range tc.records {
				r := analytics.AnalyticsRecord{
					APIName: "Test API",
					Path:    "POST",
					//RawRequest:  base64.StdEncoding.EncodeToString([]byte(cr.rawRequest)),
					//RawResponse: base64.StdEncoding.EncodeToString([]byte(cr.rawResponse)),
					//ApiSchema:   base64.StdEncoding.EncodeToString([]byte(cr.schema)),
					Tags: cr.tags,
				}
				if cr.responseCode != 0 {
					r.ResponseCode = cr.responseCode
				}
				records = append(records, r)
			}
		})
	}
}
