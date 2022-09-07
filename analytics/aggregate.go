package analytics

import (
	b64 "encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/structs"
	"github.com/sirupsen/logrus"
	"gopkg.in/mgo.v2/bson"
	"gorm.io/gorm"
)

const (
	AgggregateMixedCollectionName = "tyk_analytics_aggregates"
	MongoAggregatePrefix          = "mongo-pump-aggregate"
	AggregateSQLTable             = "tyk_aggregated"
)

//lastDcoumentTimestamp is a map to store the last document timestamps of different Mongo Aggregators
var lastDcoumentTimestamp = make(map[string]time.Time)

type ErrorData struct {
	Code  string
	Count int
}

type Counter struct {
	Hits                 int       `json:"hits"`
	Success              int       `json:"success"`
	ErrorTotal           int       `json:"error" gorm:"column:error"`
	RequestTime          float64   `json:"request_time"`
	TotalRequestTime     float64   `json:"total_request_time"`
	Identifier           string    `json:"identifier" sql:"-"`
	HumanIdentifier      string    `json:"human_identifier"`
	LastTime             time.Time `json:"last_time"`
	OpenConnections      int64     `json:"open_connections"`
	ClosedConnections    int64     `json:"closed_connections"`
	BytesIn              int64     `json:"bytes_in"`
	BytesOut             int64     `json:"bytes_out"`
	MaxUpstreamLatency   int64     `json:"max_upstream_latency"`
	MinUpstreamLatency   int64     `json:"min_upstream_latency"`
	TotalUpstreamLatency int64     `json:"total_upstream_latency"`
	UpstreamLatency      float64   `json:"upstream_latency"`

	MaxLatency   int64   `json:"max_latency"`
	MinLatency   int64   `json:"min_latency"`
	TotalLatency int64   `json:"total_latency"`
	Latency      float64 `json:"latency"`

	ErrorMap  map[string]int `json:"error_map" sql:"-"`
	ErrorList []ErrorData    `json:"error_list" sql:"-"`
}

type AnalyticsRecordAggregate struct {
	TimeStamp time.Time
	OrgID     string
	TimeID    struct {
		Year  int
		Month int
		Day   int
		Hour  int
	}

	APIKeys map[string]*Counter
	Errors  map[string]*Counter

	Versions map[string]*Counter
	APIID    map[string]*Counter
	OauthIDs map[string]*Counter
	Geo      map[string]*Counter
	Tags     map[string]*Counter

	Endpoints map[string]*Counter

	Lists struct {
		APIKeys       []Counter
		APIID         []Counter
		OauthIDs      []Counter
		Geo           []Counter
		Tags          []Counter
		Errors        []Counter
		Endpoints     []Counter
		KeyEndpoint   map[string][]Counter `bson:"keyendpoints"`
		OauthEndpoint map[string][]Counter `bson:"oauthendpoints"`
		APIEndpoint   []Counter            `bson:"apiendpoints"`
	}

	KeyEndpoint   map[string]map[string]*Counter `bson:"keyendpoints"`
	OauthEndpoint map[string]map[string]*Counter `bson:"oauthendpoints"`
	ApiEndpoint   map[string]*Counter            `bson:"apiendpoints"`

	Total Counter

	ExpireAt time.Time `bson:"expireAt" json:"expireAt"`
	LastTime time.Time
}

type SQLAnalyticsRecordAggregate struct {
	ID string `gorm:"primaryKey"`

	Counter `json:"counter" gorm:"embedded"`

	TimeStamp      int64  `json:"timestamp" gorm:"index:dimension, priority:1"`
	OrgID          string `json:"org_id" gorm:"index:dimension, priority:2"`
	Dimension      string `json:"dimension" gorm:"index:dimension, priority:3"`
	DimensionValue string `json:"dimension_value" gorm:"index:dimension, priority:4"`

	Code `json:"code" gorm:"embedded"`
}

