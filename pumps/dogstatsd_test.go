package pumps

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

// testRecord returns a fully populated record so every supported tag and field has a value.
func testRecord() analytics.AnalyticsRecord {
	return analytics.AnalyticsRecord{
		Method:       "GET",
		Path:         "/orders/",
		APIVersion:   "v1",
		APIName:      "Orders API",
		APIID:        "api-1",
		OrgID:        "org-1",
		ResponseCode: 200,
		RequestTime:  42,
		TrackPath:    true,
		APIKey:       "abcdefghijklmnop",
		OauthID:      "oauth-1",
		Latency:      analytics.Latency{Total: 42, Upstream: 30, Gateway: 12},
	}
}

// writeToDogStatsd starts a UDP listener, runs a pump configured with meta against it and
// returns every datagram received. Each metric arrives as its own packet because the pump
// disables payload batching when buffering is off.
func writeToDogStatsd(t *testing.T, meta map[string]interface{}, records ...analytics.AnalyticsRecord) ([]string, error) {
	t.Helper()

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	assert.NoError(t, err)
	defer pc.Close()

	received := make(chan string, 64)

	go func() {
		buf := make([]byte, 65535)
		for {
			n, _, readErr := pc.ReadFrom(buf)
			if readErr != nil {
				return
			}
			received <- string(buf[:n])
		}
	}()

	if meta == nil {
		meta = map[string]interface{}{}
	}
	meta["address"] = pc.LocalAddr().String()
	meta["namespace"] = "pump"
	meta["sample_rate"] = 1.0

	pump := &DogStatsdPump{}
	if initErr := pump.Init(meta); initErr != nil {
		return nil, initErr
	}

	data := make([]interface{}, 0, len(records))
	for _, record := range records {
		data = append(data, record)
	}

	writeErr := pump.WriteData(context.Background(), data)

	// Give the client a moment to flush before reading what arrived.
	time.Sleep(200 * time.Millisecond)

	packets := []string{}
	for {
		select {
		case packet := <-received:
			packets = append(packets, packet)
		default:
			return packets, writeErr
		}
	}
}

// metricNames returns the metric name of each packet. Arrival order is not significant: the
// client shards metrics across independent buffers by name, so callers compare these as a set.
func metricNames(packets []string) []string {
	names := make([]string, 0, len(packets))
	for _, packet := range packets {
		names = append(names, strings.SplitN(packet, ":", 2)[0])
	}
	return names
}

func TestDogStatsdDefaultsUnchanged(t *testing.T) {
	t.Run("no tags and no fields configured", func(t *testing.T) {
		packets, err := writeToDogStatsd(t, nil, testRecord())
		assert.NoError(t, err)
		assert.Len(t, packets, 1)
		assert.Equal(t,
			"pump.request_time:42|h|#tyk-pump,path:/orders/,method:GET,response_code:200,"+
				"api_version:v1,api_name:Orders API,api_id:api-1,org_id:org-1,tracked:true,oauth_id:oauth-1",
			packets[0])
	})

	t.Run("tags configured but no fields", func(t *testing.T) {
		packets, err := writeToDogStatsd(t, map[string]interface{}{
			"tags": []string{"method", "api_id", "org_id", "path", "oauth_id"},
		}, testRecord())
		assert.NoError(t, err)
		assert.Len(t, packets, 1)
		assert.Equal(t,
			"pump.request_time:42|h|#tyk-pump,method:GET,api_id:api-1,org_id:org-1,path:/orders,oauth_id:oauth-1",
			packets[0])
	})

	t.Run("oauth_id is skipped when empty", func(t *testing.T) {
		record := testRecord()
		record.OauthID = ""

		packets, err := writeToDogStatsd(t, map[string]interface{}{
			"tags": []string{"api_id", "oauth_id"},
		}, record)
		assert.NoError(t, err)
		assert.Len(t, packets, 1)
		assert.NotContains(t, packets[0], "oauth_id")
	})
}

