package pumps

import (
	b64 "encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/structs"
	"github.com/kelseyhightower/envconfig"
	"github.com/lonelycode/mgohacks"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
)

const (
	AgggregateMixedCollectionName string = "tyk_analytics_aggregates"
)

var mongoAggregatePumpPrefix = "PMP_MONGOAGG"

type Counter struct {
	Hits             int       `json:"hits"`
	Success          int       `json:"success"`
	ErrorTotal       int       `json:"error"`
	RequestTime      float64   `json:"request_time"`
	TotalRequestTime float64   `json:"total_request_time"`
	Identifier       string    `json:"identifier"`
	HumanIdentifier  string    `json:"human_identifier"`
	LastTime         time.Time `json:"last_time"`
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
		APIKeys   []Counter
		APIID     []Counter
		OauthIDs  []Counter
		Geo       []Counter
		Tags      []Counter
		Errors    []Counter
		Endpoints []Counter
	}

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

func (f *AnalyticsRecordAggregate) AsChange() bson.M {
	newUpdate := bson.M{
		"$inc": bson.M{},
		"$set": bson.M{},
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
		apis = append(apis, *incVal)
	}
	newUpdate["$set"].(bson.M)["lists.apiid"] = apis

	errors := make([]Counter, 0)
	newUpdate["$set"].(bson.M)["lists.errors"] = make([]interface{}, 0)
	for thisUnit, incVal := range f.Errors {
		newTime := incVal.TotalRequestTime / float64(incVal.Hits)
		newUpdate = f.generateSetterForTime("errors", thisUnit, newTime, newUpdate)
		errors = append(errors, *incVal)
	}
	newUpdate["$set"].(bson.M)["lists.errors"] = errors

	versions := make([]Counter, 0)
	newUpdate["$set"].(bson.M)["lists.versions"] = make([]interface{}, 0)
	for thisUnit, incVal := range f.Versions {
		newTime := incVal.TotalRequestTime / float64(incVal.Hits)
		newUpdate = f.generateSetterForTime("versions", thisUnit, newTime, newUpdate)
		versions = append(versions, *incVal)
	}
	newUpdate["$set"].(bson.M)["lists.versions"] = versions

	apikeys := make([]Counter, 0)
	newUpdate["$set"].(bson.M)["lists.apikeys"] = make([]interface{}, 0)
	for thisUnit, incVal := range f.APIKeys {
		newTime := incVal.TotalRequestTime / float64(incVal.Hits)
		newUpdate = f.generateSetterForTime("apikeys", thisUnit, newTime, newUpdate)
		apikeys = append(apikeys, *incVal)
	}
	newUpdate["$set"].(bson.M)["lists.apikeys"] = apikeys

	oauthids := make([]Counter, 0)
	newUpdate["$set"].(bson.M)["lists.oauthids"] = make([]interface{}, 0)
	for thisUnit, incVal := range f.OauthIDs {
		newTime := incVal.TotalRequestTime / float64(incVal.Hits)
		newUpdate = f.generateSetterForTime("oauthids", thisUnit, newTime, newUpdate)
		oauthids = append(oauthids, *incVal)
	}
	newUpdate["$set"].(bson.M)["lists.oauthids"] = oauthids

	geo := make([]Counter, 0)
	newUpdate["$set"].(bson.M)["lists.geo"] = make([]interface{}, 0)
	for thisUnit, incVal := range f.Geo {
		newTime := incVal.TotalRequestTime / float64(incVal.Hits)
		newUpdate = f.generateSetterForTime("geo", thisUnit, newTime, newUpdate)
		geo = append(geo, *incVal)
	}
	newUpdate["$set"].(bson.M)["lists.geo"] = geo

	tags := make([]Counter, 0)
	newUpdate["$set"].(bson.M)["lists.tags"] = make([]interface{}, 0)
	for thisUnit, incVal := range f.Tags {
		newTime := incVal.TotalRequestTime / float64(incVal.Hits)
		newUpdate = f.generateSetterForTime("tags", thisUnit, newTime, newUpdate)
		tags = append(tags, *incVal)
	}
	newUpdate["$set"].(bson.M)["lists.tags"] = tags

	endpoints := make([]Counter, 0)
	newUpdate["$set"].(bson.M)["lists.endpoints"] = make([]interface{}, 0)
	for thisUnit, incVal := range f.Endpoints {
		newTime := incVal.TotalRequestTime / float64(incVal.Hits)
		newUpdate = f.generateSetterForTime("endpoints", thisUnit, newTime, newUpdate)
		endpoints = append(endpoints, *incVal)
	}
	newUpdate["$set"].(bson.M)["lists.endpoints"] = endpoints

	newTime := f.Total.TotalRequestTime / float64(f.Total.Hits)
	newUpdate = f.generateSetterForTime("", "total", newTime, newUpdate)

	return newUpdate
}

