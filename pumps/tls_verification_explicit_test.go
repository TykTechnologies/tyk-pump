package pumps

import (
	"reflect"
	"testing"

	"github.com/TykTechnologies/tyk-pump/storage"
)

// TestTLSVerificationExplicit_DefaultsAreSecure verifies that every TLS-capable
// pump's config struct defaults its insecure-skip-verify flag to FALSE on the
// zero value. The contract is: insecure TLS modes MUST require explicit
// operator opt-in via a documented config flag — they shall never be
// implicit/default-on.
//
// SW-REQ-016:tls_verification_explicit:nominal
// SW-REQ-021:tls_verification_explicit:nominal
// SW-REQ-029:tls_verification_explicit:nominal
// SW-REQ-068:tls_verification_explicit:nominal
// Phase S Wave 3a reproducer test.
//
// Method: reflection-based. For each registered config struct, construct the
// zero value and assert that the documented insecure-skip-verify field is a
// bool whose default is false. Catches regressions where a maintainer might
// flip the default or rename the field without updating callers.
func TestTLSVerificationExplicit_DefaultsAreSecure(t *testing.T) {
	// Each row exercises one config struct + one field name. Field names are
	// taken from a literal grep of the source — keep them in sync.
	cases := []struct {
		name          string
		getConfig     func() interface{}
		insecureField string
		verifyingReq  string
	}{
		{
			name:          "Kafka.SSLInsecureSkipVerify",
			getConfig:     func() interface{} { return KafkaConf{} },
			insecureField: "SSLInsecureSkipVerify",
			verifyingReq:  "SW-REQ-021",
		},
		{
			name:          "Hybrid.SSLInsecureSkipVerify",
			getConfig:     func() interface{} { return HybridPumpConf{} },
			insecureField: "SSLInsecureSkipVerify",
			verifyingReq:  "SW-REQ-029",
		},
		{
			name:          "Mongo.MongoSSLInsecureSkipVerify",
			getConfig:     func() interface{} { return MongoConf{} },
			insecureField: "MongoSSLInsecureSkipVerify",
			verifyingReq:  "SW-REQ-034",
		},
		{
			name:          "MongoAggregate.MongoSSLInsecureSkipVerify",
			getConfig:     func() interface{} { return MongoAggregateConf{} },
			insecureField: "MongoSSLInsecureSkipVerify",
			verifyingReq:  "SW-REQ-034",
		},
		{
			name:          "MongoSelective.MongoSSLInsecureSkipVerify",
			getConfig:     func() interface{} { return MongoSelectiveConf{} },
			insecureField: "MongoSSLInsecureSkipVerify",
			verifyingReq:  "SW-REQ-034",
		},
		{
			name:          "Elasticsearch.SSLInsecureSkipVerify",
			getConfig:     func() interface{} { return ElasticsearchConf{} },
			insecureField: "SSLInsecureSkipVerify",
			verifyingReq:  "SW-REQ-016",
		},
		{
			name:          "Splunk.SSLInsecureSkipVerify",
			getConfig:     func() interface{} { return SplunkPumpConfig{} },
			insecureField: "SSLInsecureSkipVerify",
			verifyingReq:  "SW-REQ-016",
		},
		{
			name:          "TLSConfig.InsecureSkipVerify",
			getConfig:     func() interface{} { return TLSConfig{} },
			insecureField: "InsecureSkipVerify",
			verifyingReq:  "SW-REQ-016",
		},
		{
			name:          "RedisStorage.SSLInsecureSkipVerify",
			getConfig:     func() interface{} { return storage.TemporalStorageConfig{} },
			insecureField: "SSLInsecureSkipVerify",
			verifyingReq:  "SW-REQ-016",
		},
		{
			name:          "RedisStorage.RedisSSLInsecureSkipVerify(deprecated)",
			getConfig:     func() interface{} { return storage.TemporalStorageConfig{} },
			insecureField: "RedisSSLInsecureSkipVerify",
			verifyingReq:  "SW-REQ-016",
		},
	}

	if len(cases) == 0 {
		t.Fatal("no TLS-capable configs registered — test would silently pass")
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			cfg := c.getConfig()
			v := reflect.ValueOf(cfg)
			// FieldByName walks embedded structs too, so this works for the
			// MongoConf which embeds BaseMongoConf.
			f := v.FieldByName(c.insecureField)
			if !f.IsValid() {
				t.Fatalf("%s: zero-value config has no field %q (obligation %s)",
					c.name, c.insecureField, c.verifyingReq)
			}
			if f.Kind() != reflect.Bool {
				t.Fatalf("%s: field %q is not bool (got %s)",
					c.name, c.insecureField, f.Kind())
			}
			if f.Bool() {
				t.Errorf("%s: zero-value insecure-skip-verify is TRUE; "+
					"operators must explicitly opt-in to insecure TLS "+
					"(violates obligation tls_verification_explicit on %s)",
					c.name, c.verifyingReq)
			}
		})
	}
}
