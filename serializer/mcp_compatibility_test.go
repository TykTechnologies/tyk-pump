package serializer

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	analyticsproto "github.com/TykTechnologies/tyk-pump/analytics/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestSerializerMCPContextCompatibility(t *testing.T) {
	data, err := os.ReadFile("../analytics/testdata/mcp_context_cases.json")
	require.NoError(t, err)
	var cases []struct {
		Name   string                    `json:"name"`
		Record analytics.AnalyticsRecord `json:"record"`
	}
	require.NoError(t, json.Unmarshal(data, &cases))
	require.Len(t, cases, 9)
	for _, name := range []string{MSGP_SERIALIZER, PROTOBUF_SERIALIZER} {
		t.Run(name, func(t *testing.T) {
			codec := NewAnalyticsSerializer(name)
			for _, tc := range cases {
				t.Run(tc.Name, func(t *testing.T) {
					encoded, err := codec.Encode(&tc.Record)
					require.NoError(t, err)
					var decoded analytics.AnalyticsRecord
					require.NoError(t, codec.Decode(encoded, &decoded))
					assert.Equal(t, tc.Record.APIID, decoded.APIID)
					assert.Equal(t, tc.Record.OrgID, decoded.OrgID)
					assert.Equal(t, tc.Record.ResponseCode, decoded.ResponseCode)
					assert.True(t, decoded.IsMCPRecord())
					assert.Equal(t, tc.Record.MCPStats, decoded.MCPStats)
				})
			}
		})
	}
}

func TestSerializerMCPLegacyImmutableWireFixtures(t *testing.T) {
	for _, tc := range []struct{ name, file string }{{MSGP_SERIALIZER, "msgpack"}, {PROTOBUF_SERIALIZER, "protobuf"}} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile("../analytics/testdata/mcp_legacy." + tc.file + ".hex")
			require.NoError(t, err)
			encoded, err := hex.DecodeString(strings.TrimSpace(string(data)))
			require.NoError(t, err)
			var decoded analytics.AnalyticsRecord
			require.NoError(t, NewAnalyticsSerializer(tc.name).Decode(encoded, &decoded))
			assert.Equal(t, "legacy-api", decoded.APIID)
			assert.Equal(t, "legacy-org", decoded.OrgID)
			assert.True(t, decoded.IsMCPRecord())
			assert.Equal(t, "tools/call", decoded.MCPStats.JSONRPCMethod)
			assert.Equal(t, "tool", decoded.MCPStats.PrimitiveType)
			assert.Equal(t, "legacy_tool", decoded.MCPStats.PrimitiveName)
			assert.Empty(t, decoded.MCPStats.EffectiveProtocolVersion)
			assert.Empty(t, decoded.MCPStats.DeclaredProtocolVersion)
			assert.Empty(t, decoded.MCPStats.ProtocolVersionSource)
			assert.Zero(t, decoded.MCPStats.JSONRPCErrorCode)
			if tc.name == PROTOBUF_SERIALIZER {
				assert.Equal(t, time.Unix(0, 0).UTC(), decoded.TimeStamp)
				assert.Equal(t, time.Unix(0, 0).UTC(), decoded.ExpireAt)
				assert.Equal(t, analytics.GeoData{}, decoded.Geo)
				assert.Equal(t, analytics.NetworkStats{}, decoded.Network)
				assert.Equal(t, analytics.Latency{}, decoded.Latency)
			}
		})
	}
}

func TestSerializerMCPProtobufFieldNumbers(t *testing.T) {
	fields := (&analyticsproto.MCPStats{}).ProtoReflect().Descriptor().Fields()
	for _, tc := range []struct {
		name   protoreflect.Name
		number protoreflect.FieldNumber
		kind   protoreflect.Kind
	}{
		{"IsMCP", 1, protoreflect.BoolKind},
		{"JSONRPCMethod", 2, protoreflect.StringKind},
		{"PrimitiveType", 3, protoreflect.StringKind},
		{"PrimitiveName", 4, protoreflect.StringKind},
		{"EffectiveProtocolVersion", 5, protoreflect.StringKind},
		{"DeclaredProtocolVersion", 6, protoreflect.StringKind},
		{"ProtocolVersionSource", 7, protoreflect.StringKind},
		{"JSONRPCErrorCode", 8, protoreflect.Int64Kind},
	} {
		t.Run(string(tc.name), func(t *testing.T) {
			field := fields.ByName(tc.name)
			require.NotNil(t, field)
			assert.Equal(t, tc.number, field.Number())
			assert.Equal(t, tc.kind, field.Kind())
		})
	}
}
