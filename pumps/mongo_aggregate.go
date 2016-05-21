package pumps

import (
	b64 "encoding/base64"
	"errors"
	"github.com/Sirupsen/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/fatih/structs"
	"github.com/lonelycode/mgohacks"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"strconv"
	"strings"
	"time"
)

const (
	AgggregateMixedCollectionName string = "tyk_analytics_aggregates"
)

type Counter struct {
	Hits             int     `json:"hits"`
	Success          int     `json:"success"`
	ErrorTotal       int     `json:"error"`
	RequestTime      float64 `json:"request_time"`
	TotalRequestTime float64 `json:"total_request_time"`
	Identifier       string  `json:"identifier"`
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
	Total    Counter

	ExpireAt time.Time `bson:"expireAt" json:"expireAt"`
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

	newUpdate = f.generateBSONFromProperty("", "total", &f.Total, newUpdate)

	asTime := f.TimeStamp
	newTime := time.Date(asTime.Year(), asTime.Month(), asTime.Day(), asTime.Hour(), 0, 0, 0, asTime.Location())
	newUpdate["$set"].(bson.M)["timestamp"] = newTime
	newUpdate["$set"].(bson.M)["expireAt"] = f.ExpireAt
	newUpdate["$set"].(bson.M)["timeid.year"] = newTime.Year()
	newUpdate["$set"].(bson.M)["timeid.month"] = newTime.Month()
	newUpdate["$set"].(bson.M)["timeid.day"] = newTime.Day()
	newUpdate["$set"].(bson.M)["timeid.hour"] = newTime.Hour()

	return newUpdate
}

func (f *AnalyticsRecordAggregate) AsTimeUpdate() bson.M {
	newUpdate := bson.M{
		"$set": bson.M{},
	}

	for thisUnit, incVal := range f.APIID {
		newTime := incVal.TotalRequestTime / float64(incVal.Hits)
		newUpdate = f.generateSetterForTime("apiid", thisUnit, newTime, newUpdate)
	}

	for thisUnit, incVal := range f.Errors {
		newTime := incVal.TotalRequestTime / float64(incVal.Hits)
		newUpdate = f.generateSetterForTime("errors", thisUnit, newTime, newUpdate)
	}

	for thisUnit, incVal := range f.Versions {
		newTime := incVal.TotalRequestTime / float64(incVal.Hits)
		newUpdate = f.generateSetterForTime("versions", thisUnit, newTime, newUpdate)
	}

	for thisUnit, incVal := range f.APIKeys {
		newTime := incVal.TotalRequestTime / float64(incVal.Hits)
		newUpdate = f.generateSetterForTime("apikeys", thisUnit, newTime, newUpdate)
	}

	for thisUnit, incVal := range f.OauthIDs {
		newTime := incVal.TotalRequestTime / float64(incVal.Hits)
		newUpdate = f.generateSetterForTime("oauthids", thisUnit, newTime, newUpdate)
	}

	for thisUnit, incVal := range f.Geo {
		newTime := incVal.TotalRequestTime / float64(incVal.Hits)
		newUpdate = f.generateSetterForTime("geo", thisUnit, newTime, newUpdate)
	}

	for thisUnit, incVal := range f.Tags {
		newTime := incVal.TotalRequestTime / float64(incVal.Hits)
		newUpdate = f.generateSetterForTime("tags", thisUnit, newTime, newUpdate)
	}

	newTime := f.Total.TotalRequestTime / float64(f.Total.Hits)
	newUpdate = f.generateSetterForTime("", "total", newTime, newUpdate)

	return newUpdate
}

type MongoAggregatePump struct {
	dbSession *mgo.Session
	dbConf    *MongoAggregateConf
}

var mongoAggregatePrefix string = "mongo-pump-aggregate"

type MongoAggregateConf struct {
	MongoURL           string `mapstructure:"mongo_url"`
	UseMixedCollection bool   `mapstructure:"use_mixed_collection"`
}

func (m *MongoAggregatePump) New() Pump {
	newPump := MongoAggregatePump{}
	return &newPump
}

