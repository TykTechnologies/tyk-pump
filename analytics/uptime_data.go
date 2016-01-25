package analytics

import "time"

type UptimeReportData struct {
	URL          string
	RequestTime  int64
	ResponseCode int
	TCPError     bool
	ServerError  bool
	Day          int
	Month        time.Month
	Year         int
	Hour         int
	Minute       int
	TimeStamp    time.Time
	ExpireAt     time.Time `bson:"expireAt" json:"expireAt"`
	APIID        string
	OrgID        string
}
