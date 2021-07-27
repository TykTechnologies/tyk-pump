package analytics

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/structs"
	"gorm.io/gorm"
)

const UptimeSQLTable = "tyk_uptime_analytics"

type UptimeReportData struct {
	URL          string     `json:"url"`
	RequestTime  int64      `json:"request_time"`
	ResponseCode int        `json:"response_code"`
	TCPError     bool       `json:"tcp_error"`
	ServerError  bool       `json:"server_error"`
	Day          int        `json:"day"`
	Month        time.Month `json:"month"`
	Year         int        `json:"year"`
	Hour         int        `json:"hour"`
	Minute       int        `json:"minute"`
	TimeStamp    time.Time  `json:"timestamp"`
	ExpireAt     time.Time  `bson:"expireAt"`
	APIID        string     `json:"api_id"`
	OrgID        string     `json:"org_id"`
}

type UptimeReportAggregateSQL struct {
	ID string `gorm:"primaryKey"`

	Counter `json:"counter" gorm:"embedded"`

	TimeStamp      int64  `json:"timestamp" gorm:"index:dimension, priority:1"`
	OrgID          string `json:"org_id" gorm:"index:dimension, priority:2"`
	Dimension      string `json:"dimension" gorm:"index:dimension, priority:3"`
	DimensionValue string `json:"dimension_value" gorm:"index:dimension, priority:4"`

	Code1x  int `json:"code_1x"`
	Code200 int `json:"code_200"`
	Code201 int `json:"code_201"`
	Code2x  int `json:"code_2x"`
	Code301 int `json:"code_301"`
	Code302 int `json:"code_302"`
	Code303 int `json:"code_303"`
	Code304 int `json:"code_304"`
	Code3x  int `json:"code_3x"`
	Code400 int `json:"code_400"`
	Code401 int `json:"code_401"`
	Code403 int `json:"code_403"`
	Code404 int `json:"code_404"`
	Code429 int `json:"code_429"`
	Code4x  int `json:"code_4x"`
	Code500 int `json:"code_500"`
	Code501 int `json:"code_501"`
	Code502 int `json:"code_502"`
	Code503 int `json:"code_503"`
	Code504 int `json:"code_504"`
	Code5x  int `json:"code_5x"`
}

func (a UptimeReportAggregateSQL) GetAssignments(tableName string) map[string]interface{} {
	assignments := make(map[string]interface{})

	baseFields := structs.Fields(a)
	for _, field := range baseFields {
		colName := field.Tag("json")
		if strings.Contains(colName, "code_") {
			if !field.IsZero() {
				assignments[colName] = gorm.Expr(tableName + "." + colName + " + " + fmt.Sprint(field.Value()))
			}
		}
	}

	fields := structs.Fields(a.Counter)
	for _, field := range fields {
		colName := "counter_" + field.Tag("json")

		switch {
		case strings.Contains(colName, "hits"), strings.Contains(colName, "error"), strings.Contains(colName, "success"):
			if !field.IsZero() {
				assignments[colName] = gorm.Expr(tableName + "." + colName + " + " + fmt.Sprint(field.Value()))
			}
		case strings.Contains(colName, "total_request_time"):
			if !field.IsZero() {
				assignments[colName] = gorm.Expr(tableName + "." + colName + " + " + fmt.Sprint(a.TotalRequestTime))
			}
		case strings.Contains(colName, "request_time"):
			//AVG adding value to another AVG: newAve = ((oldAve*oldNumPoints) + x)/(oldNumPoints+1)
			if !field.IsZero() {
				assignments[colName] = gorm.Expr("(" + tableName + ".counter_total_request_time  +" + fmt.Sprintf("%v", a.TotalRequestTime) + ")/( " + tableName + ".counter_hits + " + fmt.Sprintf("%v", a.Hits) + ")")
			}
		case strings.Contains(colName, "last_time"):
			if !field.IsZero() {
				assignments[colName] = gorm.Expr("'" + a.LastTime.Format("2006-01-02 15:04:05-07:00") + "'")
			}
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

func (a *UptimeReportAggregateSQL) TableName() string {
	return UptimeSQLTable
}

func (a *UptimeReportAggregateSQL) ProcessStatusCodes() {
	for k, v := range a.Counter.ErrorMap {
		switch k {
		case "200":
			a.Code200 = v
		case "201":
			a.Code201 = v
		case "301":
			a.Code301 = v
		case "302":
			a.Code302 = v
		case "303":
			a.Code303 = v
		case "400":
			a.Code400 = v
		case "401":
			a.Code401 = v
		case "403":
			a.Code403 = v
		case "404":
			a.Code404 = v
		case "429":
			a.Code429 = v
		case "500":
			a.Code500 = v
		case "501":
			a.Code501 = v
		case "502":
			a.Code502 = v
		case "503":
			a.Code503 = v
		case "504":
			a.Code504 = v
		default:
			switch k[0] {
			case '1':
				a.Code1x = v
			case '2':
				a.Code2x = v
			case '3':
				a.Code3x = v
			case '4':
				a.Code4x = v
			case '5':
				a.Code5x = v
			}
		}
	}

	a.Counter.ErrorList = nil
	a.Counter.ErrorMap = nil
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
				//using the errorMap as ResponseCode Map for SQL purpose
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
