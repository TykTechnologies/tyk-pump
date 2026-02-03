package pumps

import (
	"os"
	"testing"

	"github.com/kelseyhightower/envconfig"
	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
)

// While this test focuses on ssl_ca_file specifically, the same configuration precedence
// and environment variable override behavior applies to all ElasticsearchPump configuration
// fields that can be set via TYK_PMP_PUMPS_ELASTICSEARCH_META_* environment variables
func TestElasticsearchPump_Init_SSLCAFile(t *testing.T) {
	t.Run("environment variable overrides config file SSL CA setting", func(t *testing.T) {
		os.Setenv("TYK_PMP_PUMPS_ELASTICSEARCH_META_SSLCAFILE", "env_var_nonexistent_ca.pem")
		defer os.Unsetenv("TYK_PMP_PUMPS_ELASTICSEARCH_META_SSLCAFILE")

		config := map[string]any{
			"elasticsearch_url": "https://localhost:9200",
			"ssl_ca_file":       "conf_nonexistent_ca.pem",
		}

		esConf := &ElasticsearchConf{}

		err := mapstructure.Decode(config, esConf)
		assert.NoError(t, err)

		// Process env vars (this is what processPumpEnvVars does)
		err = envconfig.Process("TYK_PMP_PUMPS_ELASTICSEARCH_META", esConf)

		assert.NoError(t, err)
		assert.Equal(t, "env_var_nonexistent_ca.pem", esConf.SSLCAFile)
		assert.Equal(t, "https://localhost:9200", esConf.ElasticsearchURL)
	})

	t.Run("loads SSL CA file from environment variable when no config file setting", func(t *testing.T) {
		os.Setenv("TYK_PMP_PUMPS_ELASTICSEARCH_META_SSLCAFILE", "env_var_nonexistent_ca.pem")
		defer os.Unsetenv("TYK_PMP_PUMPS_ELASTICSEARCH_META_SSLCAFILE")

		config := map[string]any{
			"elasticsearch_url": "https://localhost:9200",
		}

		esConf := &ElasticsearchConf{}
		err := mapstructure.Decode(config, esConf)
		assert.NoError(t, err)

		// Process env vars (this is what processPumpEnvVars does)
		err = envconfig.Process("TYK_PMP_PUMPS_ELASTICSEARCH_META", esConf)

		assert.NoError(t, err)
		assert.Equal(t, "env_var_nonexistent_ca.pem", esConf.SSLCAFile)
	})

	t.Run("loads SSL CA file from config file when no environment variable", func(t *testing.T) {
		config := map[string]any{
			"elasticsearch_url": "https://localhost:9200",
			"ssl_ca_file":       "conf_nonexistent_ca.pem",
		}

		esConf := &ElasticsearchConf{}
		err := mapstructure.Decode(config, esConf)
		assert.NoError(t, err)

		// Process env vars (this is what processPumpEnvVars does)
		err = envconfig.Process("TYK_PMP_PUMPS_ELASTICSEARCH_META", esConf)

		assert.NoError(t, err)
		assert.Equal(t, "conf_nonexistent_ca.pem", esConf.SSLCAFile)
	})
}
