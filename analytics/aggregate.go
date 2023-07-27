package analytics

import (
	b64 "encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TykTechnologies/storage/persistent/model"
	"github.com/fatih/structs"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	AgggregateMixedCollectionName     = "tyk_analytics_aggregates"
	GraphAggregateMixedCollectionName = "tyk_graph_analytics_aggregate"
	MongoAggregatePrefix              = "mongo-pump-aggregate"
	AggregateSQLTable                 = "tyk_aggregated"
	AggregateGraphSQLTable            = "tyk_graph_aggregated"
)

// lastDocumentTimestamp is a map to store the last document timestamps of different Mongo Aggregators
var lastDocumentTimestamp = make(map[string]time.Time)

// mutex is used to prevent concurrent writes to the same key
var mutex sync.RWMutex

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

type GraphRecordAggregate struct {
	AnalyticsRecordAggregate

	Types      map[string]*Counter
	Fields     map[string]*Counter
	Operation  map[string]*Counter
	RootFields map[string]*Counter
}

type AggregateFieldList struct {
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

type AnalyticsRecordAggregate struct {
	id        model.ObjectID `bson:"_id" gorm:"-:all"`
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

	Lists AggregateFieldList

	KeyEndpoint   map[string]map[string]*Counter `bson:"keyendpoints"`
	OauthEndpoint map[string]map[string]*Counter `bson:"oauthendpoints"`
	ApiEndpoint   map[string]*Counter            `bson:"apiendpoints"`

	Total Counter

	ExpireAt time.Time `bson:"expireAt" json:"expireAt"`
	LastTime time.Time
	Mixed    bool `bson:"-" json:"-"`
}

func (f *AnalyticsRecordAggregate) TableName() string {
	if f.Mixed {
		return AgggregateMixedCollectionName
	}
	return "z_tyk_analyticz_aggregate_" + f.OrgID
}

func (f *AnalyticsRecordAggregate) GetObjectID() model.ObjectID {
	return f.id
}

func (f *AnalyticsRecordAggregate) SetObjectID(id model.ObjectID) {
	f.id = id
}

type SQLAnalyticsRecordAggregate struct {
	ID string `gorm:"primaryKey"`

	Counter `json:"counter" gorm:"embedded"`

	TimeStamp      int64  `json:"timestamp"`
	OrgID          string `json:"org_id"`
	Dimension      string `json:"dimension"`
	DimensionValue string `json:"dimension_value"`

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

func OnConflictAssignments(tableName, tempTable string) map[string]interface{} {
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
		// hits, error, success"s, open_connections, closed_connections, bytes_in, bytes_out,total_request_time, total_upstream_latency, total_latency
		case "hits", "error", "success", "open_connections", "closed_connections", "bytes_in", "bytes_out", "total_request_time", "total_latency", "total_upstream_latency":
			assignments[colName] = gorm.Expr(tableName + "." + colName + " + " + tempTable + "." + colName)
		// request_time, upstream_latency,latency
		case "request_time", "upstream_latency", "latency":
			// AVG = (oldTotal + newTotal ) / (oldHits + newHits)
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
			// math max: 0.5 * ((@val1 + @val2) + ABS(@val1 - @val2))
			val1 := tableName + "." + colName
			val2 := tempTable + "." + colName
			assignments[colName] = gorm.Expr("0.5 * ((" + val1 + " + " + val2 + ") + ABS(" + val1 + " - " + val2 + "))")

		case "min_latency", "min_upstream_latency":
			// math min: 0.5 * ((@val1 + @val2) - ABS(@val1 - @val2))
			val1 := tableName + "." + colName
			val2 := tempTable + "." + colName
			assignments[colName] = gorm.Expr("0.5 * ((" + val1 + " + " + val2 + ") - ABS(" + val1 + " - " + val2 + ")) ")

		case "last_time":
			assignments[colName] = gorm.Expr(tempTable + "." + colName)

		}
	}

	return assignments
}

