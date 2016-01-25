package storage

import (
	"github.com/lonelycode/tyk-pump/logger"
)

var log = logger.GetLogger()
var AvailableStores map[string]AnalyticsStorage

func init() {
	AvailableStores = make(map[string]AnalyticsStorage)

	// Register all the storage handlers here
	AvailableStores["redis"] = &RedisClusterStorageManager{}
}