type Code struct {
	Code1x  int `json:"1x" gorm:"1x"`
	Code200 int `json:"200" gorm:"200"`
	Code201 int `json:"201" gorm:"201"`
	Code2x  int `json:"2x" gorm:"2x"`
	Code301 int `json:"301" gorm:"301"`
	Code302 int `json:"302" gorm:"302"`
	Code303 int `json:"303" gorm:"303"`
	Code304 int `json:"304" gorm:"304"`
	Code3x  int `json:"3x" gorm:"3x"`
	Code400 int `json:"400" gorm:"400"`
	Code401 int `json:"401" gorm:"401"`
	Code403 int `json:"403" gorm:"403"`
	Code404 int `json:"404" gorm:"404"`
	Code429 int `json:"429" gorm:"429"`
	Code4x  int `json:"4x" gorm:"4x"`
	Code500 int `json:"500" gorm:"500"`
	Code501 int `json:"501" gorm:"501"`
	Code502 int `json:"502" gorm:"502"`
	Code503 int `json:"503" gorm:"503"`
	Code504 int `json:"504" gorm:"504"`
	Code5x  int `json:"5x" gorm:"5x"`
}

func (c *Code) ProcessStatusCodes(errorMap map[string]int) {
	codeStruct := structs.New(c)
	for k, v := range errorMap {
		if field, ok := codeStruct.FieldOk("Code" + k); ok {
			_ = field.Set(v)
		} else {
			if field, ok = codeStruct.FieldOk("Code" + string(k[0]) + "x"); ok {
				_ = field.Set(v + field.Value().(int))
			}
		}
	}
}

func (f *SQLAnalyticsRecordAggregate) TableName() string {
	return AggregateSQLTable
}

func OnConflictAssignments(tableName string, tempTable string) map[string]interface{} {
	assignments := make(map[string]interface{})
	f := SQLAnalyticsRecordAggregate{}
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
		//hits, error, success"s, open_connections, closed_connections, bytes_in, bytes_out,total_request_time, total_upstream_latency, total_latency
		case "hits", "error", "success", "open_connections", "closed_connections", "bytes_in", "bytes_out", "total_request_time", "total_latency", "total_upstream_latency":
			assignments[colName] = gorm.Expr(tableName + "." + colName + " + " + tempTable + "." + colName)
		//request_time, upstream_latency,latency
		case "request_time", "upstream_latency", "latency":
			//AVG = (oldTotal + newTotal ) / (oldHits + newHits)
			var totalVal, totalCol string
			switch jsonTag {
			case "request_time":
				totalCol = "counter_total_request_time"
			case "upstream_latency":
				totalCol = "counter_total_upstream_latency"
			case "latency":
				totalCol = "counter_total_latency"
			}
			totalVal = tempTable + "." + totalCol

			assignments[colName] = gorm.Expr("(" + tableName + "." + totalCol + "  +" + totalVal + ")/CAST( " + tableName + ".counter_hits + " + tempTable + ".counter_hits" + " AS REAL)")

		case "max_upstream_latency", "max_latency":
			//math max: 0.5 * ((@val1 + @val2) + ABS(@val1 - @val2))
			val1 := tableName + "." + colName
			val2 := tempTable + "." + colName
			assignments[colName] = gorm.Expr("0.5 * ((" + val1 + " + " + val2 + ") + ABS(" + val1 + " - " + val2 + "))")

		case "min_latency", "min_upstream_latency":
			//math min: 0.5 * ((@val1 + @val2) - ABS(@val1 - @val2))
			val1 := tableName + "." + colName
			val2 := tempTable + "." + colName
			assignments[colName] = gorm.Expr("0.5 * ((" + val1 + " + " + val2 + ") - ABS(" + val1 + " - " + val2 + ")) ")

		case "last_time":
			assignments[colName] = gorm.Expr(tempTable + "." + colName)

		}
	}

	return assignments
}

func (f AnalyticsRecordAggregate) New() AnalyticsRecordAggregate {
	thisF := AnalyticsRecordAggregate{}
	thisF.APIID = make(map[string]*Counter)
	thisF.Errors = make(map[string]*Counter)
	thisF.Versions = make(map[string]*Counter)
	thisF.APIKeys = make(map[string]*Counter)
	thisF.OauthIDs = make(map[string]*Counter)
	thisF.Geo = make(map[string]*Counter)
	thisF.Tags = make(map[string]*Counter)
	thisF.Endpoints = make(map[string]*Counter)
	thisF.KeyEndpoint = make(map[string]map[string]*Counter)
	thisF.OauthEndpoint = make(map[string]map[string]*Counter)
	thisF.ApiEndpoint = make(map[string]*Counter)

	return thisF
}