func NewGraphRecordAggregate() GraphRecordAggregate {
	analyticsAggregate := AnalyticsRecordAggregate{}.New()

	return GraphRecordAggregate{
		AnalyticsRecordAggregate: analyticsAggregate,
		Types:                    make(map[string]*Counter),
		Fields:                   make(map[string]*Counter),
		Operation:                make(map[string]*Counter),
		RootFields:               make(map[string]*Counter),
	}
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

func (f *AnalyticsRecordAggregate) generateBSONFromProperty(parent, thisUnit string, incVal *Counter, newUpdate model.DBM) model.DBM {
	constructor := parent + "." + thisUnit + "."
	if parent == "" {
		constructor = thisUnit + "."
	}

	newUpdate["$inc"].(model.DBM)[constructor+"hits"] = incVal.Hits
	newUpdate["$inc"].(model.DBM)[constructor+"success"] = incVal.Success
	newUpdate["$inc"].(model.DBM)[constructor+"errortotal"] = incVal.ErrorTotal
	for k, v := range incVal.ErrorMap {
		newUpdate["$inc"].(model.DBM)[constructor+"errormap."+k] = v
	}
	newUpdate["$inc"].(model.DBM)[constructor+"totalrequesttime"] = incVal.TotalRequestTime
	newUpdate["$set"].(model.DBM)[constructor+"identifier"] = incVal.Identifier
	newUpdate["$set"].(model.DBM)[constructor+"humanidentifier"] = incVal.HumanIdentifier
	newUpdate["$set"].(model.DBM)[constructor+"lasttime"] = incVal.LastTime
	newUpdate["$set"].(model.DBM)[constructor+"openconnections"] = incVal.OpenConnections
	newUpdate["$set"].(model.DBM)[constructor+"closedconnections"] = incVal.ClosedConnections
	newUpdate["$set"].(model.DBM)[constructor+"bytesin"] = incVal.BytesIn
	newUpdate["$set"].(model.DBM)[constructor+"bytesout"] = incVal.BytesOut
	newUpdate["$max"].(model.DBM)[constructor+"maxlatency"] = incVal.MaxLatency
	// Don't update min latency in case of errors
	if incVal.Hits != incVal.ErrorTotal {
		if newUpdate["$min"] == nil {
			newUpdate["$min"] = model.DBM{}
		}
		newUpdate["$min"].(model.DBM)[constructor+"minlatency"] = incVal.MinLatency
		newUpdate["$min"].(model.DBM)[constructor+"minupstreamlatency"] = incVal.MinUpstreamLatency
	}
	newUpdate["$max"].(model.DBM)[constructor+"maxupstreamlatency"] = incVal.MaxUpstreamLatency
	newUpdate["$inc"].(model.DBM)[constructor+"totalupstreamlatency"] = incVal.TotalUpstreamLatency
	newUpdate["$inc"].(model.DBM)[constructor+"totallatency"] = incVal.TotalLatency

	return newUpdate
}

func (f *AnalyticsRecordAggregate) generateSetterForTime(parent, thisUnit string, realTime float64, newUpdate model.DBM) model.DBM {
	constructor := parent + "." + thisUnit + "."
	if parent == "" {
		constructor = thisUnit + "."
	}
	newUpdate["$set"].(model.DBM)[constructor+"requesttime"] = realTime

	return newUpdate
}

func (f *AnalyticsRecordAggregate) latencySetter(parent, thisUnit string, newUpdate model.DBM, counter *Counter) model.DBM {
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
	newUpdate["$set"].(model.DBM)[constructor+"latency"] = counter.Latency
	newUpdate["$set"].(model.DBM)[constructor+"upstreamlatency"] = counter.UpstreamLatency

	return newUpdate
}

type Dimension struct {
	Name    string
	Value   string
	Counter *Counter
}

func fnLatencySetter(counter *Counter) *Counter {
	if counter.Hits > 0 {
		counter.Latency = float64(counter.TotalLatency) / float64(counter.Hits)
		counter.UpstreamLatency = float64(counter.TotalUpstreamLatency) / float64(counter.Hits)
	}
	return counter
}

func (g *GraphRecordAggregate) Dimensions() []Dimension {
	dimensions := g.AnalyticsRecordAggregate.Dimensions()
	for key, inc := range g.Types {
		dimensions = append(dimensions, Dimension{Name: "types", Value: key, Counter: fnLatencySetter(inc)})
	}

	for key, inc := range g.Fields {
		dimensions = append(dimensions, Dimension{Name: "fields", Value: key, Counter: fnLatencySetter(inc)})
	}

	for key, inc := range g.Operation {
		dimensions = append(dimensions, Dimension{Name: "operation", Value: key, Counter: fnLatencySetter(inc)})
	}

	for key, inc := range g.RootFields {
		dimensions = append(dimensions, Dimension{Name: "rootfields", Value: key, Counter: fnLatencySetter(inc)})
	}

	return dimensions
}

func (f *AnalyticsRecordAggregate) Dimensions() (dimensions []Dimension) {
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

func (f *AnalyticsRecordAggregate) AsChange() (newUpdate model.DBM) {
	newUpdate = model.DBM{
		"$inc": model.DBM{},
		"$set": model.DBM{},
		"$max": model.DBM{},
	}

	for _, d := range f.Dimensions() {
		newUpdate = f.generateBSONFromProperty(d.Name, d.Value, d.Counter, newUpdate)
	}

	newUpdate = f.generateBSONFromProperty("", "total", &f.Total, newUpdate)

	asTime := f.TimeStamp
	newTime := time.Date(asTime.Year(), asTime.Month(), asTime.Day(), asTime.Hour(), asTime.Minute(), 0, 0, asTime.Location())
	newUpdate["$set"].(model.DBM)["timestamp"] = newTime
	newUpdate["$set"].(model.DBM)["expireAt"] = f.ExpireAt
	newUpdate["$set"].(model.DBM)["timeid.year"] = newTime.Year()
	newUpdate["$set"].(model.DBM)["timeid.month"] = newTime.Month()
	newUpdate["$set"].(model.DBM)["timeid.day"] = newTime.Day()
	newUpdate["$set"].(model.DBM)["timeid.hour"] = newTime.Hour()
	newUpdate["$set"].(model.DBM)["lasttime"] = f.LastTime

	return newUpdate
}

func (f *AnalyticsRecordAggregate) SetErrorList(parent, thisUnit string, counter *Counter, newUpdate model.DBM) {
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
	sort.SliceStable(errorlist, func(i, j int) bool {
		return errorlist[i].Code < errorlist[j].Code
	})

	counter.ErrorList = errorlist

	newUpdate["$set"].(model.DBM)[constructor+"errorlist"] = counter.ErrorList
}

func (f *AnalyticsRecordAggregate) getRecords(fieldName string, data map[string]*Counter, newUpdate model.DBM) []Counter {
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

func (f *AnalyticsRecordAggregate) AsTimeUpdate() model.DBM {
	newUpdate := model.DBM{
		"$set": model.DBM{},
	}

	// We need to create lists of API data so that we can aggregate across the list
	// in order to present top-20 style lists of APIs, Tokens etc.
	// apis := make([]Counter, 0)
	newUpdate["$set"].(model.DBM)["lists.apiid"] = f.getRecords("apiid", f.APIID, newUpdate)

	newUpdate["$set"].(model.DBM)["lists.errors"] = f.getRecords("errors", f.Errors, newUpdate)

	newUpdate["$set"].(model.DBM)["lists.versions"] = f.getRecords("versions", f.Versions, newUpdate)

	newUpdate["$set"].(model.DBM)["lists.apikeys"] = f.getRecords("apikeys", f.APIKeys, newUpdate)

	newUpdate["$set"].(model.DBM)["lists.oauthids"] = f.getRecords("oauthids", f.OauthIDs, newUpdate)

	newUpdate["$set"].(model.DBM)["lists.geo"] = f.getRecords("geo", f.Geo, newUpdate)

	newUpdate["$set"].(model.DBM)["lists.tags"] = f.getRecords("tags", f.Tags, newUpdate)

	newUpdate["$set"].(model.DBM)["lists.endpoints"] = f.getRecords("endpoints", f.Endpoints, newUpdate)

	for thisUnit, incVal := range f.KeyEndpoint {
		parent := "lists.keyendpoints." + thisUnit
		newUpdate["$set"].(model.DBM)[parent] = f.getRecords("keyendpoints."+thisUnit, incVal, newUpdate)
	}

	for thisUnit, incVal := range f.OauthEndpoint {
		parent := "lists.oauthendpoints." + thisUnit
		newUpdate["$set"].(model.DBM)[parent] = f.getRecords("oauthendpoints."+thisUnit, incVal, newUpdate)
	}

	newUpdate["$set"].(model.DBM)["lists.apiendpoints"] = f.getRecords("apiendpoints", f.ApiEndpoint, newUpdate)

	var newTime float64

	if f.Total.Hits > 0 {
		newTime = f.Total.TotalRequestTime / float64(f.Total.Hits)
	}
	f.SetErrorList("", "total", &f.Total, newUpdate)
	newUpdate = f.generateSetterForTime("", "total", newTime, newUpdate)
	newUpdate = f.latencySetter("", "total", newUpdate, &f.Total)

	return newUpdate
}

// DiscardAggregations this method discard the aggregations of X field specified in the aggregated pump configuration
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

func AggregateGraphData(data []interface{}, dbIdentifier string, aggregationTime int) map[string]GraphRecordAggregate {
	aggregateMap := make(map[string]GraphRecordAggregate)

	for _, item := range data {
		record, ok := item.(AnalyticsRecord)
		if !ok {
			continue
		}

		if !record.IsGraphRecord() {
			continue
		}

		graphRec := record.ToGraphRecord()

		aggregate, found := aggregateMap[record.OrgID]
		if !found {
			aggregate = NewGraphRecordAggregate()

			// Set the hourly timestamp & expiry
			asTime := record.TimeStamp
			aggregate.TimeStamp = setAggregateTimestamp(dbIdentifier, asTime, aggregationTime)
			aggregate.ExpireAt = record.ExpireAt
			aggregate.TimeID.Year = asTime.Year()
			aggregate.TimeID.Month = int(asTime.Month())
			aggregate.TimeID.Day = asTime.Day()
			aggregate.TimeID.Hour = asTime.Hour()
			aggregate.OrgID = record.OrgID
			aggregate.LastTime = record.TimeStamp
			aggregate.Total.ErrorMap = make(map[string]int)
		}

		var counter Counter
		aggregate.AnalyticsRecordAggregate, counter = incrementAggregate(&aggregate.AnalyticsRecordAggregate, &graphRec.AnalyticsRecord, false, nil)
		// graph errors are different from http status errors and can occur even if a response is gotten.
		// check for graph errors and increment the error count if there are indeed graph errors
		if graphRec.HasErrors && counter.ErrorTotal < 1 {
			counter.ErrorTotal++
			counter.Success--
		}
		c := incrementOrSetUnit(&counter, aggregate.Operation[graphRec.OperationType])
		aggregate.Operation[graphRec.OperationType] = c
		aggregate.Operation[graphRec.OperationType].Identifier = graphRec.OperationType
		aggregate.Operation[graphRec.OperationType].HumanIdentifier = graphRec.OperationType

		for t, fields := range graphRec.Types {
			c = incrementOrSetUnit(&counter, aggregate.Types[t])
			aggregate.Types[t] = c
			aggregate.Types[t].Identifier = t
			aggregate.Types[t].HumanIdentifier = t
			for _, f := range fields {
				label := fmt.Sprintf("%s_%s", t, f)
				c := incrementOrSetUnit(&counter, aggregate.Fields[label])
				aggregate.Fields[label] = c
				aggregate.Fields[label].Identifier = label
				aggregate.Fields[label].HumanIdentifier = label
			}
		}

		for _, field := range graphRec.RootFields {
			c = incrementOrSetUnit(&counter, aggregate.RootFields[field])
			aggregate.RootFields[field] = c
			aggregate.RootFields[field].Identifier = field
			aggregate.RootFields[field].HumanIdentifier = field
		}
		aggregateMap[record.OrgID] = aggregate
	}
	return aggregateMap
}

// AggregateData calculates aggregated data, returns map orgID => aggregated analytics data
func AggregateData(data []interface{}, trackAllPaths bool, ignoreTagPrefixList []string, dbIdentifier string, aggregationTime int) map[string]AnalyticsRecordAggregate {
	analyticsPerOrg := make(map[string]AnalyticsRecordAggregate)
	for _, v := range data {
		thisV := v.(AnalyticsRecord)
		orgID := thisV.OrgID

		if orgID == "" {
			continue
		}

		// We don't want to aggregate Graph Data with REST data - there is a different type for that.
		if thisV.IsGraphRecord() {
			continue
		}

		thisAggregate, found := analyticsPerOrg[orgID]

		if !found {
			thisAggregate = AnalyticsRecordAggregate{}.New()

			// Set the hourly timestamp & expiry
			asTime := thisV.TimeStamp
			thisAggregate.TimeStamp = setAggregateTimestamp(dbIdentifier, asTime, aggregationTime)
			thisAggregate.ExpireAt = thisV.ExpireAt
			thisAggregate.TimeID.Year = asTime.Year()
			thisAggregate.TimeID.Month = int(asTime.Month())
			thisAggregate.TimeID.Day = asTime.Day()
			thisAggregate.TimeID.Hour = asTime.Hour()
			thisAggregate.OrgID = orgID
			thisAggregate.LastTime = thisV.TimeStamp
			thisAggregate.Total.ErrorMap = make(map[string]int)
		}
		thisAggregate, _ = incrementAggregate(&thisAggregate, &thisV, trackAllPaths, ignoreTagPrefixList)
		analyticsPerOrg[orgID] = thisAggregate
	}

	return analyticsPerOrg
}

// incrementAggregate increments the analytic record aggregate fields using the analytics record
func incrementAggregate(aggregate *AnalyticsRecordAggregate, record *AnalyticsRecord, trackAllPaths bool, ignoreTagPrefixList []string) (AnalyticsRecordAggregate, Counter) {
	// Always update the last timestamp
	aggregate.LastTime = record.TimeStamp
	aggregate.Total.LastTime = record.TimeStamp

	// Create the counter for this record
	var thisCounter Counter
	if record.ResponseCode == -1 {
		thisCounter = Counter{
			LastTime:          record.TimeStamp,
			OpenConnections:   record.Network.OpenConnections,
			ClosedConnections: record.Network.ClosedConnection,
			BytesIn:           record.Network.BytesIn,
			BytesOut:          record.Network.BytesOut,
		}
		aggregate.Total.OpenConnections += thisCounter.OpenConnections
		aggregate.Total.ClosedConnections += thisCounter.ClosedConnections
		aggregate.Total.BytesIn += thisCounter.BytesIn
		aggregate.Total.BytesOut += thisCounter.BytesOut
		if record.APIID != "" {
			c := aggregate.APIID[record.APIID]
			if c == nil {
				c = &Counter{
					Identifier:      record.APIID,
					HumanIdentifier: record.APIName,
				}
				aggregate.APIID[record.APIID] = c
			}
			c.BytesIn += thisCounter.BytesIn
			c.BytesOut += thisCounter.BytesOut
		}
	} else {
		thisCounter = Counter{
			Hits:             1,
			Success:          0,
			ErrorTotal:       0,
			RequestTime:      float64(record.RequestTime),
			TotalRequestTime: float64(record.RequestTime),
			LastTime:         record.TimeStamp,

			MaxUpstreamLatency:   record.Latency.Upstream,
			MinUpstreamLatency:   record.Latency.Upstream,
			TotalUpstreamLatency: record.Latency.Upstream,
			MaxLatency:           record.Latency.Total,
			MinLatency:           record.Latency.Total,
			TotalLatency:         record.Latency.Total,
			ErrorMap:             make(map[string]int),
		}
		aggregate.Total.Hits++
		aggregate.Total.TotalRequestTime += float64(record.RequestTime)

		// We need an initial value
		aggregate.Total.RequestTime = aggregate.Total.TotalRequestTime / float64(aggregate.Total.Hits)
		if record.ResponseCode >= 400 {
			thisCounter.ErrorTotal = 1
			thisCounter.ErrorMap[strconv.Itoa(record.ResponseCode)]++
			aggregate.Total.ErrorTotal++
			aggregate.Total.ErrorMap[strconv.Itoa(record.ResponseCode)]++
		}

		if (record.ResponseCode < 300) && (record.ResponseCode >= 200) {
			thisCounter.Success = 1
			aggregate.Total.Success++
		}

		aggregate.Total.TotalLatency += record.Latency.Total
		aggregate.Total.TotalUpstreamLatency += record.Latency.Upstream

		if aggregate.Total.MaxLatency < record.Latency.Total {
			aggregate.Total.MaxLatency = record.Latency.Total
		}

		if aggregate.Total.MaxUpstreamLatency < record.Latency.Upstream {
			aggregate.Total.MaxUpstreamLatency = record.Latency.Upstream
		}

		// by default, min_total_latency will have 0 value
		// it should not be set to 0 always
		if aggregate.Total.Hits == 1 {
			aggregate.Total.MinLatency = record.Latency.Total
			aggregate.Total.MinUpstreamLatency = record.Latency.Upstream
		} else {
			// Don't update min latency in case of error
			if aggregate.Total.MinLatency > record.Latency.Total && (record.ResponseCode < 300) && (record.ResponseCode >= 200) {
				aggregate.Total.MinLatency = record.Latency.Total
			}
			// Don't update min latency in case of error
			if aggregate.Total.MinUpstreamLatency > record.Latency.Upstream && (record.ResponseCode < 300) && (record.ResponseCode >= 200) {
				aggregate.Total.MinUpstreamLatency = record.Latency.Upstream
			}
		}

		if trackAllPaths {
			record.TrackPath = true
		}

		// Convert to a map (for easy iteration)
		vAsMap := structs.Map(record)
		for key, value := range vAsMap {
			switch key {
			case "APIID":
				val, ok := value.(string)
				c := incrementOrSetUnit(&thisCounter, aggregate.APIID[val])
				if val != "" && ok {
					aggregate.APIID[val] = c
					aggregate.APIID[val].Identifier = record.APIID
					aggregate.APIID[val].HumanIdentifier = record.APIName
				}
			case "ResponseCode":
				val, ok := value.(int)
				if !ok {
					break
				}
				errAsStr := strconv.Itoa(val)
				if errAsStr != "" {
					c := incrementOrSetUnit(&thisCounter, aggregate.Errors[errAsStr])
					if c.ErrorTotal > 0 {
						aggregate.Errors[errAsStr] = c
						aggregate.Errors[errAsStr].Identifier = errAsStr
					}
				}
			case "APIVersion":
				val, ok := value.(string)
				versionStr := doHash(record.APIID + ":" + val)
				c := incrementOrSetUnit(&thisCounter, aggregate.Versions[versionStr])
				if val != "" && ok {
					aggregate.Versions[versionStr] = c
					aggregate.Versions[versionStr].Identifier = val
					aggregate.Versions[versionStr].HumanIdentifier = val
				}
			case "APIKey":
				val, ok := value.(string)
				if val != "" && ok {
					c := incrementOrSetUnit(&thisCounter, aggregate.APIKeys[val])
					aggregate.APIKeys[val] = c
					aggregate.APIKeys[val].Identifier = val
					aggregate.APIKeys[val].HumanIdentifier = record.Alias

					if record.TrackPath {
						keyStr := doHash(record.APIID + ":" + record.Path)
						data := aggregate.KeyEndpoint[val]

						if data == nil {
							data = make(map[string]*Counter)
						}

						c = incrementOrSetUnit(&thisCounter, data[keyStr])
						c.Identifier = keyStr
						c.HumanIdentifier = keyStr
						data[keyStr] = c
						aggregate.KeyEndpoint[val] = data

					}
				}
			case "OauthID":
				val, ok := value.(string)
				if val != "" && ok {
					c := incrementOrSetUnit(&thisCounter, aggregate.OauthIDs[val])
					aggregate.OauthIDs[val] = c
					aggregate.OauthIDs[val].Identifier = val

					if record.TrackPath {
						keyStr := doHash(record.APIID + ":" + record.Path)
						data := aggregate.OauthEndpoint[val]

						if data == nil {
							data = make(map[string]*Counter)
						}

						c = incrementOrSetUnit(&thisCounter, data[keyStr])
						c.Identifier = keyStr
						c.HumanIdentifier = keyStr
						data[keyStr] = c
						aggregate.OauthEndpoint[val] = data
					}
				}
			case "Geo":
				c := incrementOrSetUnit(&thisCounter, aggregate.Geo[record.Geo.Country.ISOCode])
				if record.Geo.Country.ISOCode != "" {
					aggregate.Geo[record.Geo.Country.ISOCode] = c
					aggregate.Geo[record.Geo.Country.ISOCode].Identifier = record.Geo.Country.ISOCode
					aggregate.Geo[record.Geo.Country.ISOCode].HumanIdentifier = record.Geo.Country.ISOCode
				}

			case "Tags":
				for _, thisTag := range record.Tags {
					trimmedTag := TrimTag(thisTag)

					if trimmedTag != "" && !ignoreTag(thisTag, ignoreTagPrefixList) {
						c := incrementOrSetUnit(&thisCounter, aggregate.Tags[trimmedTag])
						aggregate.Tags[trimmedTag] = c
						aggregate.Tags[trimmedTag].Identifier = trimmedTag
						aggregate.Tags[trimmedTag].HumanIdentifier = trimmedTag
					}
				}

			case "TrackPath":
				val, ok := value.(bool)
				if !ok {
					break
				}
				log.Debug("TrackPath=", val)
				if val {
					fixedPath := replaceUnsupportedChars(record.Path)
					c := incrementOrSetUnit(&thisCounter, aggregate.Endpoints[fixedPath])
					aggregate.Endpoints[fixedPath] = c
					aggregate.Endpoints[fixedPath].Identifier = record.Path
					aggregate.Endpoints[fixedPath].HumanIdentifier = record.Path

					keyStr := hex.EncodeToString([]byte(record.APIID + ":" + record.APIVersion + ":" + record.Path))
					c = incrementOrSetUnit(&thisCounter, aggregate.ApiEndpoint[keyStr])
					aggregate.ApiEndpoint[keyStr] = c
					aggregate.ApiEndpoint[keyStr].Identifier = keyStr
					aggregate.ApiEndpoint[keyStr].HumanIdentifier = record.Path
				}
			}
		}
	}
	return *aggregate, thisCounter
}

// incrementOrSetUnit is a Mini function to handle incrementing a specific counter in our object
func incrementOrSetUnit(b, c *Counter) *Counter {
	base := *b
	if c == nil {
		newCounter := base
		newCounter.ErrorMap = make(map[string]int)
		for k, v := range base.ErrorMap {
			newCounter.ErrorMap[k] = v
		}
		c = &newCounter
	} else {
		c.Hits += base.Hits
		c.Success += base.Success
		c.ErrorTotal += base.ErrorTotal
		for k, v := range base.ErrorMap {
			c.ErrorMap[k] += v
		}
		c.TotalRequestTime += base.TotalRequestTime
		c.RequestTime = c.TotalRequestTime / float64(c.Hits)

		if c.MaxLatency < base.MaxLatency {
			c.MaxLatency = base.MaxLatency
		}

		// don't update min latency in case of errors
		if c.MinLatency > base.MinLatency && base.ErrorTotal == 0 {
			c.MinLatency = base.MinLatency
		}

		if c.MaxUpstreamLatency < base.MaxUpstreamLatency {
			c.MaxUpstreamLatency = base.MaxUpstreamLatency
		}

		// don't update min latency in case of errors
		if c.MinUpstreamLatency > base.MinUpstreamLatency && base.ErrorTotal == 0 {
			c.MinUpstreamLatency = base.MinUpstreamLatency
		}

		c.TotalLatency += base.TotalLatency
		c.TotalUpstreamLatency += base.TotalUpstreamLatency

	}

	return c
}

func TrimTag(thisTag string) string {
	trimmedTag := strings.TrimSpace(thisTag)

	trimmedTag = strings.ReplaceAll(trimmedTag, ".", "")
	return trimmedTag
}

// SetlastTimestampAgggregateRecord sets the last timestamp for the aggregate record
func SetlastTimestampAgggregateRecord(id string, date time.Time) {
	mutex.Lock()
	defer mutex.Unlock()
	lastDocumentTimestamp[id] = date
}

// getLastDocumentTimestamp gets the last timestamp for the aggregate record
func getLastDocumentTimestamp(id string) (time.Time, bool) {
	mutex.RLock()
	defer mutex.RUnlock()
	ts, ok := lastDocumentTimestamp[id]
	return ts, ok
}

func setAggregateTimestamp(dbIdentifier string, asTime time.Time, aggregationTime int) time.Time {
	// if aggregationTime is set to 60, use asTime.Hour() and group every record by hour
	if aggregationTime == 60 {
		return time.Date(asTime.Year(), asTime.Month(), asTime.Day(), asTime.Hour(), 0, 0, 0, asTime.Location())
	}

	// get the last document timestamp
	lastDocumentTS, ok := getLastDocumentTimestamp(dbIdentifier)
	emptyTime := time.Time{}
	if lastDocumentTS == emptyTime || !ok {
		// if it's not set, or it's empty, just set it to the current time
		lastDocumentTS = time.Date(asTime.Year(), asTime.Month(), asTime.Day(), asTime.Hour(), asTime.Minute(), 0, 0, asTime.Location())
		SetlastTimestampAgggregateRecord(dbIdentifier, lastDocumentTS)
	}
	if dbIdentifier != "" {
		// if aggregationTime != 60 and the database is Mongo (because we have an identifier):
		if lastDocumentTS.Add(time.Minute * time.Duration(aggregationTime)).After(asTime) {
			// if the last record timestamp + aggregationTime setting is after the current time, just add the new record to the current document
			return lastDocumentTS
		}
		// if last record timestamp + amount of minutes set is before current time, just create a new record
		newTime := time.Date(asTime.Year(), asTime.Month(), asTime.Day(), asTime.Hour(), asTime.Minute(), 0, 0, asTime.Location())
		SetlastTimestampAgggregateRecord(dbIdentifier, newTime)
		return newTime
	}
	// if aggregationTime is set to 1 and DB is not Mongo, use asTime.Minute() and group every record by minute
	return time.Date(asTime.Year(), asTime.Month(), asTime.Day(), asTime.Hour(), asTime.Minute(), 0, 0, asTime.Location())
}