func TestDogStatsdAPIKeyTag(t *testing.T) {
	tcs := []struct {
		testName    string
		obfuscate   bool
		length      int
		apiKey      string
		expectedTag string
		omitted     bool
	}{
		{
			testName:    "obfuscation disabled emits the key verbatim",
			apiKey:      "abcdefghijklmnop",
			expectedTag: "api_key:abcdefghijklmnop",
		},
		{
			testName:    "obfuscation keeps the configured number of trailing characters",
			obfuscate:   true,
			length:      4,
			apiKey:      "abcdefghijklmnop",
			expectedTag: "api_key:****mnop",
		},
		{
			testName:    "obfuscation with the default length hides the key entirely",
			obfuscate:   true,
			length:      0,
			apiKey:      "abcdefghijklmnop",
			expectedTag: "api_key:****",
		},
		{
			testName:    "key shorter than the configured length is never revealed",
			obfuscate:   true,
			length:      32,
			apiKey:      "abcdefghijklmnop",
			expectedTag: "api_key:--",
		},
		{
			testName:    "key exactly the configured length is never revealed",
			obfuscate:   true,
			length:      16,
			apiKey:      "abcdefghijklmnop",
			expectedTag: "api_key:--",
		},
		{
			testName: "empty key omits the tag",
			apiKey:   "",
			omitted:  true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			record := testRecord()
			record.APIKey = tc.apiKey

			packets, err := writeToDogStatsd(t, map[string]interface{}{
				"tags":                      []string{"api_id", "api_key"},
				"obfuscate_api_keys":        tc.obfuscate,
				"obfuscate_api_keys_length": tc.length,
			}, record)
			assert.NoError(t, err)
			assert.Len(t, packets, 1)

			if tc.omitted {
				assert.NotContains(t, packets[0], "api_key")
				return
			}

			assert.Contains(t, packets[0], tc.expectedTag)
			// The raw key must never reach the wire once obfuscation is on.
			if tc.obfuscate {
				assert.NotContains(t, packets[0], "api_key:"+tc.apiKey)
			}
		})
	}
}

