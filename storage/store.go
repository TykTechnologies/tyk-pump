package storage

import "time"

type AnalyticsStorage interface {
	Init(config interface{}) error
	GetName() string
	Connect() bool
	GetAndDeleteSet(setName string, chunkSize int64, expire time.Duration) []interface{}
}

const (
	RedisKeyPrefix          string = "analytics-"
	ANALYTICS_KEYNAME       string = "tyk-system-analytics"
	UptimeAnalytics_KEYNAME string = "tyk-uptime-analytics"
)
