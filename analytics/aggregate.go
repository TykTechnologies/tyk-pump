package analytics

import (
	b64 "encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/structs"
	"gopkg.in/mgo.v2/bson"
)

const (
	AgggregateMixedCollectionName = "tyk_analytics_aggregates"
	MongoAggregatePrefix          = "mongo-pump-aggregate"
)

type Counter struct {
	Hits              int       `json:"hits"`
	Success           int       `json:"success"`
	ErrorTotal        int       `json:"error"`
	RequestTime       float64   `json:"request_time"`
	TotalRequestTime  float64   `json:"total_request_time"`
	Identifier        string    `json:"identifier"`
	HumanIdentifier   string    `json:"human_identifier"`
	LastTime          time.Time `json:"last_time"`
	OpenConnections   int64     `json:"open_connections"`
	ClosedConnections int64     `json:"closed_connections"`
	BytesIn           int64     `json:"bytes_in"`
	BytesOut          int64     `json:"bytes_out"`

	MaxUpstreamLatency   int64   `json:"max_upstream_latency"`
	MinUpstreamLatency   int64   `json:"min_upstream_latency"`
	TotalUpstreamLatency int64   `json:"total_upstream_latency"`
	UpstreamLatency      float64 `json:"upstream_latency"`

	MaxLatency   int64   `json:"max_latency"`
	MinLatency   int64   `json:"min_latency"`
	TotalLatency int64   `json:"total_latency"`
	Latency      float64 `json:"latency"`
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
	}

	KeyEndpoint   map[string]map[string]*Counter `bson:"keyendpoints"`
	OauthEndpoint map[string]map[string]*Counter `bson:"oauthendpoints"`

	Total Counter

	ExpireAt time.Time `bson:"expireAt" json:"expireAt"`
	LastTime time.Time
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
		newUpdate["$min"] = bson.M{}
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
	counter.Latency = float64(counter.TotalLatency) / float64(counter.Hits)
	counter.UpstreamLatency = float64(counter.TotalUpstreamLatency) / float64(counter.Hits)

	constructor := parent + "." + thisUnit + "."
	if parent == "" {
		constructor = thisUnit + "."
	}
	newUpdate["$set"].(bson.M)[constructor+"latency"] = counter.Latency
	newUpdate["$set"].(bson.M)[constructor+"upstreamlatency"] = counter.UpstreamLatency

	return newUpdate
}

func (f *AnalyticsRecordAggregate) AsChange() bson.M {
	newUpdate := bson.M{
		"$inc": bson.M{},
		"$set": bson.M{},
		"$max": bson.M{},
	}

	for thisUnit, incVal := range f.APIID {
		newUpdate = f.generateBSONFromProperty("apiid", thisUnit, incVal, newUpdate)
	}

	for thisUnit, incVal := range f.Errors {
		newUpdate = f.generateBSONFromProperty("errors", thisUnit, incVal, newUpdate)
	}

	for thisUnit, incVal := range f.Versions {
		newUpdate = f.generateBSONFromProperty("versions", thisUnit, incVal, newUpdate)
	}

	for thisUnit, incVal := range f.APIKeys {
		newUpdate = f.generateBSONFromProperty("apikeys", thisUnit, incVal, newUpdate)
	}

	for thisUnit, incVal := range f.OauthIDs {
		newUpdate = f.generateBSONFromProperty("oauthids", thisUnit, incVal, newUpdate)
	}

	for thisUnit, incVal := range f.Geo {
		newUpdate = f.generateBSONFromProperty("geo", thisUnit, incVal, newUpdate)
	}

	for thisUnit, incVal := range f.Tags {
		newUpdate = f.generateBSONFromProperty("tags", thisUnit, incVal, newUpdate)
	}

	for thisUnit, incVal := range f.Endpoints {
		newUpdate = f.generateBSONFromProperty("endpoints", thisUnit, incVal, newUpdate)
	}

	for thisUnit, incVal := range f.KeyEndpoint {
		parent := "keyendpoints." + thisUnit
		for k, v := range incVal {
			newUpdate = f.generateBSONFromProperty(parent, k, v, newUpdate)
		}
	}

	for thisUnit, incVal := range f.OauthEndpoint {
		parent := "oauthendpoints." + thisUnit
		for k, v := range incVal {
			newUpdate = f.generateBSONFromProperty(parent, k, v, newUpdate)
		}
	}

	newUpdate = f.generateBSONFromProperty("", "total", &f.Total, newUpdate)

	asTime := f.TimeStamp
	newTime := time.Date(asTime.Year(), asTime.Month(), asTime.Day(), asTime.Hour(), 0, 0, 0, asTime.Location())
	newUpdate["$set"].(bson.M)["timestamp"] = newTime
	newUpdate["$set"].(bson.M)["expireAt"] = f.ExpireAt
	newUpdate["$set"].(bson.M)["timeid.year"] = newTime.Year()
	newUpdate["$set"].(bson.M)["timeid.month"] = newTime.Month()
	newUpdate["$set"].(bson.M)["timeid.day"] = newTime.Day()
	newUpdate["$set"].(bson.M)["timeid.hour"] = newTime.Hour()
	newUpdate["$set"].(bson.M)["lasttime"] = f.LastTime

	return newUpdate
}

