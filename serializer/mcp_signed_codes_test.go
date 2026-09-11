package serializer

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/TykTechnologies/tyk-pump/analytics"
	analyticsproto "github.com/TykTechnologies/tyk-pump/analytics/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestSerializerMCPSignedCodeBoundaries(t *testing.T) {
	for name, code := range map[string]int64{"zero": 0, "reserved-denial": -33002, "min32": math.MinInt32, "max32": math.MaxInt32, "below32": -2147483649, "above32": 2147483648, "min64": math.MinInt64, "max64": math.MaxInt64} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile("../analytics/testdata/mcp_signed_codes/" + name + ".json")
			require.NoError(t, err)
			var input analytics.AnalyticsRecord
			decodeErr := json.Unmarshal(data, &input)
			pb := &ProtobufSerializer{}
			if code < int64(math.MinInt) || code > int64(math.MaxInt) {
				require.Error(t, decodeErr)
				wireRecord := pb.TransformSingleRecordToProto(analytics.AnalyticsRecord{MCPStats: analytics.MCPStats{IsMCP: true}})
				wireRecord.MCPStats.JSONRPCErrorCode = code
				sentinel := analytics.AnalyticsRecord{APIID: "unchanged", MCPStats: analytics.MCPStats{IsMCP: true, JSONRPCErrorCode: -33002}}
				destination := sentinel
				require.ErrorContains(t, pb.TransformSingleProtoToAnalyticsRecord(wireRecord, &destination), "cannot be represented as int")
				assert.Equal(t, sentinel, destination)
				wire, marshalErr := proto.Marshal(wireRecord)
				require.NoError(t, marshalErr)
				require.ErrorContains(t, pb.Decode(wire, &destination), "cannot be represented as int")
				assert.Equal(t, sentinel, destination)
				return
			}
			require.NoError(t, decodeErr)
			require.Equal(t, code, int64(input.MCPStats.JSONRPCErrorCode))
			require.Empty(t, input.APIKey)
			transformed := pb.TransformSingleRecordToProto(input)
			assert.Equal(t, code, transformed.MCPStats.JSONRPCErrorCode)
			var restored analytics.AnalyticsRecord
			require.NoError(t, pb.TransformSingleProtoToAnalyticsRecord(transformed, &restored))
			assert.Equal(t, input.MCPStats, restored.MCPStats)
			assert.Equal(t, input.APIID, restored.APIID)
			for _, format := range []string{MSGP_SERIALIZER, PROTOBUF_SERIALIZER} {
				t.Run(format, func(t *testing.T) {
					codec := NewAnalyticsSerializer(format)
					encoded, encodeErr := codec.Encode(&input)
					require.NoError(t, encodeErr)
					var result analytics.AnalyticsRecord
					require.NoError(t, codec.Decode(encoded, &result))
					assert.Equal(t, code, int64(result.MCPStats.JSONRPCErrorCode))
					assert.Equal(t, input.MCPStats, result.MCPStats)
					assert.Equal(t, input.APIID, result.APIID)
					assert.Equal(t, input.OrgID, result.OrgID)
					assert.Equal(t, input.UserAgent, result.UserAgent)
				})
			}
		})
	}
}