func TestDogStatsdFields(t *testing.T) {
	t.Run("latency fields replace the default metric", func(t *testing.T) {
		packets, err := writeToDogStatsd(t, map[string]interface{}{
			"tags":   []string{"api_id"},
			"fields": []string{"latency_total", "latency_upstream"},
		}, testRecord())
		assert.NoError(t, err)
		assert.Len(t, packets, 2)
		assert.ElementsMatch(t, []string{"pump.latency_total", "pump.latency_upstream"}, metricNames(packets))
		assert.ElementsMatch(t, []string{
			"pump.latency_total:42|h|#tyk-pump,api_id:api-1",
			"pump.latency_upstream:30|h|#tyk-pump,api_id:api-1",
		}, packets)
	})

	t.Run("every field carries the full tag set", func(t *testing.T) {
		packets, err := writeToDogStatsd(t, map[string]interface{}{
			"tags":   []string{"api_id", "org_id"},
			"fields": []string{"request_time", "latency_total", "latency_upstream", "latency_gateway"},
		}, testRecord())
		assert.NoError(t, err)
		assert.Len(t, packets, 4)
		assert.ElementsMatch(t, []string{
			"pump.request_time", "pump.latency_total", "pump.latency_upstream", "pump.latency_gateway",
		}, metricNames(packets))

		for _, packet := range packets {
			assert.Contains(t, packet, "#tyk-pump,api_id:api-1,org_id:org-1")
		}
	})

	t.Run("zero latency values are still emitted", func(t *testing.T) {
		record := testRecord()
		record.Latency = analytics.Latency{}

		packets, err := writeToDogStatsd(t, map[string]interface{}{
			"tags":   []string{"api_id"},
			"fields": []string{"latency_total", "latency_gateway"},
		}, record)
		assert.NoError(t, err)
		assert.Len(t, packets, 2)
		assert.ElementsMatch(t, []string{
			"pump.latency_total:0|h|#tyk-pump,api_id:api-1",
			"pump.latency_gateway:0|h|#tyk-pump,api_id:api-1",
		}, packets)
	})

	t.Run("an unknown field is skipped and the rest still emit", func(t *testing.T) {
		packets, err := writeToDogStatsd(t, map[string]interface{}{
			"tags":   []string{"api_id"},
			"fields": []string{"request_time", "nope", "latency_upstream"},
		}, testRecord())
		assert.NoError(t, err)
		assert.Len(t, packets, 2)
		assert.ElementsMatch(t, []string{"pump.request_time", "pump.latency_upstream"}, metricNames(packets))
	})

	t.Run("an all-unknown field list falls back to the default", func(t *testing.T) {
		packets, err := writeToDogStatsd(t, map[string]interface{}{
			"tags":   []string{"api_id"},
			"fields": []string{"nope", "also_nope"},
		}, testRecord())
		assert.NoError(t, err)
		assert.Len(t, packets, 1)
		assert.Equal(t, []string{"pump.request_time"}, metricNames(packets))
	})

	t.Run("every advertised field name resolves", func(t *testing.T) {
		// Guards the warning message against drifting from what the pump actually supports.
		for _, field := range dogstatsdSupportedFields {
			_, ok := dogstatsdFieldValue(field, &analytics.AnalyticsRecord{})
			assert.Truef(t, ok, "advertised field %q is not resolvable", field)
		}
	})

	t.Run("field names are trimmed", func(t *testing.T) {
		// Comma-separated environment variables commonly carry a space after the separator.
		packets, err := writeToDogStatsd(t, map[string]interface{}{
			"tags":   []string{"api_id"},
			"fields": []string{"request_time", " latency_upstream"},
		}, testRecord())
		assert.NoError(t, err)
		assert.Len(t, packets, 2)
		assert.ElementsMatch(t, []string{"pump.request_time", "pump.latency_upstream"}, metricNames(packets))
	})

	t.Run("a duplicated field is emitted once", func(t *testing.T) {
		packets, err := writeToDogStatsd(t, map[string]interface{}{
			"tags":   []string{"api_id"},
			"fields": []string{"request_time", "request_time"},
		}, testRecord())
		assert.NoError(t, err)
		assert.Len(t, packets, 1)
	})

	t.Run("an all-empty field list falls back to the default", func(t *testing.T) {
		packets, err := writeToDogStatsd(t, map[string]interface{}{
			"tags":   []string{"api_id"},
			"fields": []string{"", "  "},
		}, testRecord())
		assert.NoError(t, err)
		assert.Len(t, packets, 1)
		assert.Equal(t, []string{"pump.request_time"}, metricNames(packets))
	})
}

func TestDogStatsdBatching(t *testing.T) {
	t.Run("every record in the batch is written", func(t *testing.T) {
		first, second := testRecord(), testRecord()
		second.APIID = "api-2"

		packets, err := writeToDogStatsd(t, map[string]interface{}{
			"tags":   []string{"api_id"},
			"fields": []string{"request_time", "latency_total"},
		}, first, second)
		assert.NoError(t, err)
		assert.Len(t, packets, 4)
		assert.ElementsMatch(t, []string{
			"pump.request_time:42|h|#tyk-pump,api_id:api-1",
			"pump.latency_total:42|h|#tyk-pump,api_id:api-1",
			"pump.request_time:42|h|#tyk-pump,api_id:api-2",
			"pump.latency_total:42|h|#tyk-pump,api_id:api-2",
		}, packets)
	})

	t.Run("an empty batch writes nothing", func(t *testing.T) {
		packets, err := writeToDogStatsd(t, map[string]interface{}{
			"fields": []string{"request_time", "latency_total"},
		})
		assert.NoError(t, err)
		assert.Empty(t, packets)
	})

	t.Run("buffered mode still emits every field", func(t *testing.T) {
		packets, err := writeToDogStatsd(t, map[string]interface{}{
			"tags":                  []string{"api_id"},
			"fields":                []string{"request_time", "latency_total", "latency_upstream", "latency_gateway"},
			"buffered":              true,
			"buffered_max_messages": 32,
		}, testRecord())
		assert.NoError(t, err)

		// Buffering packs several metrics into each datagram, so assert on the combined payload
		// rather than on a packet count.
		combined := strings.Join(packets, "\n")
		for _, name := range []string{"request_time", "latency_total", "latency_upstream", "latency_gateway"} {
			assert.Contains(t, combined, "pump."+name+":")
		}
	})
}

