package pumps

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHybridPump_aggregationTime(t *testing.T) {
	t.Run("default is 60 minutes", func(t *testing.T) {
		p := &HybridPump{storeAnalyticPerMinute: false}
		assert.Equal(t, 60, p.aggregationTime())
	})

	t.Run("per minute when configured", func(t *testing.T) {
		p := &HybridPump{storeAnalyticPerMinute: true}
		assert.Equal(t, 1, p.aggregationTime())
	})
}

func TestDispatcherFuncsIncludesMCPAggregated(t *testing.T) {
	fn, ok := dispatcherFuncs["PurgeAnalyticsDataMCPAggregated"]
	assert.True(t, ok, "dispatcherFuncs must include PurgeAnalyticsDataMCPAggregated")

	typedFn, ok := fn.(func(string) error)
	assert.True(t, ok, "PurgeAnalyticsDataMCPAggregated must be func(string) error")

	err := typedFn("test")
	assert.NoError(t, err)
}
