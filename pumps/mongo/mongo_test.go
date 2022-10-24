package mongo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/logger"
	"github.com/TykTechnologies/tyk-pump/pumps/mongo/drivers/mgo"
	"github.com/TykTechnologies/tyk-pump/pumps/mongo/drivers/mgo/mocks"
	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/mock"
)

var (
	dbAddr  = "127.0.0.1:27017"
	colName = "test_collection"
)

func defaultConf() Config {
	conf := Config{
		CollectionName:          colName,
		MaxInsertBatchSizeBytes: 10 * MiB,
		MaxDocumentSizeBytes:    10 * MiB,
	}

	conf.MongoURL = dbAddr
	conf.MongoSSLInsecureSkipVerify = true

	return conf
}

func TestCollectionExists(t *testing.T) {
	tcs := []struct {
		testName        string
		expectedErr     error
		expectedExist   bool
		givenCollection string
		setupCalls      func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager)
	}{
		{
			testName:        "error getting collections",
			expectedErr:     errors.New("error"),
			expectedExist:   false,
			givenCollection: "tyk_analytics",

			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager) {
				collection := &mocks.CollectionManager{}

				database := &mocks.DatabaseManager{}
				database.On("CollectionNames").Return([]string{}, errors.New("error"))

				session := &mocks.SessionManager{}
				session.On("Copy").Return(session)
				session.On("DB", mock.Anything).Return(database)
				session.On("Close")

				return session, database, collection
			},
		},
		{
			testName:        "collection doesn't exist",
			expectedErr:     nil,
			expectedExist:   false,
			givenCollection: "tyk_analytics",
			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager) {
				collection := &mocks.CollectionManager{}

				database := &mocks.DatabaseManager{}
				database.On("CollectionNames").Return([]string{"collection_1", "collection_2"}, nil)

				session := &mocks.SessionManager{}
				session.On("Copy").Return(session)
				session.On("DB", mock.Anything).Return(database)
				session.On("Close")

				return session, database, collection
			},
		},
		{
			testName:        "collection exist",
			expectedErr:     nil,
			expectedExist:   true,
			givenCollection: "tyk_analytics",
			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager) {
				collection := &mocks.CollectionManager{}

				database := &mocks.DatabaseManager{}
				database.On("CollectionNames").Return([]string{"collection_1", "tyk_analytics", "collection_2"}, nil)

				session := &mocks.SessionManager{}
				session.On("Copy").Return(session)
				session.On("DB", mock.Anything).Return(database)
				session.On("Close")

				return session, database, collection
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			conf := defaultConf()
			pmp := &Pump{
				dbConf: &conf,
			}
			pmp.Log = logger.GetLogger().WithField("test", mongoPrefix)

			session, database, collection := tc.setupCalls()

			//we set the mocked session as the wanted sess
			pmp.dbSession = session

			exists, err := pmp.collectionExists(tc.givenCollection)
			assert.Equal(t, tc.expectedErr, err)
			assert.Equal(t, tc.expectedExist, exists)

			//asserting if everything we determined in tc.setupCalls were called
			session.AssertExpectations(t)
			database.AssertExpectations(t)
			collection.AssertExpectations(t)
		})
	}
}