func (f *AnalyticsRecordAggregate) AsTimeUpdate() bson.M {
	newUpdate := bson.M{
		"$set": bson.M{},
	}

	// We need to create lists of API data so that we can aggregate across the list
	// in order to present top-20 style lists of APIs, Tokens etc.
	apis := make([]Counter, 0)
	newUpdate["$set"].(bson.M)["lists.apiid"] = make([]interface{}, 0)
	for thisUnit, incVal := range f.APIID {
		newTime := incVal.TotalRequestTime / float64(incVal.Hits)
		newUpdate = f.generateSetterForTime("apiid", thisUnit, newTime, newUpdate)
		newUpdate = f.latencySetter("apiid", thisUnit, newUpdate, incVal)
		apis = append(apis, *incVal)
	}
	newUpdate["$set"].(bson.M)["lists.apiid"] = apis

	errors := make([]Counter, 0)
	newUpdate["$set"].(bson.M)["lists.errors"] = make([]interface{}, 0)
	for thisUnit, incVal := range f.Errors {
		newTime := incVal.TotalRequestTime / float64(incVal.Hits)
		newUpdate = f.generateSetterForTime("errors", thisUnit, newTime, newUpdate)
		newUpdate = f.latencySetter("errors", thisUnit, newUpdate, incVal)
		errors = append(errors, *incVal)
	}
	newUpdate["$set"].(bson.M)["lists.errors"] = errors

	versions := make([]Counter, 0)
	newUpdate["$set"].(bson.M)["lists.versions"] = make([]interface{}, 0)
	for thisUnit, incVal := range f.Versions {
		newTime := incVal.TotalRequestTime / float64(incVal.Hits)
		newUpdate = f.generateSetterForTime("versions", thisUnit, newTime, newUpdate)
		newUpdate = f.latencySetter("versions", thisUnit, newUpdate, incVal)
		versions = append(versions, *incVal)
	}
	newUpdate["$set"].(bson.M)["lists.versions"] = versions

	apikeys := make([]Counter, 0)
	newUpdate["$set"].(bson.M)["lists.apikeys"] = make([]interface{}, 0)
	for thisUnit, incVal := range f.APIKeys {
		newTime := incVal.TotalRequestTime / float64(incVal.Hits)
		newUpdate = f.generateSetterForTime("apikeys", thisUnit, newTime, newUpdate)
		newUpdate = f.latencySetter("apikeys", thisUnit, newUpdate, incVal)
		apikeys = append(apikeys, *incVal)
	}
	newUpdate["$set"].(bson.M)["lists.apikeys"] = apikeys

	oauthids := make([]Counter, 0)
	newUpdate["$set"].(bson.M)["lists.oauthids"] = make([]interface{}, 0)
	for thisUnit, incVal := range f.OauthIDs {
		newTime := incVal.TotalRequestTime / float64(incVal.Hits)
		newUpdate = f.generateSetterForTime("oauthids", thisUnit, newTime, newUpdate)
		newUpdate = f.latencySetter("oauthids", thisUnit, newUpdate, incVal)
		oauthids = append(oauthids, *incVal)
	}
	newUpdate["$set"].(bson.M)["lists.oauthids"] = oauthids

	geo := make([]Counter, 0)
	newUpdate["$set"].(bson.M)["lists.geo"] = make([]interface{}, 0)
	for thisUnit, incVal := range f.Geo {
		newTime := incVal.TotalRequestTime / float64(incVal.Hits)
		newUpdate = f.generateSetterForTime("geo", thisUnit, newTime, newUpdate)
		newUpdate = f.latencySetter("geo", thisUnit, newUpdate, incVal)
		geo = append(geo, *incVal)
	}
	newUpdate["$set"].(bson.M)["lists.geo"] = geo

	tags := make([]Counter, 0)
	newUpdate["$set"].(bson.M)["lists.tags"] = make([]interface{}, 0)
	for thisUnit, incVal := range f.Tags {
		newTime := incVal.TotalRequestTime / float64(incVal.Hits)
		newUpdate = f.generateSetterForTime("tags", thisUnit, newTime, newUpdate)
		newUpdate = f.latencySetter("tags", thisUnit, newUpdate, incVal)
		tags = append(tags, *incVal)
	}
	newUpdate["$set"].(bson.M)["lists.tags"] = tags

	endpoints := make([]Counter, 0)
	newUpdate["$set"].(bson.M)["lists.endpoints"] = make([]interface{}, 0)
	for thisUnit, incVal := range f.Endpoints {
		newTime := incVal.TotalRequestTime / float64(incVal.Hits)
		newUpdate = f.generateSetterForTime("endpoints", thisUnit, newTime, newUpdate)
		newUpdate = f.latencySetter("endpoints", thisUnit, newUpdate, incVal)
		endpoints = append(endpoints, *incVal)
	}
	newUpdate["$set"].(bson.M)["lists.endpoints"] = endpoints

	for thisUnit, incVal := range f.KeyEndpoint {
		parent := "lists.keyendpoints." + thisUnit
		keyendpoints := make([]Counter, 0)

		newUpdate["$set"].(bson.M)[parent] = make([]interface{}, 0)
		for k, v := range incVal {
			newTime := v.TotalRequestTime / float64(v.Hits)
			newUpdate = f.generateSetterForTime("keyendpoints."+thisUnit, k, newTime, newUpdate)
			newUpdate = f.latencySetter("keyendpoints."+thisUnit, k, newUpdate, v)
			keyendpoints = append(keyendpoints, *v)
		}
		newUpdate["$set"].(bson.M)[parent] = keyendpoints
	}

	for thisUnit, incVal := range f.OauthEndpoint {
		parent := "lists.oauthendpoints." + thisUnit
		oauthendpoints := make([]Counter, 0)

		newUpdate["$set"].(bson.M)[parent] = make([]interface{}, 0)
		for k, v := range incVal {
			newTime := v.TotalRequestTime / float64(v.Hits)
			newUpdate = f.generateSetterForTime("oauthendpoints."+thisUnit, k, newTime, newUpdate)
			newUpdate = f.latencySetter("oauthendpoints."+thisUnit, k, newUpdate, v)
			oauthendpoints = append(oauthendpoints, *v)
		}
		newUpdate["$set"].(bson.M)[parent] = oauthendpoints
	}

	newTime := f.Total.TotalRequestTime / float64(f.Total.Hits)
	newUpdate = f.generateSetterForTime("", "total", newTime, newUpdate)
	newUpdate = f.latencySetter("", "total", newUpdate, &f.Total)

	return newUpdate
}