func (f *AnalyticsRecordAggregate) generateBSONFromProperty(parent, thisUnit string, incVal *Counter, newUpdate bson.M) bson.M {

	constructor := parent + "." + thisUnit + "."
	if parent == "" {
		constructor = thisUnit + "."
	}

	newUpdate["$inc"].(bson.M)[constructor+"hits"] = incVal.Hits
	newUpdate["$inc"].(bson.M)[constructor+"success"] = incVal.Success
	newUpdate["$inc"].(bson.M)[constructor+"errortotal"] = incVal.ErrorTotal
	for k, v := range incVal.ErrorMap {
		newUpdate["$inc"].(bson.M)[constructor+"errormap."+k] = v
	}
	newUpdate["$inc"].(bson.M)[constructor+"totalrequesttime"] = incVal.TotalRequestTime
	newUpdate["$set"].(bson.M)[constructor+"identifier"] = incVal.Identifier
	newUpdate["$set"].(bson.M)[constructor+"humanidentifier"] = incVal.HumanIdentifier
	newUpdate["$set"].(bson.M)[constructor+"lasttime"] = incVal.LastTime
	newUpdate["$set"].(bson.M)[constructor+"openconnections"] = incVal.OpenConnections
	newUpdate["$set"].(bson.M)[constructor+"closedconnections"] = incVal.ClosedConnections
	newUpdate["$set"].(bson.M)[constructor+"bytesin"] = incVal.BytesIn
	newUpdate["$set"].(bson.M)[constructor+"bytesout"] = incVal.BytesOut
	newUpdate["$max"].(bson.M)[constructor+"maxlatency"] = incVal.MaxLatency
	// Don't update min latency in case of errors
	if incVal.Hits != incVal.ErrorTotal {
		if newUpdate["$min"] == nil {
			newUpdate["$min"] = bson.M{}
		}
		newUpdate["$min"].(bson.M)[constructor+"minlatency"] = incVal.MinLatency
		newUpdate["$min"].(bson.M)[constructor+"minupstreamlatency"] = incVal.MinUpstreamLatency
	}
	newUpdate["$max"].(bson.M)[constructor+"maxupstreamlatency"] = incVal.MaxUpstreamLatency
	newUpdate["$inc"].(bson.M)[constructor+"totalupstreamlatency"] = incVal.TotalUpstreamLatency
	newUpdate["$inc"].(bson.M)[constructor+"totallatency"] = incVal.TotalLatency

	return newUpdate
}

func (f *AnalyticsRecordAggregate) generateSetterForTime(parent, thisUnit string, realTime float64, newUpdate bson.M) bson.M {

	constructor := parent + "." + thisUnit + "."
	if parent == "" {
		constructor = thisUnit + "."
	}
	newUpdate["$set"].(bson.M)[constructor+"requesttime"] = realTime

	return newUpdate
}

func (f *AnalyticsRecordAggregate) latencySetter(parent, thisUnit string, newUpdate bson.M, counter *Counter) bson.M {
	if counter.Hits > 0 {
		counter.Latency = float64(counter.TotalLatency) / float64(counter.Hits)
		counter.UpstreamLatency = float64(counter.TotalUpstreamLatency) / float64(counter.Hits)
	} else {
		counter.Latency = 0.0
		counter.UpstreamLatency = 0.0
	}

	constructor := parent + "." + thisUnit + "."
	if parent == "" {
		constructor = thisUnit + "."
	}
	newUpdate["$set"].(bson.M)[constructor+"latency"] = counter.Latency
	newUpdate["$set"].(bson.M)[constructor+"upstreamlatency"] = counter.UpstreamLatency

	return newUpdate
}

type Dimension struct {
	Name    string
	Value   string
	Counter *Counter
}