func TestCapCollection(t *testing.T) {
	tcs := []struct {
		testName          string
		expectedResult    bool
		givenCollection   string
		givenMaxSizeBytes int
		givenCapEnabled   bool

		setupCalls func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager)
	}{
		{
			testName:          "capping disabled - should not cap",
			expectedResult:    false,
			givenMaxSizeBytes: 0,
			givenCapEnabled:   false,
			givenCollection:   "tyk_analytics",

			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager) {
				collection := &mocks.CollectionManager{}

				database := &mocks.DatabaseManager{}

				session := &mocks.SessionManager{}

				return session, database, collection
			},
		},
		{
			testName:          "capping enabled - no error but collection already exists",
			expectedResult:    false,
			givenMaxSizeBytes: 0,
			givenCapEnabled:   true,
			givenCollection:   colName,

			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager) {
				collection := &mocks.CollectionManager{}

				database := &mocks.DatabaseManager{}
				database.On("CollectionNames").Return([]string{"collection_1", colName, "collection_2"}, nil)

				session := &mocks.SessionManager{}
				session.On("Copy").Return(session)
				session.On("DB", mock.Anything).Return(database)
				session.On("Close")

				return session, database, collection
			},
		},
		{
			testName:          "capping enabled - error getting collection names",
			expectedResult:    false,
			givenMaxSizeBytes: 0,
			givenCapEnabled:   true,
			givenCollection:   colName,

			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager) {
				collection := &mocks.CollectionManager{}

				database := &mocks.DatabaseManager{}
				database.On("CollectionNames").Return([]string{}, errors.New("error getting collection names"))

				session := &mocks.SessionManager{}
				session.On("DB", "").Return(database)
				session.On("Copy").Return(session)
				session.On("Close")

				return session, database, collection
			},
		},
		{
			testName:          "capping enabled - collection doesn't exist - default maxSizeByteValues",
			expectedResult:    true,
			givenMaxSizeBytes: 0,
			givenCapEnabled:   true,
			givenCollection:   colName,

			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager) {
				collection := &mocks.CollectionManager{}
				arg := &mgo.CollectionInfo{Capped: true, MaxBytes: 5 * GiB}
				collection.On("Create", arg).Return(nil)

				database := &mocks.DatabaseManager{}
				database.On("CollectionNames").Return([]string{"collection_1", "collection_2"}, nil)
				database.On("C", colName).Return(collection)

				session := &mocks.SessionManager{}
				session.On("Copy").Return(session)
				session.On("DB", mock.Anything).Return(database)
				session.On("Close")

				return session, database, collection
			},
		},
		{
			testName:          "capping enabled - collection doesn't exist - custom maxSizeByteValues",
			expectedResult:    true,
			givenMaxSizeBytes: 3000,
			givenCapEnabled:   true,
			givenCollection:   colName,

			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager) {
				collection := &mocks.CollectionManager{}
				arg := &mgo.CollectionInfo{Capped: true, MaxBytes: 3000}
				collection.On("Create", arg).Return(nil)

				database := &mocks.DatabaseManager{}
				database.On("CollectionNames").Return([]string{"collection_1", "collection_2"}, nil)
				database.On("C", colName).Return(collection)

				session := &mocks.SessionManager{}
				session.On("Copy").Return(session)
				session.On("DB", mock.Anything).Return(database)
				session.On("Close")

				return session, database, collection
			},
		},
		{
			testName:          "capping enabled - collection doesn't exist - error capping",
			expectedResult:    false,
			givenMaxSizeBytes: 3000,
			givenCapEnabled:   true,
			givenCollection:   colName,

			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager) {
				collection := &mocks.CollectionManager{}
				arg := &mgo.CollectionInfo{Capped: true, MaxBytes: 3000}
				collection.On("Create", arg).Return(errors.New("error capping"))

				database := &mocks.DatabaseManager{}
				database.On("CollectionNames").Return([]string{"collection_1", "collection_2"}, nil)
				database.On("C", colName).Return(collection)

				session := &mocks.SessionManager{}
				session.On("Copy").Return(session)
				session.On("DB", mock.Anything).Return(database)
				session.On("Close")

				return session, database, collection
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			conf := defaultConf()
			conf.CollectionCapEnable = tc.givenCapEnabled
			conf.CollectionCapMaxSizeBytes = tc.givenMaxSizeBytes
			pmp := &Pump{
				dbConf: &conf,
			}
			pmp.Log = logger.GetLogger().WithField("test", mongoPrefix)
			// setup the expected calls
			session, database, collection := tc.setupCalls()

			// we set the mocked session as the wanted sess
			pmp.dbSession = session

			capped := pmp.capCollection()
			assert.Equal(t, tc.expectedResult, capped)

			//asserting if everything we determined in tc.setupCalls were called
			session.AssertExpectations(t)
			database.AssertExpectations(t)
			collection.AssertExpectations(t)
		})
	}
}

