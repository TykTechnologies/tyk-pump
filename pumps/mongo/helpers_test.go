package mongo

import (
	"errors"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/pumps/internal/mgo"
	"github.com/TykTechnologies/tyk-pump/pumps/internal/mgo/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetMongoType(t *testing.T) {
	tcs := []struct {
		testName string

		expectedType MongoType

		setupCalls func() *mocks.SessionManager
	}{
		{
			testName: "Error calling mongo cmd",
			setupCalls: func() *mocks.SessionManager {
				session := &mocks.SessionManager{}
				cmd := struct {
					Code int `bson:"code"`
				}{}
				session.On("Run", "features", &cmd).Return(errors.New("error calling cmd"))

				return session
			},
			expectedType: StandardMongo,
		},
		{
			testName: "Standard Mongo",
			setupCalls: func() *mocks.SessionManager {
				session := &mocks.SessionManager{}
				cmd := struct {
					Code int `bson:"code"`
				}{}
				session.On("Run", "features", &cmd).Return(nil)

				return session
			},
			expectedType: StandardMongo,
		},
		{
			testName: "DocDB Mongo",
			setupCalls: func() *mocks.SessionManager {
				session := &mocks.SessionManager{}

				cmd := struct {
					Code int `bson:"code"`
				}{}
				session.On("Run", "features", &cmd).Return(nil).Run(func(args mock.Arguments) {
					//we get the second argument to modify the response
					resp := args.Get(1).(*struct {
						Code int `bson:"code"`
					})
					resp.Code = 303 // AWSError
				})

				return session
			},
			expectedType: AWSDocumentDB,
		},
		{
			testName: "CosmosDB Mongo",
			setupCalls: func() *mocks.SessionManager {
				session := &mocks.SessionManager{}

				cmd := struct {
					Code int `bson:"code"`
				}{}
				session.On("Run", "features", &cmd).Return(nil).Run(func(args mock.Arguments) {
					//we get the second argument to modify the response
					resp := args.Get(1).(*struct {
						Code int `bson:"code"`
					})
					resp.Code = 115 // CosmosDBError
				})

				return session
			},
			expectedType: CosmosDB,
		},
		{
			testName: "Random type - Standard Mongo",
			setupCalls: func() *mocks.SessionManager {
				session := &mocks.SessionManager{}

				cmd := struct {
					Code int `bson:"code"`
				}{}
				session.On("Run", "features", &cmd).Return(nil).Run(func(args mock.Arguments) {
					//we get the second argument to modify the response
					resp := args.Get(1).(*struct {
						Code int `bson:"code"`
					})
					resp.Code = 5 // random error code
				})

				return session
			},
			expectedType: StandardMongo,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			session := tc.setupCalls()

			actualType := GetMongoType(session)
			assert.Equal(t, tc.expectedType, actualType)

			session.AssertExpectations(t)
		})
	}
}

func TestNewSession(t *testing.T) {
	tcs := []struct {
		testName string

		givenConf    BaseConfig
		givenTimeout int
		expectedErr  error
		setupCalls   func() (*mocks.SessionManager, *mocks.Dialer)
	}{
		{
			testName:  "no error - defaults",
			givenConf: BaseConfig{MongoURL: dbAddr},
			setupCalls: func() (*mocks.SessionManager, *mocks.Dialer) {
				session := &mocks.SessionManager{}

				dialer := &mocks.Dialer{}
				dialInfo := &mgo.DialInfo{
					Addrs: []string{dbAddr},
				}
				dialer.On("DialWithInfo", dialInfo).Return(session, nil)

				return session, dialer
			},
			expectedErr: nil,
		},
		{
			testName:  " error connecting",
			givenConf: BaseConfig{MongoURL: dbAddr},
			setupCalls: func() (*mocks.SessionManager, *mocks.Dialer) {
				session := &mocks.SessionManager{}

				dialer := &mocks.Dialer{}
				dialInfo := &mgo.DialInfo{
					Addrs: []string{dbAddr},
				}
				dialer.On("DialWithInfo", dialInfo).Return(session, errors.New("error connecting"))

				return session, dialer
			},
			expectedErr: errors.New("error connecting"),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			sess, dialer := tc.setupCalls()

			actualSession, actualErr := NewSession(dialer, tc.givenConf, tc.givenTimeout)

			assert.Equal(t, tc.expectedErr, actualErr)
			assert.Equal(t, sess, actualSession)
		})
	}
}