func (f *AnalyticsRecordAggregate) Dimensions() (dimensions []Dimension) {
	fnLatencySetter := func(counter *Counter) *Counter {
		if counter.Hits > 0 {
			counter.Latency = float64(counter.TotalLatency) / float64(counter.Hits)
			counter.UpstreamLatency = float64(counter.TotalUpstreamLatency) / float64(counter.Hits)
		}
		return counter
	}

	for key, inc := range f.APIID {
		dimensions = append(dimensions, Dimension{"apiid", key, fnLatencySetter(inc)})
	}

	for key, inc := range f.Errors {
		dimensions = append(dimensions, Dimension{"errors", key, fnLatencySetter(inc)})
	}

	for key, inc := range f.Versions {
		dimensions = append(dimensions, Dimension{"versions", key, fnLatencySetter(inc)})
	}

	for key, inc := range f.APIKeys {
		dimensions = append(dimensions, Dimension{"apikeys", key, fnLatencySetter(inc)})
	}

	for key, inc := range f.OauthIDs {
		dimensions = append(dimensions, Dimension{"oauthids", key, fnLatencySetter(inc)})
	}

	for key, inc := range f.Geo {
		dimensions = append(dimensions, Dimension{"geo", key, fnLatencySetter(inc)})
	}

	for key, inc := range f.Tags {
		dimensions = append(dimensions, Dimension{"tags", key, fnLatencySetter(inc)})
	}

	for key, inc := range f.Endpoints {
		dimensions = append(dimensions, Dimension{"endpoints", key, fnLatencySetter(inc)})
	}

	for key, inc := range f.KeyEndpoint {
		for k, v := range inc {
			dimensions = append(dimensions, Dimension{"keyendpoints", key + "." + k, fnLatencySetter(v)})
		}
	}

	for key, inc := range f.OauthEndpoint {
		for k, v := range inc {
			dimensions = append(dimensions, Dimension{"oauthendpoints", key + "." + k, fnLatencySetter(v)})
		}
	}

	for key, inc := range f.ApiEndpoint {
		dimensions = append(dimensions, Dimension{"apiendpoints", key, fnLatencySetter(inc)})
	}

	dimensions = append(dimensions, Dimension{"", "total", fnLatencySetter(&f.Total)})

	return
}

func (f *AnalyticsRecordAggregate) AsChange() (newUpdate bson.M) {
	newUpdate = bson.M{
		"$inc": bson.M{},
		"$set": bson.M{},
		"$max": bson.M{},
	}

	for _, d := range f.Dimensions() {
		newUpdate = f.generateBSONFromProperty(d.Name, d.Value, d.Counter, newUpdate)
	}

	newUpdate = f.generateBSONFromProperty("", "total", &f.Total, newUpdate)

	asTime := f.TimeStamp
	newTime := time.Date(asTime.Year(), asTime.Month(), asTime.Day(), asTime.Hour(), asTime.Minute(), 0, 0, asTime.Location())
	newUpdate["$set"].(bson.M)["timestamp"] = newTime
	newUpdate["$set"].(bson.M)["expireAt"] = f.ExpireAt
	newUpdate["$set"].(bson.M)["timeid.year"] = newTime.Year()
	newUpdate["$set"].(bson.M)["timeid.month"] = newTime.Month()
	newUpdate["$set"].(bson.M)["timeid.day"] = newTime.Day()
	newUpdate["$set"].(bson.M)["timeid.hour"] = newTime.Hour()
	newUpdate["$set"].(bson.M)["lasttime"] = f.LastTime

	return newUpdate
}

func (f *AnalyticsRecordAggregate) SetErrorList(parent, thisUnit string, counter *Counter, newUpdate bson.M) {
	constructor := parent + "." + thisUnit + "."
	if parent == "" {
		constructor = thisUnit + "."
	}

	errorlist := make([]ErrorData, 0)

	for k, v := range counter.ErrorMap {
		element := ErrorData{
			Code:  k,
			Count: v,
		}
		errorlist = append(errorlist, element)
	}
	counter.ErrorList = errorlist

	newUpdate["$set"].(bson.M)[constructor+"errorlist"] = counter.ErrorList
}

