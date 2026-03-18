package pumps

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDispatcherFuncsIncludesMCPAggregated(t *testing.T) {
	fn, ok := dispatcherFuncs["PurgeAnalyticsDataMCPAggregated"]
	assert.True(t, ok, "dispatcherFuncs must include PurgeAnalyticsDataMCPAggregated")

	typedFn, ok := fn.(func(string) error)
	assert.True(t, ok, "PurgeAnalyticsDataMCPAggregated must be func(string) error")

	assert.NoError(t, typedFn("test"))
}
