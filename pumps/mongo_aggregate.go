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
	"gopkg.in/vmihailenco/msgpack.v2"
	"strconv"
	"strings"
	"time"
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

type MongoAggregatePump struct {
	dbSession *mgo.Session
	dbConf    *MongoAggregateConf
}

var mongoAggregatePrefix string = "mongo-pump-aggregate"

type MongoAggregateConf struct {
	MongoURL string `mapstructure:"mongo_url"`
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

			existingAgg := AnalyticsRecordAggregate{}
			fErr := analyticsCollection.Find(query).One(&existingAgg)
			if fErr != nil {
				insertErr := analyticsCollection.Insert(filtered_data)
				if insertErr != nil {
					return m.HandleWriteErr(insertErr)
				}
				return nil
			}

			// Not a new record, lets increment fields and update
			updatedAggregate := m.UpdateExistingMgoRecord(filtered_data, existingAgg)
			updateErr := analyticsCollection.Update(query, updatedAggregate)
			if updateErr != nil {
				return m.HandleWriteErr(updateErr)
			}

		}

	}

	return nil
}

func (m *MongoAggregatePump) UpdateExistingMgoRecord(newAgg, existingAgg AnalyticsRecordAggregate) AnalyticsRecordAggregate {
	for k, v := range newAgg.APIKeys {
		if existingAgg.APIKeys[k] == nil {
			existingAgg.APIKeys[k] = v
		} else {
			existingAgg.APIKeys[k].Hits += v.Hits
			existingAgg.APIKeys[k].Success += v.Success
			existingAgg.APIKeys[k].ErrorTotal += v.ErrorTotal
			existingAgg.APIKeys[k].TotalRequestTime += v.TotalRequestTime
			existingAgg.APIKeys[k].RequestTime = existingAgg.APIKeys[k].TotalRequestTime / float64(existingAgg.APIKeys[k].Hits)
		}
	}

	for k, v := range newAgg.Errors {
		if existingAgg.Errors[k] == nil {
			existingAgg.Errors[k] = v
		} else {
			existingAgg.Errors[k].Hits += v.Hits
			existingAgg.Errors[k].Success += v.Success
			existingAgg.Errors[k].ErrorTotal += v.ErrorTotal
			existingAgg.Errors[k].TotalRequestTime += v.TotalRequestTime
			existingAgg.Errors[k].RequestTime = existingAgg.Errors[k].TotalRequestTime / float64(existingAgg.Errors[k].Hits)
		}
	}

	for k, v := range newAgg.Versions {
		if existingAgg.Versions[k] == nil {
			existingAgg.Versions[k] = v
		} else {
			existingAgg.Versions[k].Hits += v.Hits
			existingAgg.Versions[k].Success += v.Success
			existingAgg.Versions[k].ErrorTotal += v.ErrorTotal
			existingAgg.Versions[k].TotalRequestTime += v.TotalRequestTime
			existingAgg.Versions[k].RequestTime = existingAgg.Versions[k].TotalRequestTime / float64(existingAgg.Versions[k].Hits)
		}
	}

	for k, v := range newAgg.APIID {
		if existingAgg.APIID[k] == nil {
			existingAgg.APIID[k] = v
		} else {
			existingAgg.APIID[k].Hits += v.Hits
			existingAgg.APIID[k].Success += v.Success
			existingAgg.APIID[k].ErrorTotal += v.ErrorTotal
			existingAgg.APIID[k].TotalRequestTime += v.TotalRequestTime
			existingAgg.APIID[k].RequestTime = existingAgg.APIID[k].TotalRequestTime / float64(existingAgg.APIID[k].Hits)
		}
	}

	for k, v := range newAgg.OauthIDs {
		if existingAgg.OauthIDs[k] == nil {
			existingAgg.OauthIDs[k] = v
		} else {
			existingAgg.OauthIDs[k].Hits += v.Hits
			existingAgg.OauthIDs[k].Success += v.Success
			existingAgg.OauthIDs[k].ErrorTotal += v.ErrorTotal
			existingAgg.OauthIDs[k].TotalRequestTime += v.TotalRequestTime
			existingAgg.OauthIDs[k].RequestTime = existingAgg.OauthIDs[k].TotalRequestTime / float64(existingAgg.OauthIDs[k].Hits)
		}
	}

	for k, v := range newAgg.Geo {
		if existingAgg.Geo[k] == nil {
			existingAgg.Geo[k] = v
		} else {
			existingAgg.Geo[k].Hits += v.Hits
			existingAgg.Geo[k].Success += v.Success
			existingAgg.Geo[k].ErrorTotal += v.ErrorTotal
			existingAgg.Geo[k].TotalRequestTime += v.TotalRequestTime
			existingAgg.Geo[k].RequestTime = existingAgg.Geo[k].TotalRequestTime / float64(existingAgg.Geo[k].Hits)
		}
	}

	for k, v := range newAgg.Tags {
		if existingAgg.Tags[k] == nil {
			existingAgg.Tags[k] = v
		} else {
			existingAgg.Tags[k].Hits += v.Hits
			existingAgg.Tags[k].Success += v.Success
			existingAgg.Tags[k].ErrorTotal += v.ErrorTotal
			existingAgg.Tags[k].TotalRequestTime += v.TotalRequestTime
			existingAgg.Tags[k].RequestTime = existingAgg.Tags[k].TotalRequestTime / float64(existingAgg.Tags[k].Hits)
		}
	}

	existingAgg.Total.Hits += newAgg.Total.Hits
	existingAgg.Total.Success += newAgg.Total.Success
	existingAgg.Total.ErrorTotal += newAgg.Total.ErrorTotal
	existingAgg.Total.TotalRequestTime += newAgg.Total.TotalRequestTime
	existingAgg.Total.RequestTime = existingAgg.Total.TotalRequestTime / float64(existingAgg.Total.Hits)

	return existingAgg
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
	if m.dbSession == nil {
		log.Debug("Connecting to mongoDB store")
		m.connect()
		m.WriteUptimeData(data)
	} else {
		log.Info("MONGO Aggregate Should not be writing uptime data!")
		collectionName := "tyk_uptime_analytics"
		analyticsCollection := m.dbSession.DB("").C(collectionName)

		log.WithFields(logrus.Fields{
			"prefix": mongoAggregatePrefix,
		}).Debug("Uptime Data: ", len(data))

		if len(data) > 0 {
			keys := make([]interface{}, len(data), len(data))

			for i, v := range data {
				decoded := analytics.UptimeReportData{}
				err := msgpack.Unmarshal(v.([]byte), &decoded)
				log.WithFields(logrus.Fields{
					"prefix": mongoAggregatePrefix,
				}).Debug("Decoded Record: ", decoded)
				if err != nil {
					log.WithFields(logrus.Fields{
						"prefix": mongoAggregatePrefix,
					}).Error("Couldn't unmarshal analytics data:", err)

				} else {
					keys[i] = interface{}(decoded)
				}
			}

			err := analyticsCollection.Insert(keys...)
			log.WithFields(logrus.Fields{
				"prefix": mongoAggregatePrefix,
			}).Debug("Wrote data to ", collectionName)

			if err != nil {
				log.WithFields(logrus.Fields{
					"prefix": mongoAggregatePrefix,
				}).Error("Problem inserting to mongo collection: ", err)
				if strings.Contains(err.Error(), "Closed explicitly") {
					log.WithFields(logrus.Fields{
						"prefix": mongoAggregatePrefix,
					}).Warning("--> Detected connection failure, reconnecting")
					m.connect()
				}
			}
		}
	}

}