func TestEnsureIndexes(t *testing.T) {
	tcs := []struct {
		testName               string
		expectedErr            error
		givenOmitIndexCreation bool
		givenDbType            MongoType
		setupCalls             func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager)
	}{
		{
			testName:               "omitting index creation - StandardMongo",
			expectedErr:            nil,
			givenOmitIndexCreation: true,
			givenDbType:            StandardMongo,
			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager) {
				collection := &mocks.CollectionManager{}

				database := &mocks.DatabaseManager{}

				session := &mocks.SessionManager{}

				return session, database, collection
			},
		},
		{
			testName:               "not omitting index creation but mongo collection already exists - StandardMongo",
			expectedErr:            nil,
			givenOmitIndexCreation: false,
			givenDbType:            StandardMongo,
			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager) {
				collection := &mocks.CollectionManager{}

				database := &mocks.DatabaseManager{}
				database.On("CollectionNames").Return([]string{"collection_1", colName, "collection_2"}, nil)

				session := &mocks.SessionManager{}
				session.On("Copy").Return(session)
				session.On("DB", mock.Anything).Return(database)
				session.On("Close")

				return session, database, collection
			},
		},
		{
			testName:               "not omitting index creation, error getting collection - StandardMongo",
			expectedErr:            nil,
			givenOmitIndexCreation: false,
			givenDbType:            StandardMongo,
			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager) {
				collection := &mocks.CollectionManager{}
				orgIndex := mgo.Index{
					Key:        []string{"orgid"},
					Background: true,
				}
				collection.On("EnsureIndex", orgIndex).Return(nil)
				apiIndex := mgo.Index{
					Key:        []string{"apiid"},
					Background: true,
				}
				collection.On("EnsureIndex", apiIndex).Return(nil)
				logBrowserIndex := mgo.Index{
					Name:       "logBrowserIndex",
					Key:        []string{"-timestamp", "orgid", "apiid", "apikey", "responsecode"},
					Background: true,
				}
				collection.On("EnsureIndex", logBrowserIndex).Return(nil)

				database := &mocks.DatabaseManager{}
				database.On("CollectionNames").Return([]string{}, errors.New("error getting collection"))
				database.On("C", colName).Return(collection)

				session := &mocks.SessionManager{}
				session.On("Copy").Return(session)
				session.On("DB", mock.Anything).Return(database)
				session.On("Close")

				return session, database, collection
			},
		},
		{
			testName:               "not omitting index creation, error ensuring OrgIndex - StandardMongo",
			expectedErr:            errors.New("error with orgIndex"),
			givenOmitIndexCreation: false,
			givenDbType:            StandardMongo,
			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager) {
				collection := &mocks.CollectionManager{}
				orgIndex := mgo.Index{
					Key:        []string{"orgid"},
					Background: true,
				}
				collection.On("EnsureIndex", orgIndex).Return(errors.New("error with orgIndex"))

				database := &mocks.DatabaseManager{}
				database.On("CollectionNames").Return([]string{}, nil)
				database.On("C", colName).Return(collection)

				session := &mocks.SessionManager{}
				session.On("Copy").Return(session)
				session.On("DB", mock.Anything).Return(database)
				session.On("Close")

				return session, database, collection
			},
		},
		{
			testName:               "not omitting index creation, error ensuring ApiIndex - StandardMongo",
			expectedErr:            errors.New("error setting apiIndex"),
			givenOmitIndexCreation: false,
			givenDbType:            StandardMongo,
			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager) {
				collection := &mocks.CollectionManager{}
				orgIndex := mgo.Index{
					Key:        []string{"orgid"},
					Background: true,
				}
				collection.On("EnsureIndex", orgIndex).Return(nil)
				apiIndex := mgo.Index{
					Key:        []string{"apiid"},
					Background: true,
				}
				collection.On("EnsureIndex", apiIndex).Return(errors.New("error setting apiIndex"))

				database := &mocks.DatabaseManager{}
				database.On("CollectionNames").Return([]string{}, nil)
				database.On("C", colName).Return(collection)

				session := &mocks.SessionManager{}
				session.On("Copy").Return(session)
				session.On("DB", mock.Anything).Return(database)
				session.On("Close")

				return session, database, collection
			},
		},
		{
			testName:               "not omitting index creation, error ensuring logBrowserIndex - StandardMongo",
			expectedErr:            errors.New("error ensuring logBrowserIndex"),
			givenOmitIndexCreation: false,
			givenDbType:            StandardMongo,
			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager) {
				collection := &mocks.CollectionManager{}
				orgIndex := mgo.Index{
					Key:        []string{"orgid"},
					Background: true,
				}
				collection.On("EnsureIndex", orgIndex).Return(nil)
				apiIndex := mgo.Index{
					Key:        []string{"apiid"},
					Background: true,
				}
				collection.On("EnsureIndex", apiIndex).Return(nil)
				logBrowserIndex := mgo.Index{
					Name:       "logBrowserIndex",
					Key:        []string{"-timestamp", "orgid", "apiid", "apikey", "responsecode"},
					Background: true,
				}
				collection.On("EnsureIndex", logBrowserIndex).Return(errors.New("error ensuring logBrowserIndex"))

				database := &mocks.DatabaseManager{}
				database.On("CollectionNames").Return([]string{}, nil)
				database.On("C", colName).Return(collection)

				session := &mocks.SessionManager{}
				session.On("Copy").Return(session)
				session.On("DB", mock.Anything).Return(database)
				session.On("Close")

				return session, database, collection
			},
		},
		{
			testName:               "not omitting index creation, no error setting all indexes - StandardMongo",
			expectedErr:            nil,
			givenOmitIndexCreation: false,
			givenDbType:            StandardMongo,
			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager) {
				collection := &mocks.CollectionManager{}
				orgIndex := mgo.Index{
					Key:        []string{"orgid"},
					Background: true,
				}
				collection.On("EnsureIndex", orgIndex).Return(nil)
				apiIndex := mgo.Index{
					Key:        []string{"apiid"},
					Background: true,
				}
				collection.On("EnsureIndex", apiIndex).Return(nil)
				logBrowserIndex := mgo.Index{
					Name:       "logBrowserIndex",
					Key:        []string{"-timestamp", "orgid", "apiid", "apikey", "responsecode"},
					Background: true,
				}
				collection.On("EnsureIndex", logBrowserIndex).Return(nil)

				database := &mocks.DatabaseManager{}
				database.On("CollectionNames").Return([]string{}, nil)
				database.On("C", colName).Return(collection)

				session := &mocks.SessionManager{}
				session.On("Copy").Return(session)
				session.On("DB", mock.Anything).Return(database)
				session.On("Close")

				return session, database, collection
			},
		},
		{
			testName:               "omitting index creation - DocDB",
			expectedErr:            nil,
			givenOmitIndexCreation: true,
			givenDbType:            AWSDocumentDB,
			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager) {
				collection := &mocks.CollectionManager{}

				database := &mocks.DatabaseManager{}

				session := &mocks.SessionManager{}

				return session, database, collection
			},
		},
		{
			testName:               "not omitting index creation but mongo collection already exists - DocDB",
			expectedErr:            nil,
			givenOmitIndexCreation: false,
			givenDbType:            AWSDocumentDB,
			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager) {
				collection := &mocks.CollectionManager{}
				orgIndex := mgo.Index{
					Key:        []string{"orgid"},
					Background: false,
				}
				collection.On("EnsureIndex", orgIndex).Return(nil)
				apiIndex := mgo.Index{
					Key:        []string{"apiid"},
					Background: false,
				}
				collection.On("EnsureIndex", apiIndex).Return(nil)
				logBrowserIndex := mgo.Index{
					Name:       "logBrowserIndex",
					Key:        []string{"-timestamp", "orgid", "apiid", "apikey", "responsecode"},
					Background: false,
				}
				collection.On("EnsureIndex", logBrowserIndex).Return(nil)

				database := &mocks.DatabaseManager{}
				//we are not calling CollectionExist here since it only works for StandardMongo
				database.On("C", colName).Return(collection)

				session := &mocks.SessionManager{}
				session.On("Copy").Return(session)
				session.On("DB", mock.Anything).Return(database)
				session.On("Close")

				return session, database, collection
			},
		},
		{
			testName:               "omitting index creation  - DocDB",
			expectedErr:            nil,
			givenOmitIndexCreation: true,
			givenDbType:            AWSDocumentDB,
			setupCalls: func() (*mocks.SessionManager, *mocks.DatabaseManager, *mocks.CollectionManager) {
				collection := &mocks.CollectionManager{}

				database := &mocks.DatabaseManager{}

				session := &mocks.SessionManager{}

				return session, database, collection
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			conf := defaultConf()
			conf.OmitIndexCreation = tc.givenOmitIndexCreation
			conf.MongoDBType = tc.givenDbType
			pmp := &Pump{
				dbConf: &conf,
			}
			pmp.Log = logger.GetLogger().WithField("test", mongoPrefix)

			session, database, collection := tc.setupCalls()

			//we set the mocked session as the wanted sess
			pmp.dbSession = session

			err := pmp.ensureIndexes()
			assert.Equal(t, tc.expectedErr, err)

			//asserting if everything we determined in tc.setupCalls were called
			session.AssertExpectations(t)
			database.AssertExpectations(t)
			collection.AssertExpectations(t)
		})
	}
}

