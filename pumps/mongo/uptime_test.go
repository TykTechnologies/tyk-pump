package mongo

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/logger"
	"github.com/TykTechnologies/tyk-pump/pumps/internal/mgo/mocks"
	"gopkg.in/vmihailenco/msgpack.v2"
)

func TestWriteUptimeData(t *testing.T) {
	tcs := []struct {
		testName   string
		pumpConfig Config
		setupCalls func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager, []interface{})
	}{
		{
			testName:   "success writing",
			pumpConfig: defaultConf(),
			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager, []interface{}) {
				now := time.Now()
				data := []analytics.UptimeReportData{
					{OrgID: "orgID", ResponseCode: http.StatusOK, URL: "uptimeURL1", TimeStamp: now},
					{OrgID: "orgID", ResponseCode: http.StatusOK, URL: "uptimeURL1", TimeStamp: now},
				}

				redisKeys, keys := marshalledUptimeData(data)

				//check what functions from Collection are going to be called
				collection := &mocks.CollectionManager{}
				collection.On("Insert", keys...).Return(nil)

				//check what functions from Database are going to be called
				database := &mocks.DatabaseManager{}
				database.On("C", "tyk_uptime_analytics").Return(collection)

				//check what functions from Session are going to be called
				session := &mocks.SessionManager{}
				session.On("DB", "").Return(database)
				session.On("Copy").Return(session)
				session.On("Close").Maybe()
				return session, database, collection, redisKeys
			},
		},
		{
			testName:   "invalid record type",
			pumpConfig: defaultConf(),
			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager, []interface{}) {
				now := time.Now()
				keys := make([]interface{}, 2)
				keys[0] = analytics.AnalyticsRecord{APIID: "api1", OrgID: "123", TimeStamp: now, APIKey: "apikey1"}
				keys[1] = analytics.AnalyticsRecord{APIID: "api1", OrgID: "123", TimeStamp: now, APIKey: "apikey1"}

				//check what functions from Collection are going to be called
				collection := &mocks.CollectionManager{}

				//check what functions from Database are going to be called
				database := &mocks.DatabaseManager{}
				database.On("C", "tyk_uptime_analytics").Return(collection)

				//check what functions from Session are going to be called
				session := &mocks.SessionManager{}
				session.On("DB", "").Return(database)
				session.On("Copy").Return(session)
				session.On("Close").Maybe()
				return session, database, collection, keys
			},
		},
		{
			testName:   "error unmarshalling ",
			pumpConfig: defaultConf(),
			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager, []interface{}) {
				now := time.Now()
				keys := make([]interface{}, 2)
				keys[0] = analytics.AnalyticsRecord{APIID: "api1", OrgID: "123", TimeStamp: now, APIKey: "apikey1"}
				keys[1] = analytics.AnalyticsRecord{APIID: "api1", OrgID: "123", TimeStamp: now, APIKey: "apikey1"}

				// trying with different encoder
				redisKeys := make([]interface{}, len(keys))
				for i, report := range redisKeys {
					encoded, _ := json.Marshal(report)
					redisKeys[i] = string(encoded)
				}

				//check what functions from Collection are going to be called
				collection := &mocks.CollectionManager{}

				//check what functions from Database are going to be called
				database := &mocks.DatabaseManager{}
				database.On("C", "tyk_uptime_analytics").Return(collection)

				//check what functions from Session are going to be called
				session := &mocks.SessionManager{}
				session.On("DB", "").Return(database)
				session.On("Copy").Return(session)
				session.On("Close").Maybe()
				return session, database, collection, redisKeys
			},
		},
		{
			testName:   "writing mongo pump uptime data - error insert",
			pumpConfig: defaultConf(),
			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager, []interface{}) {
				now := time.Now()
				data := []analytics.UptimeReportData{
					{OrgID: "orgID", ResponseCode: http.StatusOK, URL: "uptimeURL1", TimeStamp: now},
					{OrgID: "orgID", ResponseCode: http.StatusOK, URL: "uptimeURL1", TimeStamp: now},
				}

				redisKeys, keys := marshalledUptimeData(data)

				//check what functions from Collection are going to be called
				collection := &mocks.CollectionManager{}
				collection.On("Insert", keys...).Return(errors.New("error inserting"))

				//check what functions from Database are going to be called
				database := &mocks.DatabaseManager{}
				database.On("C", "tyk_uptime_analytics").Return(collection)

				//check what functions from Session are going to be called
				session := &mocks.SessionManager{}
				session.On("DB", "").Return(database)
				session.On("Copy").Return(session)
				session.On("Close").Maybe()
				return session, database, collection, redisKeys
			},
		},

		{
			testName:   "error insert Closed explicitly",
			pumpConfig: defaultConf(),
			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager, []interface{}) {
				now := time.Now()
				data := []analytics.UptimeReportData{
					{OrgID: "orgID", ResponseCode: http.StatusOK, URL: "uptimeURL1", TimeStamp: now},
					{OrgID: "orgID", ResponseCode: http.StatusOK, URL: "uptimeURL1", TimeStamp: now},
				}

				redisKeys, keys := marshalledUptimeData(data)

				//check what functions from Collection are going to be called
				collection := &mocks.CollectionManager{}
				collection.On("Insert", keys...).Return(errors.New("error inserting:Closed explicitly"))

				//check what functions from Database are going to be called
				database := &mocks.DatabaseManager{}
				database.On("C", "tyk_uptime_analytics").Return(collection)

				//check what functions from Session are going to be called
				session := &mocks.SessionManager{}
				session.On("DB", "").Return(database)
				session.On("Copy").Return(session)
				session.On("Close").Maybe()
				return session, database, collection, redisKeys
			},
		},
		{
			testName:   "error no data",
			pumpConfig: defaultConf(),
			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager, []interface{}) {

				//check what functions from Collection are going to be called
				collection := &mocks.CollectionManager{}

				//check what functions from Database are going to be called
				database := &mocks.DatabaseManager{}

				//check what functions from Session are going to be called
				session := &mocks.SessionManager{}
				return session, database, collection, []interface{}{}
			},
		},
	}

	logger := logger.GetLogger().WithField("test", mongoPrefix)
	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			conf := tc.pumpConfig
			pmp := &Pump{
				dbConf: &conf,
			}
			pmp.Log = logger

			session, database, collection, records := tc.setupCalls()

			//we set the mocked session as the wanted sess
			pmp.dbSession = session

			pmp.WriteUptimeData(records)

			//asserting if everything we determined in tc.setupCalls were called
			session.AssertExpectations(t)
			database.AssertExpectations(t)
			collection.AssertExpectations(t)
		})
	}
}

func marshalledUptimeData(data []analytics.UptimeReportData) (redisKeys []interface{}, dataInterfaces []interface{}) {
	redisKeys = make([]interface{}, len(data))
	for i, report := range data {
		encoded, _ := msgpack.Marshal(report)
		redisKeys[i] = string(encoded)
	}

	dataInterfaces = make([]interface{}, len(data))
	for i, v := range redisKeys {
		decoded := analytics.UptimeReportData{}
		if err := msgpack.Unmarshal([]byte(v.(string)), &decoded); err != nil {
			continue
		}
		dataInterfaces[i] = interface{}(decoded)
	}

	return
}