func doHash(in string) string {
	sEnc := b64.StdEncoding.EncodeToString([]byte(in))
	search := strings.TrimRight(sEnc, "=")
	return search
}

// AggregateData calculates aggregated data, returns map orgID => aggregated analytics data
func AggregateData(data []interface{}, trackAllPaths bool) map[string]AnalyticsRecordAggregate {
	analyticsPerOrg := make(map[string]AnalyticsRecordAggregate)

	for _, v := range data {
		thisV := v.(AnalyticsRecord)
		orgID := thisV.OrgID

		if orgID == "" {
			continue
		}

		thisAggregate, found := analyticsPerOrg[orgID]

		if !found {
			thisAggregate = AnalyticsRecordAggregate{}.New()

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
		}

		// Always update the last timestamp
		thisAggregate.LastTime = thisV.TimeStamp

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
			}
			thisAggregate.Total.Hits++
			thisAggregate.Total.TotalRequestTime += float64(thisV.RequestTime)

			// We need an initial value
			thisAggregate.Total.RequestTime = thisAggregate.Total.TotalRequestTime / float64(thisAggregate.Total.Hits)
			if thisV.ResponseCode > 400 {
				thisCounter.ErrorTotal = 1
				thisAggregate.Total.ErrorTotal++
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
						c = &newCounter
					} else {
						c.Hits += thisCounter.Hits
						c.Success += thisCounter.Success
						c.ErrorTotal += thisCounter.ErrorTotal
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
						c := IncrementOrSetUnit(thisAggregate.Tags[thisTag])
						thisAggregate.Tags[thisTag] = c
						thisAggregate.Tags[thisTag].Identifier = thisTag
						thisAggregate.Tags[thisTag].HumanIdentifier = thisTag
					}
					break

				case "TrackPath":
					if value.(bool) {
						c := IncrementOrSetUnit(thisAggregate.Endpoints[thisV.Path])
						thisAggregate.Endpoints[thisV.Path] = c
						thisAggregate.Endpoints[thisV.Path].Identifier = thisV.Path
						thisAggregate.Endpoints[thisV.Path].HumanIdentifier = thisV.Path
					}
					break
				}
			}

		}

		analyticsPerOrg[orgID] = thisAggregate

	}

	return analyticsPerOrg
}
