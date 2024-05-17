package analytics

import (
	"strconv"
	"time"

	"gorm.io/gorm"

	"github.com/TykTechnologies/storage/persistent/model"
	"github.com/fatih/structs"
)

const UptimeSQLTable = "tyk_uptime_analytics"

type UptimeReportData struct {
	ID           model.ObjectID `json:"_id" bson:"_id" gorm:"-:all"`
	URL          string         `json:"url"`
	RequestTime  int64          `json:"request_time"`
	ResponseCode int            `json:"response_code"`
	TCPError     bool           `json:"tcp_error"`
	ServerError  bool           `json:"server_error"`
	Day          int            `json:"day"`
	Month        time.Month     `json:"month"`
	Year         int            `json:"year"`
	Hour         int            `json:"hour"`
	Minute       int            `json:"minute"`
	TimeStamp    time.Time      `json:"timestamp"`
	ExpireAt     time.Time      `bson:"expireAt"`
	APIID        string         `json:"api_id"`
	OrgID        string         `json:"org_id"`
}

type UptimeReportAggregateSQL struct {
	ID string `gorm:"primaryKey"`

	Counter `json:"counter" gorm:"embedded"`

	TimeStamp      int64  `json:"timestamp" gorm:"index:dimension, priority:1"`
	OrgID          string `json:"org_id" gorm:"index:dimension, priority:2"`
	Dimension      string `json:"dimension" gorm:"index:dimension, priority:3"`
	DimensionValue string `json:"dimension_value" gorm:"index:dimension, priority:4"`

	Code `json:"code" gorm:"embedded"`
}

func (a *UptimeReportAggregateSQL) TableName() string {
	return UptimeSQLTable
}

func (a *UptimeReportData) GetObjectID() model.ObjectID {
	return a.ID
}

func (a *UptimeReportData) SetObjectID(id model.ObjectID) {
	a.ID = id
}

func (a *UptimeReportData) TableName() string {
	return UptimeSQLTable
}

func OnConflictUptimeAssignments(tableName, tempTable string) map[string]interface{} {
	assignments := make(map[string]interface{})
	f := UptimeReportAggregateSQL{}
	baseFields := structs.Fields(f.Code)
	for _, field := range baseFields {
		jsonTag := field.Tag("json")
		colName := "code_" + jsonTag
		assignments[colName] = gorm.Expr(tableName + "." + colName + " + " + tempTable + "." + colName)

	}

	fields := structs.Fields(f.Counter)
	for _, field := range fields {
		jsonTag := field.Tag("json")
		colName := "counter_" + jsonTag
		switch jsonTag {
		case "hits", "error", "success", "total_request_time":
			assignments[colName] = gorm.Expr(tableName + "." + colName + " + " + tempTable + "." + colName)
		case "request_time":
			if !field.IsZero() {
				assignments[colName] = gorm.Expr("(" + tableName + ".counter_total_request_time  +" + tempTable + "." + "counter_total_request_time" + ")/( " + tableName + ".counter_hits + " + tempTable + ".counter_hits" + ")")
			}
		case "last_time":
			assignments[colName] = gorm.Expr(tempTable + "." + colName)
		}
	}
	return assignments
}

func (u *UptimeReportAggregate) Dimensions() (dimensions []Dimension) {
	for key, inc := range u.URL {
		dimensions = append(dimensions, Dimension{"url", key, inc})
	}

	for key, inc := range u.Errors {
		dimensions = append(dimensions, Dimension{"errors", key, inc})
	}

	dimensions = append(dimensions, Dimension{"", "total", &u.Total})

	return
}

type UptimeReportAggregate struct {
	TimeStamp time.Time
	OrgID     string
	TimeID    struct {
		Year  int
		Month int
		Day   int
		Hour  int
	}

	URL    map[string]*Counter
	Errors map[string]*Counter

	Total Counter

	ExpireAt time.Time `bson:"expireAt" json:"expireAt"`
	LastTime time.Time
}

func (u UptimeReportAggregate) New() UptimeReportAggregate {
	agg := UptimeReportAggregate{}

	agg.URL = make(map[string]*Counter)
	agg.Errors = make(map[string]*Counter)

	return agg
}

