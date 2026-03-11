package pumps

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TykTechnologies/storage/persistent/model"
	"github.com/TykTechnologies/tyk-pump/analytics"
)

func TestAddMCPDimensionUpdates_IncludesLatencyFields(t *testing.T) {
	// Create MCP analytics records with non-zero latency and request time.
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	data := []interface{}{
		analytics.AnalyticsRecord{
			OrgID: "org1", APIID: "api1", TimeStamp: ts,
			ResponseCode: 200,
			RequestTime:  150,
			Latency: analytics.Latency{
				Total:    150,
				Upstream: 100,
			},
			MCPStats: analytics.MCPStats{
				IsMCP:         true,
				JSONRPCMethod: "tools/call",
				PrimitiveType: "tool",
				PrimitiveName: "my_tool",
			},
		},
		analytics.AnalyticsRecord{
			OrgID: "org1", APIID: "api1", TimeStamp: ts,
			ResponseCode: 429,
			RequestTime:  50,
			Latency: analytics.Latency{
				Total:    50,
				Upstream: 0,
			},
			MCPStats: analytics.MCPStats{
				IsMCP:         true,
				JSONRPCMethod: "tools/call",
				PrimitiveType: "tool",
				PrimitiveName: "my_tool",
			},
		},
	}

	// Aggregate the data
	result := analytics.AggregateMCPData(data, "", 60)
	require.Len(t, result, 1)

	ag := result["api1"]

	// Verify the in-memory counters have latency data
	require.Contains(t, ag.Names, "tool_my_tool")
	nameCounter := ag.Names["tool_my_tool"]
	assert.Equal(t, 2, nameCounter.Hits, "expected 2 hits")
	assert.NotZero(t, nameCounter.TotalRequestTime, "counter TotalRequestTime should be non-zero")
	assert.NotZero(t, nameCounter.TotalLatency, "counter TotalLatency should be non-zero")
	assert.NotZero(t, nameCounter.MaxLatency, "counter MaxLatency should be non-zero")

	// Now build the MongoDB update doc the same way DoMCPAggregatedWriting does
	updateDoc := ag.AnalyticsRecordAggregate.AsChange()
	addMCPDimensionUpdates(&ag, updateDoc)

	incDoc := updateDoc["$inc"].(model.DBM)
	maxDoc := updateDoc["$max"].(model.DBM)

	prefix := "names.tool_my_tool."

	// These fields must be present for latency data to be written to MongoDB.
	assert.Contains(t, incDoc, prefix+"totalrequesttime",
		"addMCPDimensionUpdates must write totalrequesttime for MCP dimensions")
	assert.Contains(t, incDoc, prefix+"totallatency",
		"addMCPDimensionUpdates must write totallatency for MCP dimensions")
	assert.Contains(t, incDoc, prefix+"totalupstreamlatency",
		"addMCPDimensionUpdates must write totalupstreamlatency for MCP dimensions")
	assert.Contains(t, maxDoc, prefix+"maxlatency",
		"addMCPDimensionUpdates must write maxlatency for MCP dimensions")
	assert.Contains(t, maxDoc, prefix+"maxupstreamlatency",
		"addMCPDimensionUpdates must write maxupstreamlatency for MCP dimensions")

	// Verify actual values are non-zero
	if val, ok := incDoc[prefix+"totalrequesttime"]; ok {
		assert.NotZero(t, val, "totalrequesttime value should be non-zero")
	}
	if val, ok := incDoc[prefix+"totallatency"]; ok {
		assert.NotZero(t, val, "totallatency value should be non-zero")
	}
	if val, ok := maxDoc[prefix+"maxlatency"]; ok {
		assert.NotZero(t, val, "maxlatency value should be non-zero")
	}
}
