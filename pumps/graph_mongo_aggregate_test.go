package pumps

import (
	"context"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGraphMongoAggregatePump_Init(t *testing.T) {
	cfg := make(map[string]interface{})
	cfg["mongo_url"] = "mongodb://localhost:27017/tyk_analytics"
	cfg["use_mixed_collection"] = true

	pmp := GraphMongoAggregatePump{}
	err := pmp.Init(cfg)
	require.NoError(t, err)
	assert.Equal(t, "Graph MongoDB Aggregate Pump", pmp.GetName())
}

func TestGraphMongoAggregatePump_WriteData(t *testing.T) {
	cfg := make(map[string]interface{})
	cfg["mongo_url"] = "mongodb://localhost:27017/tyk_analytics"
	cfg["use_mixed_collection"] = true

	pmp := GraphMongoAggregatePump{}
	err := pmp.Init(cfg)
	require.NoError(t, err)

	now := time.Now()
	
	record := analytics.AnalyticsRecord{
		APIID:        "test-api",
		OrgID:        "test-org",
		TimeStamp:    now,
		ResponseCode: 200,
		RequestTime:  100,
		GraphQLStats: analytics.GraphQLStats{
			IsGraphQL: true,
			OperationType: analytics.OperationQuery,
		},
	}

	data := []interface{}{record}
	err = pmp.WriteData(context.Background(), data)
	require.NoError(t, err)
}