func (m *MongoAggregatePump) doHash(in string) string {
	return b64.StdEncoding.EncodeToString([]byte(in))
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

	m.connect()

	log.WithFields(logrus.Fields{
		"prefix": mongoAggregatePrefix,
	}).Debug("MongoDB DB CS: ", m.dbConf.MongoURL)

	return nil
}

func (m *MongoAggregatePump) connect() {
	var err error
	m.dbSession, err = mgo.Dial(m.dbConf.MongoURL)
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

	err = c.EnsureIndex(orgIndex)
	if err != nil {
		return err
	}

	return nil
}

func (m *MongoAggregatePump) WriteData(data []interface{}) error {
	log.WithFields(logrus.Fields{
		"prefix": mongoAggregatePrefix,
	}).Info("Writing ", len(data), " records")

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
				}

				// Create the counter for this record
				thisCounter := Counter{
					Hits:             1,
					Success:          0,
					ErrorTotal:       0,
					RequestTime:      float64(thisV.RequestTime),
					TotalRequestTime: float64(thisV.RequestTime),
				}

				thisAggregate.Total.Hits += 1
				thisAggregate.Total.TotalRequestTime += float64(thisV.RequestTime)

				// We need an initial value
				thisAggregate.Total.RequestTime = thisAggregate.Total.TotalRequestTime / float64(thisAggregate.Total.Hits)

				if thisV.ResponseCode > 400 {
					thisCounter.ErrorTotal = 1
					thisAggregate.Total.ErrorTotal += 1
				}

				if (thisV.ResponseCode < 300) && (thisV.ResponseCode >= 200) {
					thisCounter.Success = 1
					thisAggregate.Total.Success += 1
				}

				// Convert to a map (for easy iteration)
				vAsMap := structs.Map(thisV)
				for key, value := range vAsMap {

					// Mini function to handle incrementing a specific counter in our object
					IncrementOrSetUnit := func(c *Counter) *Counter {
						if c == nil {
							var newCounter Counter
							newCounter = thisCounter
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
							thisAggregate.APIID[value.(string)].Identifier = thisV.APIName
						}
						break
					case "ResponseCode":
						errAsStr := strconv.Itoa(value.(int))
						c := IncrementOrSetUnit(thisAggregate.Errors[errAsStr])
						if errAsStr != "" {
							thisAggregate.Errors[errAsStr] = c
							thisAggregate.Errors[errAsStr].Identifier = errAsStr
						}
						break
					case "APIVersion":
						versionStr := m.doHash(thisV.APIID + ":" + value.(string))
						c := IncrementOrSetUnit(thisAggregate.Versions[versionStr])
						if value.(string) != "" {
							thisAggregate.Versions[versionStr] = c
							thisAggregate.Versions[versionStr].Identifier = value.(string)
						}
						break
					case "APIKey":
						c := IncrementOrSetUnit(thisAggregate.APIKeys[value.(string)])
						if value.(string) != "" {
							thisAggregate.APIKeys[value.(string)] = c
							thisAggregate.APIKeys[value.(string)].Identifier = value.(string)
							if thisV.Alias != "" {
								thisAggregate.APIKeys[value.(string)].Identifier += " (" + thisV.Alias + ")"
							}
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
						}
						break
					case "Tags":
						for _, thisTag := range thisV.Tags {
							c := IncrementOrSetUnit(thisAggregate.Tags[thisTag])
							thisAggregate.Tags[thisTag] = c
							thisAggregate.Tags[thisTag].Identifier = thisTag
						}
						break
					}

					analyticsPerOrg[collectionName] = thisAggregate

				}
			}
		}

		for col_name, filtered_data := range analyticsPerOrg {
			analyticsCollection := m.dbSession.DB("").C(col_name)
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
				m.doMixedWrite(withTimeUpdate, query)
			}
		}

	}

	return nil
}

func (m *MongoAggregatePump) doMixedWrite(changeDoc AnalyticsRecordAggregate, query bson.M) {
	analyticsCollection := m.dbSession.DB("").C(AgggregateMixedCollectionName)
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
		if strings.Contains(strings.ToLower(err.Error()), "closed explicitly") {
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