func (f *AnalyticsRecordAggregate) getRecords(fieldName string, data map[string]*Counter, newUpdate bson.M) []Counter {
	result := make([]Counter, 0)

	for thisUnit, incVal := range data {
		var newTime float64

		if incVal.Hits > 0 {
			newTime = incVal.TotalRequestTime / float64(incVal.Hits)
		}
		f.SetErrorList(fieldName, thisUnit, incVal, newUpdate)
		newUpdate = f.generateSetterForTime(fieldName, thisUnit, newTime, newUpdate)
		newUpdate = f.latencySetter(fieldName, thisUnit, newUpdate, incVal)
		result = append(result, *incVal)
	}

	return result
}

func (f *AnalyticsRecordAggregate) AsTimeUpdate() bson.M {
	newUpdate := bson.M{
		"$set": bson.M{},
	}

	// We need to create lists of API data so that we can aggregate across the list
	// in order to present top-20 style lists of APIs, Tokens etc.
	//apis := make([]Counter, 0)
	newUpdate["$set"].(bson.M)["lists.apiid"] = f.getRecords("apiid", f.APIID, newUpdate)

	newUpdate["$set"].(bson.M)["lists.errors"] = f.getRecords("errors", f.Errors, newUpdate)

	newUpdate["$set"].(bson.M)["lists.versions"] = f.getRecords("versions", f.Versions, newUpdate)

	newUpdate["$set"].(bson.M)["lists.apikeys"] = f.getRecords("apikeys", f.APIKeys, newUpdate)

	newUpdate["$set"].(bson.M)["lists.oauthids"] = f.getRecords("oauthids", f.OauthIDs, newUpdate)

	newUpdate["$set"].(bson.M)["lists.geo"] = f.getRecords("geo", f.Geo, newUpdate)

	newUpdate["$set"].(bson.M)["lists.tags"] = f.getRecords("tags", f.Tags, newUpdate)

	newUpdate["$set"].(bson.M)["lists.endpoints"] = f.getRecords("endpoints", f.Endpoints, newUpdate)

	for thisUnit, incVal := range f.KeyEndpoint {
		parent := "lists.keyendpoints." + thisUnit
		newUpdate["$set"].(bson.M)[parent] = f.getRecords("keyendpoints."+thisUnit, incVal, newUpdate)
	}

	for thisUnit, incVal := range f.OauthEndpoint {
		parent := "lists.oauthendpoints." + thisUnit
		newUpdate["$set"].(bson.M)[parent] = f.getRecords("oauthendpoints."+thisUnit, incVal, newUpdate)
	}

	newUpdate["$set"].(bson.M)["lists.apiendpoints"] = f.getRecords("apiendpoints", f.ApiEndpoint, newUpdate)

	var newTime float64

	if f.Total.Hits > 0 {
		newTime = f.Total.TotalRequestTime / float64(f.Total.Hits)
	}
	f.SetErrorList("", "total", &f.Total, newUpdate)
	newUpdate = f.generateSetterForTime("", "total", newTime, newUpdate)
	newUpdate = f.latencySetter("", "total", newUpdate, &f.Total)

	return newUpdate
}

//DiscardAggregations this method discard the aggregations of X field specified in the aggregated pump configuration
func (f *AnalyticsRecordAggregate) DiscardAggregations(fields []string) {
	for _, field := range fields {
		switch field {
		case "APIID", "apiid":
			f.APIID = make(map[string]*Counter)
		case "Errors", "errors":
			f.Errors = make(map[string]*Counter)
		case "Versions", "versions":
			f.Versions = make(map[string]*Counter)
		case "APIKeys", "apikeys":
			f.APIKeys = make(map[string]*Counter)
		case "OauthIDs", "oauthids":
			f.OauthIDs = make(map[string]*Counter)
		case "Geo", "geo":
			f.Geo = make(map[string]*Counter)
		case "Tags", "tags":
			f.Tags = make(map[string]*Counter)
		case "Endpoints", "endpoints":
			f.Endpoints = make(map[string]*Counter)
		case "KeyEndpoint", "keyendpoints":
			f.KeyEndpoint = make(map[string]map[string]*Counter)
		case "OauthEndpoint", "oauthendpoints":
			f.OauthEndpoint = make(map[string]map[string]*Counter)
		case "ApiEndpoint", "apiendpoints":
			f.ApiEndpoint = make(map[string]*Counter)
		default:
			log.WithFields(logrus.Fields{
				"prefix": MongoAggregatePrefix,
				"field":  field,
			}).Warning("Invalid field in the ignore list. Skipping.")
		}
	}
}