func TestSerializerMCPOldInt32WireCompatibility(t *testing.T) {
	data, err := os.ReadFile("../analytics/testdata/mcp_int32_977/analytics.descriptor.bin")
	require.NoError(t, err)
	var frozen descriptorpb.FileDescriptorSet
	require.NoError(t, proto.Unmarshal(data, &frozen))
	files, err := protodesc.NewFiles(&frozen)
	require.NoError(t, err)
	file, err := files.FindFileByPath("analytics.proto")
	require.NoError(t, err)
	oldMCPDescriptor := file.Messages().ByName("MCPStats")
	require.NotNil(t, oldMCPDescriptor)
	require.Equal(t, protoreflect.Int32Kind, oldMCPDescriptor.Fields().ByNumber(8).Kind())
	oldRecordDescriptor := file.Messages().ByName("AnalyticsRecord")
	require.NotNil(t, oldRecordDescriptor)
	for name, code := range map[string]int64{"zero": 0, "reserved-denial": -33002, "min32": math.MinInt32, "max32": math.MaxInt32, "below32": -2147483649, "above32": 2147483648, "min64": math.MinInt64, "max64": math.MaxInt64} {
		t.Run(name, func(t *testing.T) {
			current := &analyticsproto.MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "signed_code_probe", EffectiveProtocolVersion: "2026-07-28", DeclaredProtocolVersion: "2026-07-28", ProtocolVersionSource: "header_body", JSONRPCErrorCode: code}
			currentBytes, marshalErr := proto.Marshal(current)
			require.NoError(t, marshalErr)
			old := dynamicpb.NewMessage(oldMCPDescriptor)
			if code < math.MinInt32 || code > math.MaxInt32 {
				require.NoError(t, proto.Unmarshal(currentBytes, old))
				// The frozen old reader cannot preserve wide values. This is a rollout limitation.
				expectedOld := map[string]int64{"below32": math.MaxInt32, "above32": math.MinInt32, "min64": 0, "max64": -1}
				assert.Equal(t, expectedOld[name], old.Get(oldMCPDescriptor.Fields().ByNumber(8)).Int())
				assert.NotEqual(t, code, old.Get(oldMCPDescriptor.Fields().ByNumber(8)).Int())
				return
			}
			old.Set(oldMCPDescriptor.Fields().ByNumber(1), protoreflect.ValueOfBool(true))
			for number, value := range map[protoreflect.FieldNumber]string{2: "tools/call", 3: "tool", 4: "signed_code_probe", 5: "2026-07-28", 6: "2026-07-28", 7: "header_body"} {
				old.Set(oldMCPDescriptor.Fields().ByNumber(number), protoreflect.ValueOfString(value))
			}
			old.Set(oldMCPDescriptor.Fields().ByNumber(8), protoreflect.ValueOfInt32(int32(code)))
			oldBytes, oldMarshalErr := proto.MarshalOptions{Deterministic: true}.Marshal(old)
			require.NoError(t, oldMarshalErr)
			assert.Equal(t, oldBytes, currentBytes, "signed32 values retain identical varint encoding")
			var currentDecoded analyticsproto.MCPStats
			require.NoError(t, proto.Unmarshal(oldBytes, &currentDecoded))
			assert.True(t, proto.Equal(current, &currentDecoded))

			input := analytics.AnalyticsRecord{APIID: "old-int32-api", OrgID: "old-int32-org", UserAgent: "old-int32-request"}
			pb := &ProtobufSerializer{}
			complete := pb.TransformSingleRecordToProto(input)
			complete.MCPStats = current
			completeBytes, fullMarshalErr := proto.Marshal(complete)
			require.NoError(t, fullMarshalErr)
			oldRecord := dynamicpb.NewMessage(oldRecordDescriptor)
			require.NoError(t, proto.Unmarshal(completeBytes, oldRecord))
			oldFullBytes, fullOldMarshalErr := proto.MarshalOptions{Deterministic: true}.Marshal(oldRecord)
			require.NoError(t, fullOldMarshalErr)
			var decoded analytics.AnalyticsRecord
			require.NoError(t, pb.Decode(oldFullBytes, &decoded))
			assert.Equal(t, input.APIID, decoded.APIID)
			assert.Equal(t, input.OrgID, decoded.OrgID)
			assert.Equal(t, input.UserAgent, decoded.UserAgent)
			assert.Equal(t, code, int64(decoded.MCPStats.JSONRPCErrorCode))
			assert.Equal(t, current.JSONRPCMethod, decoded.MCPStats.JSONRPCMethod)
			assert.Equal(t, current.PrimitiveType, decoded.MCPStats.PrimitiveType)
			assert.Equal(t, current.PrimitiveName, decoded.MCPStats.PrimitiveName)
			assert.Equal(t, current.EffectiveProtocolVersion, decoded.MCPStats.EffectiveProtocolVersion)
			assert.Equal(t, current.DeclaredProtocolVersion, decoded.MCPStats.DeclaredProtocolVersion)
			assert.Equal(t, current.ProtocolVersionSource, decoded.MCPStats.ProtocolVersionSource)
		})
	}
}