type MongoAggregatePump struct {
	dbSession *mgo.Session
	dbConf    *MongoAggregateConf
}

var mongoAggregatePrefix = "mongo-pump-aggregate"

type MongoAggregateConf struct {
	MongoURL                   string `mapstructure:"mongo_url"`
	MongoUseSSL                bool   `mapstructure:"mongo_use_ssl"`
	MongoSSLInsecureSkipVerify bool   `mapstructure:"mongo_ssl_insecure_skip_verify"`
	UseMixedCollection         bool   `mapstructure:"use_mixed_collection"`
}

func (m *MongoAggregatePump) New() Pump {
	newPump := MongoAggregatePump{}
	return &newPump
}

func (m *MongoAggregatePump) doHash(in string) string {
	sEnc := b64.StdEncoding.EncodeToString([]byte(in))
	search := strings.TrimRight(sEnc, "=")
	return search
}

func (m *MongoAggregatePump) GetName() string {
	return "MongoDB Aggregate Pump"
}

func (m *MongoAggregatePump) GetCollectionName(orgid string) (string, error) {
	if orgid == "" {
		return "", errors.New("OrgID cannot be empty")
	}

	return "z_tyk_analyticz_aggregate_" + orgid, nil
}

func (m *MongoAggregatePump) Init(config interface{}) error {
	m.dbConf = &MongoAggregateConf{}
	err := mapstructure.Decode(config, &m.dbConf)

	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoAggregatePrefix,
		}).Fatal("Failed to decode configuration: ", err)
	}

	overrideErr := envconfig.Process(mongoAggregatePumpPrefix, m.dbConf)
	if overrideErr != nil {
		log.Error("Failed to process environment variables for mongo aggregate pump: ", overrideErr)
	}

	m.connect()

	log.WithFields(logrus.Fields{
		"prefix": mongoAggregatePrefix,
	}).Debug("MongoDB DB CS: ", m.dbConf.MongoURL)

	return nil
}

func (m *MongoAggregatePump) connect() {
	var err error
	var dialInfo *mgo.DialInfo

	dialInfo, err = mongoDialInfo(m.dbConf.MongoURL, m.dbConf.MongoUseSSL, m.dbConf.MongoSSLInsecureSkipVerify)
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Panic("Mongo URL is invalid: ", err)
	}

	m.dbSession, err = mgo.DialWithInfo(dialInfo)
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoAggregatePrefix,
		}).Error("Mongo connection failed:", err)
		time.Sleep(5)
		m.connect()
	}
}

func (m *MongoAggregatePump) ensureIndexes(c *mgo.Collection) error {
	var err error
	ttlIndex := mgo.Index{
		Key:         []string{"expireAt"},
		ExpireAfter: 0,
		Background:  true,
	}

	err = mgohacks.EnsureTTLIndex(c, ttlIndex)
	if err != nil {
		return err
	}

	apiIndex := mgo.Index{
		Key:        []string{"timestamp"},
		Background: true,
	}

	err = c.EnsureIndex(apiIndex)
	if err != nil {
		return err
	}

	orgIndex := mgo.Index{
		Key:        []string{"orgid"},
		Background: true,
	}

	return c.EnsureIndex(orgIndex)
}