func AggregateUptimeData(data []UptimeReportData) map[string]UptimeReportAggregate {
	analyticsPerOrg := make(map[string]UptimeReportAggregate)

	for _, thisV := range data {
		orgID := thisV.OrgID

		if orgID == "" {
			continue
		}

		thisAggregate, found := analyticsPerOrg[orgID]

		if !found {
			thisAggregate = UptimeReportAggregate{}.New()

			// Set the hourly timestamp & expiry
			asTime := thisV.TimeStamp
			thisAggregate.TimeStamp = time.Date(asTime.Year(), asTime.Month(), asTime.Day(), asTime.Hour(), 0, 0, 0, asTime.Location())

			thisAggregate.ExpireAt = thisV.ExpireAt
			thisAggregate.TimeID.Year = asTime.Year()
			thisAggregate.TimeID.Month = int(asTime.Month())
			thisAggregate.TimeID.Day = asTime.Day()
			thisAggregate.TimeID.Hour = asTime.Hour()
			thisAggregate.OrgID = orgID
			thisAggregate.LastTime = thisV.TimeStamp
			thisAggregate.Total.ErrorMap = make(map[string]int)
		}

		// Always update the last timestamp
		thisAggregate.LastTime = thisV.TimeStamp

		// Create the counter for this record
		var thisCounter Counter
		if thisV.ResponseCode == -1 {
			thisCounter = Counter{
				LastTime: thisV.TimeStamp,
			}
			if thisV.URL != "" {
				c := thisAggregate.URL[thisV.URL]
				if c == nil {
					c = &Counter{
						Identifier: thisV.URL,
					}
					thisAggregate.URL[thisV.URL] = c
				}
			}
		} else {
			thisCounter = Counter{
				Hits:             1,
				Success:          0,
				ErrorTotal:       0,
				RequestTime:      float64(thisV.RequestTime),
				TotalRequestTime: float64(thisV.RequestTime),
				LastTime:         thisV.TimeStamp,
				ErrorMap:         make(map[string]int),
				ErrorList:        []ErrorData{},
			}
			thisAggregate.Total.Hits++
			thisAggregate.Total.TotalRequestTime += float64(thisV.RequestTime)

			// We need an initial value
			thisAggregate.Total.RequestTime = thisAggregate.Total.TotalRequestTime / float64(thisAggregate.Total.Hits)
			if thisV.ResponseCode >= 400 {
				thisCounter.ErrorTotal = 1
				thisCounter.ErrorMap[strconv.Itoa(thisV.ResponseCode)]++
				thisAggregate.Total.ErrorTotal++
				thisAggregate.Total.ErrorMap[strconv.Itoa(thisV.ResponseCode)]++
			}

			if (thisV.ResponseCode < 300) && (thisV.ResponseCode >= 200) {
				thisCounter.Success = 1
				thisAggregate.Total.Success++
				// using the errorMap as ResponseCode Map for SQL purpose
				thisCounter.ErrorMap[strconv.Itoa(thisV.ResponseCode)]++
				thisAggregate.Total.ErrorMap[strconv.Itoa(thisV.ResponseCode)]++
			}

			// Convert to a map (for easy iteration)
			vAsMap := structs.Map(thisV)
			for key, value := range vAsMap {

				// Mini function to handle incrementing a specific counter in our object
				IncrementOrSetUnit := func(c *Counter) *Counter {
					if c == nil {
						newCounter := thisCounter
						newCounter.ErrorMap = make(map[string]int)
						for k, v := range thisCounter.ErrorMap {
							newCounter.ErrorMap[k] = v
						}
						c = &newCounter
					} else {
						c.Hits += thisCounter.Hits
						c.Success += thisCounter.Success
						c.ErrorTotal += thisCounter.ErrorTotal
						for k, v := range thisCounter.ErrorMap {
							c.ErrorMap[k] += v
						}
						c.TotalRequestTime += thisCounter.TotalRequestTime
						c.RequestTime = c.TotalRequestTime / float64(c.Hits)
					}

					return c
				}

				switch key {
				case "URL":
					c := IncrementOrSetUnit(thisAggregate.URL[value.(string)])
					if value.(string) != "" {
						thisAggregate.URL[value.(string)] = c
						thisAggregate.URL[value.(string)].Identifier = thisV.URL
					}
					break
				case "ResponseCode":
					errAsStr := strconv.Itoa(value.(int))
					if errAsStr != "" {
						c := IncrementOrSetUnit(thisAggregate.Errors[errAsStr])
						if c.ErrorTotal > 0 {
							thisAggregate.Errors[errAsStr] = c
							thisAggregate.Errors[errAsStr].Identifier = errAsStr
						}
					}
					break
				}
			}

		}

		analyticsPerOrg[orgID] = thisAggregate

	}

	return analyticsPerOrg
}
