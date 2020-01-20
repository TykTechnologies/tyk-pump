package storage

type AnalyticsStorage interface {
	Init(config interface{}) error
	GetName() string
	Connect() bool
	GetAndDeleteSet(setName string, chunkSize int64) []interface{}
}

const (
	RedisKeyPrefix          string = "analytics-"
	ANALYTICS_KEYNAME       string = "tyk-system-analytics"
	UptimeAnalytics_KEYNAME string = "tyk-uptime-analytics"
)