func TestDogStatsdEnvVarOverrides(t *testing.T) {
	t.Setenv("TYK_PMP_PUMPS_DOGSTATSD_META_FIELDS", "latency_total,latency_upstream")
	t.Setenv("TYK_PMP_PUMPS_DOGSTATSD_META_TAGS", "api_id,api_key")
	t.Setenv("TYK_PMP_PUMPS_DOGSTATSD_META_OBFUSCATEAPIKEYS", "true")
	t.Setenv("TYK_PMP_PUMPS_DOGSTATSD_META_OBFUSCATEAPIKEYSLENGTH", "4")

	packets, err := writeToDogStatsd(t, nil, testRecord())
	assert.NoError(t, err)
	assert.Len(t, packets, 2)
	assert.ElementsMatch(t, []string{
		"pump.latency_total:42|h|#tyk-pump,api_id:api-1,api_key:****mnop",
		"pump.latency_upstream:30|h|#tyk-pump,api_id:api-1,api_key:****mnop",
	}, packets)
}

func TestDogStatsdAPIKeyEdgeCases(t *testing.T) {
	t.Run("a negative obfuscation length does not panic", func(t *testing.T) {
		record := testRecord()

		packets, err := writeToDogStatsd(t, map[string]interface{}{
			"tags":                      []string{"api_key"},
			"obfuscate_api_keys":        true,
			"obfuscate_api_keys_length": -1,
		}, record)
		assert.NoError(t, err)
		assert.Len(t, packets, 1)
		assert.Contains(t, packets[0], "api_key:****")
		assert.NotContains(t, packets[0], record.APIKey)
	})

	t.Run("a multi-byte key is not split mid-character", func(t *testing.T) {
		record := testRecord()
		record.APIKey = "key-\u65e5\u672c\u8a9e\u30c6\u30b9\u30c8"

		packets, err := writeToDogStatsd(t, map[string]interface{}{
			"tags":                      []string{"api_key"},
			"obfuscate_api_keys":        true,
			"obfuscate_api_keys_length": 4,
		}, record)
		assert.NoError(t, err)
		assert.Len(t, packets, 1)
		assert.True(t, utf8.ValidString(packets[0]), "metric line must stay valid UTF-8")
		// The last four characters, not the last four bytes.
		assert.Contains(t, packets[0], "api_key:****\u8a9e\u30c6\u30b9\u30c8")
	})

	t.Run("separators in a key cannot corrupt the metric line", func(t *testing.T) {
		record := testRecord()
		record.APIKey = "aaa,bbb|ccc#ddd"

		packets, err := writeToDogStatsd(t, map[string]interface{}{
			"tags": []string{"api_id", "api_key"},
		}, record)
		assert.NoError(t, err)
		assert.Len(t, packets, 1)
		assert.Equal(t, "pump.request_time:42|h|#tyk-pump,api_id:api-1,api_key:aaa_bbb_ccc_ddd", packets[0])
	})
}

func TestDogStatsdUnsupportedTags(t *testing.T) {
	for _, tag := range []string{"raw_request", "raw_response"} {
		t.Run(tag+" remains unsupported", func(t *testing.T) {
			_, err := writeToDogStatsd(t, map[string]interface{}{
				"tags": []string{tag},
			}, testRecord())
			assert.EqualError(t, err, "undefined tag '"+tag+"'")
		})
	}
}