func TestAccumulateSet(t *testing.T) {
	conf := defaultConf()
	conf.MaxInsertBatchSizeBytes = 5120

	numRecords := 100
	// assumed from sizeBytes in AccumulateSet
	const dataSize = 1024
	totalData := dataSize * numRecords

	mPump := &Pump{}
	mPump.dbConf = &conf
	mPump.Log = logger.GetLogger().WithField("prefix", mongoPrefix)

	record := analytics.AnalyticsRecord{}
	data := make([]interface{}, 0)

	for i := 0; i < numRecords; i++ {
		data = append(data, record)
	}

	set := mPump.AccumulateSet(data)

	if len(set) != totalData/conf.MaxInsertBatchSizeBytes {
		t.Errorf("expected accumulator chunks to equal %d, got %d", totalData/conf.MaxInsertBatchSizeBytes, len(set))
	}
}

func TestWriteData(t *testing.T) {
	timeNow := time.Now()
	keys := make([]interface{}, 2)
	keys[0] = analytics.AnalyticsRecord{APIID: "api1", OrgID: "123", TimeStamp: timeNow, APIKey: "apikey1"}
	keys[1] = analytics.AnalyticsRecord{APIID: "api1", OrgID: "123", TimeStamp: timeNow, APIKey: "apikey1"}

	//check what functions from Collection are going to be called
	collection := &mocks.CollectionManager{}
	collection.On("Insert", keys...).Return(nil)

	//check what functions from Database are going to be called
	database := &mocks.DatabaseManager{}
	database.On("C", colName).Return(collection)

	//check what functions from Session are going to be called
	session := &mocks.SessionManager{}
	session.On("DB", "").Return(database)
	session.On("Copy").Return(session)
	session.On("Close")

	conf := defaultConf()
	conf.MaxInsertBatchSizeBytes = 5120
	pmp := &Pump{
		dbConf: &conf,
	}
	pmp.Log = logger.GetLogger().WithField("test", mongoPrefix)
	//we set the mocked session as the wanted sess
	pmp.dbSession = session

	//Execute pump writing
	err := pmp.WriteData(context.Background(), keys)
	assert.Nil(t, err)

	//assert all the expected mocked calls
	session.AssertExpectations(t)
	database.AssertExpectations(t)
	collection.AssertExpectations(t)
}