func TestBuildDialInfo(t *testing.T) {
	tcs := []struct {
		testName string

		givenConf    BaseConfig
		givenTimeout int
		expectedErr  error
		setupCalls   func() *mocks.Dialer
		assertions   func(t *testing.T, info *mgo.DialInfo)
	}{
		{
			testName:    "no error - defaults",
			givenConf:   BaseConfig{MongoURL: dbAddr},
			expectedErr: nil,
			assertions: func(t *testing.T, actualDialInfo *mgo.DialInfo) {
				assert.Equal(t, []string{dbAddr}, actualDialInfo.Addrs)

				assert.Equal(t, false, actualDialInfo.InsecureSkipVerify)
				assert.Equal(t, false, actualDialInfo.SSLAllowInvalidHostnames)
				assert.Equal(t, "", actualDialInfo.SSLCAFile)
				assert.Equal(t, "", actualDialInfo.SSLPEMKeyFile)
				assert.Equal(t, time.Duration(0), actualDialInfo.Timeout)
			},
		},
		{
			testName: "no errors - config values",
			givenConf: BaseConfig{
				MongoURL:                      dbAddr,
				MongoSSLInsecureSkipVerify:    true,
				MongoSSLPEMKeyfile:            "MongoSSLPEMKeyfile",
				MongoUseSSL:                   true,
				MongoSSLAllowInvalidHostnames: true,
				MongoSSLCAFile:                "MongoSSLCAFile",
			},
			givenTimeout: 10,
			expectedErr:  nil,
			assertions: func(t *testing.T, actualDialInfo *mgo.DialInfo) {
				assert.Equal(t, []string{dbAddr}, actualDialInfo.Addrs)

				assert.Equal(t, true, actualDialInfo.InsecureSkipVerify)
				assert.Equal(t, true, actualDialInfo.SSLAllowInvalidHostnames)
				assert.Equal(t, "MongoSSLCAFile", actualDialInfo.SSLCAFile)
				assert.Equal(t, "MongoSSLPEMKeyfile", actualDialInfo.SSLPEMKeyFile)

				assert.Equal(t, time.Duration(10*time.Second), actualDialInfo.Timeout)
			},
		},
		{
			testName: "no errors - config values but no use_ssl",
			givenConf: BaseConfig{
				MongoURL:                      dbAddr,
				MongoSSLInsecureSkipVerify:    true,
				MongoSSLPEMKeyfile:            "MongoSSLPEMKeyfile",
				MongoUseSSL:                   false,
				MongoSSLAllowInvalidHostnames: true,
				MongoSSLCAFile:                "MongoSSLCAFile",
			},
			expectedErr: nil,
			assertions: func(t *testing.T, actualDialInfo *mgo.DialInfo) {
				assert.Equal(t, []string{dbAddr}, actualDialInfo.Addrs)

				assert.Equal(t, false, actualDialInfo.InsecureSkipVerify)
				assert.Equal(t, false, actualDialInfo.SSLAllowInvalidHostnames)
				assert.Equal(t, "", actualDialInfo.SSLCAFile)
				assert.Equal(t, "", actualDialInfo.SSLPEMKeyFile)
				assert.Equal(t, time.Duration(0), actualDialInfo.Timeout)
			},
		},
		{
			testName:    "error - malformed connection opts",
			givenConf:   BaseConfig{MongoURL: dbAddr + "?malformed=true"},
			expectedErr: errors.New("unsupported connection URL option: malformed=true"),
			assertions: func(t *testing.T, actualDialInfo *mgo.DialInfo) {
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {

			dialinfo, err := buildDialInfo(tc.givenConf, tc.givenTimeout)
			assert.Equal(t, tc.expectedErr, err)
			tc.assertions(t, dialinfo)

		})
	}
}
