package storage

import (
	"github.com/TykTechnologies/tyk-pump/logger"
)

var log = logger.GetLogger()
var AvailableStores map[string]AnalyticsStorage

func init() {
	AvailableStores = make(map[string]AnalyticsStorage)

	// Register all the storage handlers here
	AvailableStores["redis"] = &TemporalStorageHandler{Config: TemporalStorageConfig{
		Type: "redis",
	}}
}