func (m *MongoAggregatePump) WriteData(data []interface{}) error {
	if len(data) > 0 {
		log.WithFields(logrus.Fields{
			"prefix": mongoAggregatePrefix,
		}).Info("Writing ", len(data), " records")
	}

	if m.dbSession == nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoAggregatePrefix,
		}).Debug("Connecting to analytics store")
		m.connect()
		m.WriteData(data)
	} else {
		analyticsPerOrg := make(map[string]AnalyticsRecordAggregate)

		for _, v := range data {
			orgID := v.(analytics.AnalyticsRecord).OrgID
			collectionName, collErr := m.GetCollectionName(orgID)
			skip := false
			if collErr != nil {
				log.WithFields(logrus.Fields{
					"prefix": mongoAggregatePrefix,
				}).Info("No OrgID for AnalyticsRecord, skipping")
				skip = true
			}

			thisAggregate, found := analyticsPerOrg[collectionName]

			if !skip {
				thisV := v.(analytics.AnalyticsRecord)

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
				thisCounter := Counter{
					Hits:             1,
					Success:          0,
					ErrorTotal:       0,
					RequestTime:      float64(thisV.RequestTime),
					TotalRequestTime: float64(thisV.RequestTime),
					LastTime:         thisV.TimeStamp,
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
						versionStr := m.doHash(thisV.APIID + ":" + value.(string))
						c := IncrementOrSetUnit(thisAggregate.Versions[versionStr])
						if value.(string) != "" {
							thisAggregate.Versions[versionStr] = c
							thisAggregate.Versions[versionStr].Identifier = value.(string)
							thisAggregate.Versions[versionStr].HumanIdentifier = value.(string)
						}
						break
					case "APIKey":
						c := IncrementOrSetUnit(thisAggregate.APIKeys[value.(string)])
						if value.(string) != "" {
							thisAggregate.APIKeys[value.(string)] = c
							thisAggregate.APIKeys[value.(string)].Identifier = value.(string)
							thisAggregate.APIKeys[value.(string)].HumanIdentifier = thisV.Alias

						}
						break
					case "OauthID":
						c := IncrementOrSetUnit(thisAggregate.OauthIDs[value.(string)])
						if value.(string) != "" {
							thisAggregate.OauthIDs[value.(string)] = c
							thisAggregate.OauthIDs[value.(string)].Identifier = value.(string)
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

				analyticsPerOrg[collectionName] = thisAggregate

			}
		}

		for col_name, filtered_data := range analyticsPerOrg {
			thisSession := m.dbSession.Copy()
			defer thisSession.Close()
			analyticsCollection := thisSession.DB("").C(col_name)
			indexCreateErr := m.ensureIndexes(analyticsCollection)

			if indexCreateErr != nil {
				log.WithFields(logrus.Fields{
					"prefix": mongoAggregatePrefix,
				}).Error(indexCreateErr)
			}

			query := bson.M{
				"orgid":     filtered_data.OrgID,
				"timestamp": filtered_data.TimeStamp,
			}

			updateDoc := filtered_data.AsChange()

			change := mgo.Change{
				Update:    updateDoc,
				ReturnNew: true,
				Upsert:    true,
			}

			doc := AnalyticsRecordAggregate{}
			_, err := analyticsCollection.Find(query).Apply(change, &doc)

			if err != nil {
				log.WithFields(logrus.Fields{
					"prefix": mongoAggregatePrefix,
				}).Error("UPSERT Failure: ", err)
				return m.HandleWriteErr(err)
			}

			// We have the new doc back, lets fix the averages
			avgUpdateDoc := doc.AsTimeUpdate()
			avgChange := mgo.Change{
				Update:    avgUpdateDoc,
				ReturnNew: true,
			}
			withTimeUpdate := AnalyticsRecordAggregate{}
			_, avgErr := analyticsCollection.Find(query).Apply(avgChange, &withTimeUpdate)

			if avgErr != nil {
				log.WithFields(logrus.Fields{
					"prefix": mongoAggregatePrefix,
				}).Error("AvgUpdate Failure: ", avgErr)
				return m.HandleWriteErr(avgErr)
			}

			if m.dbConf.UseMixedCollection {
				thisData := AnalyticsRecordAggregate{}
				err := analyticsCollection.Find(query).One(&thisData)
				if err != nil {
					log.Error("Couldn't find query doc!")
				} else {
					m.doMixedWrite(thisData, query)
				}

			}
		}

	}

	return nil
}

func (m *MongoAggregatePump) doMixedWrite(changeDoc AnalyticsRecordAggregate, query bson.M) {
	thisSession := m.dbSession.Copy()
	defer thisSession.Close()
	analyticsCollection := thisSession.DB("").C(AgggregateMixedCollectionName)
	m.ensureIndexes(analyticsCollection)

	avgChange := mgo.Change{
		Update:    changeDoc,
		ReturnNew: true,
		Upsert:    true,
	}

	final := AnalyticsRecordAggregate{}
	_, avgErr := analyticsCollection.Find(query).Apply(avgChange, &final)

	if avgErr != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoAggregatePrefix,
		}).Error("Mixed coll upsert failure: ", avgErr)
		m.HandleWriteErr(avgErr)
	}
}

func (m *MongoAggregatePump) HandleWriteErr(err error) error {
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoAggregatePrefix,
		}).Error("Problem inserting or updating to mongo collection: ", err)
		if strings.Contains(err.Error(), "Closed explicitly") || strings.Contains(err.Error(), "EOF") {
			log.WithFields(logrus.Fields{
				"prefix": mongoAggregatePrefix,
			}).Warning("--> Detected connection failure, reconnecting")
			m.connect()
		}
	}
	return err
}

// WriteUptimeData will pull the data from the in-memory store and drop it into the specified MongoDB collection
func (m *MongoAggregatePump) WriteUptimeData(data []interface{}) {
	log.WithFields(logrus.Fields{
		"prefix": mongoAggregatePrefix,
	}).Warning("Mongo Aggregate should not be writing uptime data!")
}
