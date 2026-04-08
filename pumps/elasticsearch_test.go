package pumps

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestElasticsearchPump_TLSConfig_ErrorCases(t *testing.T) {
	t.Run("should return wrapped error with invalid cert file", func(t *testing.T) {
		pump := &ElasticsearchPump{}
		pump.log = log.WithField("prefix", "test")
		pump.esConf = &ElasticsearchConf{
			ElasticsearchURL: "https://localhost:9200",
			IndexName:        "test",
			Version:          "7",
			UseSSL:           true,
			SSLCertFile:      "/nonexistent/cert.pem",
			SSLKeyFile:       "/nonexistent/key.pem",
		}

		operator, err := pump.getOperator()

		assert.Error(t, err)
		assert.Nil(t, operator)
		assert.Contains(t, err.Error(), "failed to configure TLS for Elasticsearch connection")
	})

	t.Run("should return wrapped error with invalid CA file", func(t *testing.T) {
		pump := &ElasticsearchPump{}
		pump.log = log.WithField("prefix", "test")
		pump.esConf = &ElasticsearchConf{
			ElasticsearchURL: "https://localhost:9200",
			IndexName:        "test",
			Version:          "7",
			UseSSL:           true,
			SSLCAFile:        "/nonexistent/ca.pem",
		}

		operator, err := pump.getOperator()

		assert.Error(t, err)
		assert.Nil(t, operator)
		assert.Contains(t, err.Error(), "failed to configure TLS for Elasticsearch connection")
	})
}
