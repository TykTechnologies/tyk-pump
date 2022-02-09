package storage

import "time"

type AnalyticsStorage interface {
	Init(config interface{}) error
	GetName() string
	Connect() bool
	GetAndDeleteSet(setName string, chunkSize int64, expire time.Duration) []interface{}
}

type StorageHandler interface {
	Init(config interface{}) error
	Connect() bool
	AppendToSet(string, string)
	SetKey(string, string, int64) error
	GetListRange(string, int64, int64) ([]string, error)
	SetPrefix(string)
	DeleteKey(string) bool
	RemoveFromList(string,string) error
}

const (
	RedisKeyPrefix          string = "analytics-"
	ANALYTICS_KEYNAME       string = "tyk-system-analytics"
	UptimeAnalytics_KEYNAME string = "tyk-uptime-analytics"
)