func doHash(in string) string {
	sEnc := b64.StdEncoding.EncodeToString([]byte(in))
	search := strings.TrimRight(sEnc, "=")
	return search
}

func ignoreTag(tag string, ignoreTagPrefixList []string) bool {
	// ignore tag added for key by gateway
	if strings.HasPrefix(tag, "key-") {
		return true
	}

	for _, prefix := range ignoreTagPrefixList {
		if strings.HasPrefix(tag, prefix) {
			return true
		}
	}

	return false
}

func replaceUnsupportedChars(path string) string {
	result := path

	if strings.Contains(path, ".") {
		dotUnicode := fmt.Sprintf("\\u%x", ".")
		result = strings.Replace(path, ".", dotUnicode, -1)
	}

	return result
}

// AggregateData calculates aggregated data, returns map orgID => aggregated analytics data
func AggregateData(data []interface{}, trackAllPaths bool, ignoreTagPrefixList []string, dbIdentifier string, AnalyticsStoredPerMinute int, ignoreGraphData bool) map[string]AnalyticsRecordAggregate {
	fmt.Println("-------MINUTES:-----------", AnalyticsStoredPerMinute)
	analyticsPerOrg := make(map[string]AnalyticsRecordAggregate)
	for _, v := range data {
		thisV := v.(AnalyticsRecord)
		orgID := thisV.OrgID

		if orgID == "" {
			continue
		}

		// We don't want to aggregate Graph Data with REST data - there is a different type for that.
		if ignoreGraphData && thisV.IsGraphRecord() {
			continue
		}

		thisAggregate, found := analyticsPerOrg[orgID]

		if !found {
			thisAggregate = AnalyticsRecordAggregate{}.New()

			// Set the hourly timestamp & expiry
			asTime := thisV.TimeStamp

			if AnalyticsStoredPerMinute == 60 {
				//if AnalyticsStoredPerMinute is set to 60, use asTime.Hour()
				thisAggregate.TimeStamp = time.Date(asTime.Year(), asTime.Month(), asTime.Day(), asTime.Hour(), 0, 0, 0, asTime.Location())
			} else if dbIdentifier != "" {
				//if AnalyticsStoredPerMinute != 60 and the database is Mongo (because we have an identifier):
				if lastDcoumentTimestamp[dbIdentifier].Add(time.Minute * time.Duration(AnalyticsStoredPerMinute)).After(asTime) {
					//if the last record timestamp + amount of minutes set < current time, just add the new record to the current document
					thisAggregate.TimeStamp = lastDcoumentTimestamp[dbIdentifier]
				} else {
					//if last record timestamp + amount of minutes set > current time, just create a new record
					newTime := time.Date(asTime.Year(), asTime.Month(), asTime.Day(), asTime.Hour(), asTime.Minute(), 0, 0, asTime.Location())
					lastDcoumentTimestamp[dbIdentifier] = newTime
					thisAggregate.TimeStamp = newTime
				}
			} else {
				//if AnalyticsStoredPerMinute != 60, and the DB is not Mongo, the only option is use asTime.Minute()
				thisAggregate.TimeStamp = time.Date(asTime.Year(), asTime.Month(), asTime.Day(), asTime.Hour(), asTime.Minute(), 0, 0, asTime.Location())
			}
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
		thisAggregate.Total.LastTime = thisV.TimeStamp

		// Create the counter for this record
		var thisCounter Counter
		if thisV.ResponseCode == -1 {
			thisCounter = Counter{
				LastTime:          thisV.TimeStamp,
				OpenConnections:   thisV.Network.OpenConnections,
				ClosedConnections: thisV.Network.ClosedConnection,
				BytesIn:           thisV.Network.BytesIn,
				BytesOut:          thisV.Network.BytesOut,
			}
			thisAggregate.Total.OpenConnections += thisCounter.OpenConnections
			thisAggregate.Total.ClosedConnections += thisCounter.ClosedConnections
			thisAggregate.Total.BytesIn += thisCounter.BytesIn
			thisAggregate.Total.BytesOut += thisCounter.BytesOut
			if thisV.APIID != "" {
				c := thisAggregate.APIID[thisV.APIID]
				if c == nil {
					c = &Counter{
						Identifier:      thisV.APIID,
						HumanIdentifier: thisV.APIName,
					}
					thisAggregate.APIID[thisV.APIID] = c
				}
				c.BytesIn += thisCounter.BytesIn
				c.BytesOut += thisCounter.BytesOut
			}
		} else {
			thisCounter = Counter{
				Hits:             1,
				Success:          0,
				ErrorTotal:       0,
				RequestTime:      float64(thisV.RequestTime),
				TotalRequestTime: float64(thisV.RequestTime),
				LastTime:         thisV.TimeStamp,

				MaxUpstreamLatency:   thisV.Latency.Upstream,
				MinUpstreamLatency:   thisV.Latency.Upstream,
				TotalUpstreamLatency: thisV.Latency.Upstream,
				MaxLatency:           thisV.Latency.Total,
				MinLatency:           thisV.Latency.Total,
				TotalLatency:         thisV.Latency.Total,
				ErrorMap:             make(map[string]int),
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
			}

			thisAggregate.Total.TotalLatency += thisV.Latency.Total
			thisAggregate.Total.TotalUpstreamLatency += thisV.Latency.Upstream

			if thisAggregate.Total.MaxLatency < thisV.Latency.Total {
				thisAggregate.Total.MaxLatency = thisV.Latency.Total
			}

			if thisAggregate.Total.MaxUpstreamLatency < thisV.Latency.Upstream {
				thisAggregate.Total.MaxUpstreamLatency = thisV.Latency.Upstream
			}

			// by default, min_total_latency will have 0 value
			// it should not be set to 0 always
			if thisAggregate.Total.Hits == 1 {
				thisAggregate.Total.MinLatency = thisV.Latency.Total
				thisAggregate.Total.MinUpstreamLatency = thisV.Latency.Upstream
			} else {
				// Don't update min latency in case of error
				if thisAggregate.Total.MinLatency > thisV.Latency.Total && (thisV.ResponseCode < 300) && (thisV.ResponseCode >= 200) {
					thisAggregate.Total.MinLatency = thisV.Latency.Total
				}
				// Don't update min latency in case of error
				if thisAggregate.Total.MinUpstreamLatency > thisV.Latency.Upstream && (thisV.ResponseCode < 300) && (thisV.ResponseCode >= 200) {
					thisAggregate.Total.MinUpstreamLatency = thisV.Latency.Upstream
				}
			}

			if trackAllPaths {
				thisV.TrackPath = true
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

						if c.MaxLatency < thisCounter.MaxLatency {
							c.MaxLatency = thisCounter.MaxLatency
						}

						// don't update min latency in case of errors
						if c.MinLatency > thisCounter.MinLatency && thisCounter.ErrorTotal == 0 {
							c.MinLatency = thisCounter.MinLatency
						}

						if c.MaxUpstreamLatency < thisCounter.MaxUpstreamLatency {
							c.MaxUpstreamLatency = thisCounter.MaxUpstreamLatency
						}

						// don't update min latency in case of errors
						if c.MinUpstreamLatency > thisCounter.MinUpstreamLatency && thisCounter.ErrorTotal == 0 {
							c.MinUpstreamLatency = thisCounter.MinUpstreamLatency
						}

						c.TotalLatency += thisCounter.TotalLatency
						c.TotalUpstreamLatency += thisCounter.TotalUpstreamLatency

					}

					return c
				}

				switch key {
				case "APIID":
					c := IncrementOrSetUnit(thisAggregate.APIID[value.(string)])
					if value.(string) != "" {
						thisAggregate.APIID[value.(string)] = c
						thisAggregate.APIID[value.(string)].Identifier = thisV.APIID
						thisAggregate.APIID[value.(string)].HumanIdentifier = thisV.APIName
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
				case "APIVersion":
					versionStr := doHash(thisV.APIID + ":" + value.(string))
					c := IncrementOrSetUnit(thisAggregate.Versions[versionStr])
					if value.(string) != "" {
						thisAggregate.Versions[versionStr] = c
						thisAggregate.Versions[versionStr].Identifier = value.(string)
						thisAggregate.Versions[versionStr].HumanIdentifier = value.(string)
					}
					break
				case "APIKey":
					if value.(string) != "" {
						c := IncrementOrSetUnit(thisAggregate.APIKeys[value.(string)])
						thisAggregate.APIKeys[value.(string)] = c
						thisAggregate.APIKeys[value.(string)].Identifier = value.(string)
						thisAggregate.APIKeys[value.(string)].HumanIdentifier = thisV.Alias

						if thisV.TrackPath {
							keyStr := doHash(thisV.APIID + ":" + thisV.Path)
							data := thisAggregate.KeyEndpoint[value.(string)]

							if data == nil {
								data = make(map[string]*Counter)
							}

							c = IncrementOrSetUnit(data[keyStr])
							c.Identifier = keyStr
							c.HumanIdentifier = keyStr
							data[keyStr] = c
							thisAggregate.KeyEndpoint[value.(string)] = data

						}
					}
					break
				case "OauthID":
					if value.(string) != "" {
						c := IncrementOrSetUnit(thisAggregate.OauthIDs[value.(string)])
						thisAggregate.OauthIDs[value.(string)] = c
						thisAggregate.OauthIDs[value.(string)].Identifier = value.(string)

						if thisV.TrackPath {
							keyStr := doHash(thisV.APIID + ":" + thisV.Path)
							data := thisAggregate.OauthEndpoint[value.(string)]

							if data == nil {
								data = make(map[string]*Counter)
							}

							c = IncrementOrSetUnit(data[keyStr])
							c.Identifier = keyStr
							c.HumanIdentifier = keyStr
							data[keyStr] = c
							thisAggregate.OauthEndpoint[value.(string)] = data
						}
					}
					break
				case "Geo":
					c := IncrementOrSetUnit(thisAggregate.Geo[thisV.Geo.Country.ISOCode])
					if thisV.Geo.Country.ISOCode != "" {
						thisAggregate.Geo[thisV.Geo.Country.ISOCode] = c
						thisAggregate.Geo[thisV.Geo.Country.ISOCode].Identifier = thisV.Geo.Country.ISOCode
						thisAggregate.Geo[thisV.Geo.Country.ISOCode].HumanIdentifier = thisV.Geo.Country.ISOCode
					}
					break

				case "Tags":
					for _, thisTag := range thisV.Tags {
						trimmedTag := TrimTag(thisTag)

						if trimmedTag != "" && !ignoreTag(thisTag, ignoreTagPrefixList) {
							c := IncrementOrSetUnit(thisAggregate.Tags[trimmedTag])
							thisAggregate.Tags[trimmedTag] = c
							thisAggregate.Tags[trimmedTag].Identifier = trimmedTag
							thisAggregate.Tags[trimmedTag].HumanIdentifier = trimmedTag
						}
					}
					break

				case "TrackPath":
					log.Debug("TrackPath=", value.(bool))
					if value.(bool) {
						fixedPath := replaceUnsupportedChars(thisV.Path)
						c := IncrementOrSetUnit(thisAggregate.Endpoints[fixedPath])
						thisAggregate.Endpoints[fixedPath] = c
						thisAggregate.Endpoints[fixedPath].Identifier = thisV.Path
						thisAggregate.Endpoints[fixedPath].HumanIdentifier = thisV.Path

						keyStr := hex.EncodeToString([]byte(thisV.APIID + ":" + thisV.APIVersion + ":" + thisV.Path))
						c = IncrementOrSetUnit(thisAggregate.ApiEndpoint[keyStr])
						thisAggregate.ApiEndpoint[keyStr] = c
						thisAggregate.ApiEndpoint[keyStr].Identifier = keyStr
						thisAggregate.ApiEndpoint[keyStr].HumanIdentifier = thisV.Path
					}
					break
				}
			}

		}
		analyticsPerOrg[orgID] = thisAggregate

	}

	return analyticsPerOrg
}

func TrimTag(thisTag string) string {
	trimmedTag := strings.TrimSpace(thisTag)

	trimmedTag = strings.Replace(trimmedTag, ".", "", -1)
	return trimmedTag
}

func SetlastTimestampAgggregateRecord(id string, date time.Time) {
	lastDcoumentTimestamp[id] = date
}
