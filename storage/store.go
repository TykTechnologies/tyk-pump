package storage

type AnalyticsStorage interface {
	Init(config interface{}) error
	GetName() string
	Connect() bool
	GetAndDeleteSet(string) []interface{}
}

const (
	RedisKeyPrefix    string = "analytics-"
	ANALYTICS_KEYNAME string = "tyk-system-analytics"
)
