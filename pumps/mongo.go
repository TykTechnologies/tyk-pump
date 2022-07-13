package pumps

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/pumps/common"
	"github.com/kelseyhightower/envconfig"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/mgo.v2"
	"gopkg.in/vmihailenco/msgpack.v2"
)

const (
	_   = iota // ignore zero iota
	KiB = 1 << (10 * iota)
	MiB
	GiB
	TiB
)

type MongoPump struct {
	IsUptime  bool
	dbSession *mgo.Session
	dbConf    *MongoConf
	common.Pump
}

var mongoPrefix = "mongo-pump"
var mongoPumpPrefix = "PMP_MONGO"
var mongoDefaultEnv = PUMPS_ENV_PREFIX + "_MONGO" + PUMPS_ENV_META_PREFIX

type MongoType int

const (
	StandardMongo MongoType = iota
	AWSDocumentDB
)

type BaseMongoConf struct {
	EnvPrefix string `mapstructure:"meta_env_prefix"`
	// The full URL to your MongoDB instance, this can be a clustered instance if necessary and
	// should include the database and username / password data.
	MongoURL string `json:"mongo_url" mapstructure:"mongo_url"`
	// Set to true to enable Mongo SSL connection.
	MongoUseSSL bool `json:"mongo_use_ssl" mapstructure:"mongo_use_ssl"`
	// Allows the use of self-signed certificates when connecting to an encrypted MongoDB database.
	MongoSSLInsecureSkipVerify bool `json:"mongo_ssl_insecure_skip_verify" mapstructure:"mongo_ssl_insecure_skip_verify"`
	// Ignore hostname check when it differs from the original (for example with SSH tunneling).
	// The rest of the TLS verification will still be performed.
	MongoSSLAllowInvalidHostnames bool `json:"mongo_ssl_allow_invalid_hostnames" mapstructure:"mongo_ssl_allow_invalid_hostnames"`
	// Path to the PEM file with trusted root certificates
	MongoSSLCAFile string `json:"mongo_ssl_ca_file" mapstructure:"mongo_ssl_ca_file"`
	// Path to the PEM file which contains both client certificate and private key. This is
	// required for Mutual TLS.
	MongoSSLPEMKeyfile string `json:"mongo_ssl_pem_keyfile" mapstructure:"mongo_ssl_pem_keyfile"`
	// Specifies the mongo DB Type. If it's 0, it means that you are using standard mongo db, but if it's 1 it means you are using AWS Document DB.
	// Defaults to Standard mongo (0).
	MongoDBType MongoType `json:"mongo_db_type" mapstructure:"mongo_db_type"`
	// Set to true to disable the default tyk index creation.
	OmitIndexCreation bool `json:"omit_index_creation" mapstructure:"omit_index_creation"`
}

func (b *BaseMongoConf) GetBlurredURL() string {
	// mongo uri match with regex ^(mongodb:(?:\/{2})?)((\w+?):(\w+?)@|:?@?)(\S+?):(\d+)(\/(\S+?))?(\?replicaSet=(\S+?))?$
	// but we need only a segment, so regex explanation: https://regex101.com/r/E34wQO/1
	regex := `^(mongodb:(?:\/{2})?)((\w+?):(\w+?)@|:?@?)`
	var re = regexp.MustCompile(regex)

	blurredUrl := re.ReplaceAllString(b.MongoURL, "***:***@")
	return blurredUrl
}

// @PumpConf Mongo
type MongoConf struct {
	// TYKCONFIGEXPAND
	BaseMongoConf

	// Specifies the mongo collection name.
	CollectionName string `json:"collection_name" mapstructure:"collection_name"`
	// Maximum insert batch size for mongo selective pump. If the batch we are writing surpass this value, it will be send in multiple batchs.
	// Defaults to 10Mb.
	MaxInsertBatchSizeBytes int `json:"max_insert_batch_size_bytes" mapstructure:"max_insert_batch_size_bytes"`
	// Maximum document size. If the document exceed this value, it will be skipped.
	// Defaults to 10Mb.
	MaxDocumentSizeBytes int `json:"max_document_size_bytes" mapstructure:"max_document_size_bytes"`
	// Amount of bytes of the capped collection in 64bits architectures.
	// Defaults to 5GB.
	CollectionCapMaxSizeBytes int `json:"collection_cap_max_size_bytes" mapstructure:"collection_cap_max_size_bytes"`
	// Enable collection capping. It's used to set a maximum size of the collection.
	CollectionCapEnable bool `json:"collection_cap_enable" mapstructure:"collection_cap_enable"`
}

func loadCertficateAndKeyFromFile(path string) (*tls.Certificate, error) {
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cert tls.Certificate
	for {
		block, rest := pem.Decode(raw)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			cert.Certificate = append(cert.Certificate, block.Bytes)
		} else {
			cert.PrivateKey, err = parsePrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("Failure reading private key from \"%s\": %s", path, err)
			}
		}
		raw = rest
	}

	if len(cert.Certificate) == 0 {
		return nil, fmt.Errorf("No certificate found in \"%s\"", path)
	} else if cert.PrivateKey == nil {
		return nil, fmt.Errorf("No private key found in \"%s\"", path)
	}

	return &cert, nil
}

func parsePrivateKey(der []byte) (crypto.PrivateKey, error) {
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		switch key := key.(type) {
		case *rsa.PrivateKey, *ecdsa.PrivateKey:
			return key, nil
		default:
			return nil, fmt.Errorf("Found unknown private key type in PKCS#8 wrapping")
		}
	}
	if key, err := x509.ParseECPrivateKey(der); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("Failed to parse private key")
}

func GetMongoType(session *mgo.Session) MongoType {
	// Querying for the features which 100% not supported by AWS DocumentDB
	var result struct {
		Code int `bson:"code"`
	}
	session.Run("features", &result)

	if result.Code == 303 {
		return AWSDocumentDB
	} else {
		return StandardMongo
	}
}

func DialInfo(conf BaseMongoConf) (dialInfo *mgo.DialInfo, err error) {
	if dialInfo, err = mgo.ParseURL(conf.MongoURL); err != nil {
		return dialInfo, err
	}

	if conf.MongoUseSSL {
		dialInfo.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
			tlsConfig := &tls.Config{}
			if conf.MongoSSLInsecureSkipVerify {
				tlsConfig.InsecureSkipVerify = true
			}

			if conf.MongoSSLCAFile != "" {
				caCert, err := ioutil.ReadFile(conf.MongoSSLCAFile)
				if err != nil {
					return nil, errors.New("Can't load mongo CA certificates: "+err.Error())
				}
				caCertPool := x509.NewCertPool()
				caCertPool.AppendCertsFromPEM(caCert)
				tlsConfig.RootCAs = caCertPool
			}

			if conf.MongoSSLAllowInvalidHostnames {
				tlsConfig.InsecureSkipVerify = true
				tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
					// Code copy/pasted and adapted from
					// https://github.com/golang/go/blob/81555cb4f3521b53f9de4ce15f64b77cc9df61b9/src/crypto/tls/handshake_client.go#L327-L344, but adapted to skip the hostname verification.
					// See https://github.com/golang/go/issues/21971#issuecomment-412836078.

					// If this is the first handshake on a connection, process and
					// (optionally) verify the server's certificates.
					certs := make([]*x509.Certificate, len(rawCerts))
					for i, asn1Data := range rawCerts {
						cert, err := x509.ParseCertificate(asn1Data)
						if err != nil {
							return err
						}
						certs[i] = cert
					}

					opts := x509.VerifyOptions{
						Roots:         tlsConfig.RootCAs,
						CurrentTime:   time.Now(),
						DNSName:       "", // <- skip hostname verification
						Intermediates: x509.NewCertPool(),
					}

					for i, cert := range certs {
						if i == 0 {
							continue
						}
						opts.Intermediates.AddCert(cert)
					}
					_, err := certs[0].Verify(opts)

					return err
				}
			}

			if conf.MongoSSLPEMKeyfile != "" {
				cert, err := loadCertficateAndKeyFromFile(conf.MongoSSLPEMKeyfile)
				if err != nil {
					return nil, errors.New("Can't load mongo client certificate: "+ err.Error())
				}

				tlsConfig.Certificates = []tls.Certificate{*cert}
			}

			return tls.Dial("tcp", addr.String(), tlsConfig)
		}
	}

	return dialInfo, err
}

func (m *MongoPump) New() Pump {
	newPump := MongoPump{}
	return &newPump
}

func (m *MongoPump) GetName() string {
	return "MongoDB Pump"
}

func (m *MongoPump) GetEnvPrefix() string {
	return m.dbConf.EnvPrefix
}

func (m *MongoPump) Init(config interface{}) error {
	m.dbConf = &MongoConf{}
	m.Log = log.WithField("prefix", mongoPrefix)

	err := mapstructure.Decode(config, &m.dbConf)
	if err == nil {
		err = mapstructure.Decode(config, &m.dbConf.BaseMongoConf)
		m.Log.WithFields(logrus.Fields{
			"url":             m.dbConf.GetBlurredURL(),
			"collection_name": m.dbConf.CollectionName,
		}).Info("Init")
		if err != nil {
			panic(m.dbConf.BaseMongoConf)
		}
	}
	if err != nil {
		m.Log.Fatal("Failed to decode configuration: ", err)
	}

	//we check for the environment configuration if this pumps is not the uptime pump
	if !m.IsUptime {
		m.ProcessEnvVars(m.Log, m.dbConf, mongoDefaultEnv)

		//we keep this env check for backward compatibility
		overrideErr := envconfig.Process(mongoPumpPrefix, m.dbConf)
		if overrideErr != nil {
			m.Log.Error("Failed to process environment variables for mongo pump: ", overrideErr)
		}
	} else if m.IsUptime && m.dbConf.MongoURL == "" {
		m.Log.Debug("Trying to set uptime pump with PMP_MONGO env vars")
		//we keep this env check for backward compatibility
		overrideErr := envconfig.Process(mongoPumpPrefix, m.dbConf)
		if overrideErr != nil {
			m.Log.Error("Failed to process environment variables for mongo pump: ", overrideErr)
		}
	}

	if m.dbConf.MaxInsertBatchSizeBytes == 0 {
		m.Log.Info("-- No max batch size set, defaulting to 10MB")
		m.dbConf.MaxInsertBatchSizeBytes = 10 * MiB
	}

	if m.dbConf.MaxDocumentSizeBytes == 0 {
		m.Log.Info("-- No max document size set, defaulting to 10MB")
		m.dbConf.MaxDocumentSizeBytes = 10 * MiB
	}

	m.connect()

	m.capCollection()

	indexCreateErr := m.ensureIndexes()
	if indexCreateErr != nil {
		m.Log.Error(indexCreateErr)
	}

	m.Log.Debug("MongoDB DB CS: ", m.dbConf.GetBlurredURL())
	m.Log.Debug("MongoDB Col: ", m.dbConf.CollectionName)

	m.Log.Info(m.GetName() + " Initialized")

	return nil
}

func (m *MongoPump) capCollection() (ok bool) {

	var colName = m.dbConf.CollectionName
	var colCapMaxSizeBytes = m.dbConf.CollectionCapMaxSizeBytes
	var colCapEnable = m.dbConf.CollectionCapEnable

	if !colCapEnable {
		return false
	}

	exists, err := m.collectionExists(colName)
	if err != nil {
		m.Log.Errorf("Unable to determine if collection (%s) exists. Not capping collection: %s", colName, err.Error())

		return false
	}

	if exists {
		m.Log.Warnf("Collection (%s) already exists. Capping could result in data loss. Ignoring", colName)

		return false
	}

	if strconv.IntSize < 64 {
		m.Log.Warn("Pump running < 64bit architecture. Not capping collection as max size would be 2gb")

		return false
	}

	if colCapMaxSizeBytes == 0 {
		defaultBytes := 5
		colCapMaxSizeBytes = defaultBytes * GiB

		m.Log.Infof("-- No max collection size set for %s, defaulting to %d", colName, colCapMaxSizeBytes)
	}

	sess := m.dbSession.Copy()
	defer sess.Close()

	err = m.dbSession.DB("").C(colName).Create(&mgo.CollectionInfo{Capped: true, MaxBytes: colCapMaxSizeBytes})
	if err != nil {
		m.Log.Errorf("Unable to create capped collection for (%s). %s", colName, err.Error())

		return false
	}

	m.Log.Infof("Capped collection (%s) created. %d bytes", colName, colCapMaxSizeBytes)

	return true
}

// collectionExists checks to see if a collection name exists in the db.
func (m *MongoPump) collectionExists(name string) (bool, error) {
	sess := m.dbSession.Copy()
	defer sess.Close()

	colNames, err := sess.DB("").CollectionNames()
	if err != nil {
		m.Log.Error("Unable to get collection names: ", err)

		return false, err
	}

	for _, coll := range colNames {
		if coll == name {
			return true, nil
		}
	}

	return false, nil
}

func (m *MongoPump) ensureIndexes() error {
	if m.dbConf.OmitIndexCreation {
		m.Log.Debug("omit_index_creation set to true, omitting index creation..")
		return nil
	}

	if m.dbConf.MongoDBType == StandardMongo {
		exists, errExists := m.collectionExists(m.dbConf.CollectionName)
		if errExists == nil && exists {
			m.Log.Info("Collection ", m.dbConf.CollectionName, " exists, omitting index creation..")
			return nil
		}
	}

	var err error

	sess := m.dbSession.Copy()
	defer sess.Close()

	c := sess.DB("").C(m.dbConf.CollectionName)

	orgIndex := mgo.Index{
		Key:        []string{"orgid"},
		Background: m.dbConf.MongoDBType == StandardMongo,
	}

	err = c.EnsureIndex(orgIndex)
	if err != nil {
		return err
	}

	apiIndex := mgo.Index{
		Key:        []string{"apiid"},
		Background: m.dbConf.MongoDBType == StandardMongo,
	}

	err = c.EnsureIndex(apiIndex)
	if err != nil {
		return err
	}

	logBrowserIndex := mgo.Index{
		Name:       "logBrowserIndex",
		Key:        []string{"-timestamp", "orgid", "apiid", "apikey", "responsecode"},
		Background: m.dbConf.MongoDBType == StandardMongo,
	}

	err = c.EnsureIndex(logBrowserIndex)
	if err != nil && !strings.Contains(err.Error(), "already exists with a different name") {
		return err
	}

	return nil
}

func (m *MongoPump) connect() {
	var err error
	var dialInfo *mgo.DialInfo

	dialInfo, err = DialInfo(m.dbConf.BaseMongoConf)
	if err != nil {
		m.Log.Panic("Mongo URL is invalid: ", err)
	}

	if m.Timeout > 0 {
		dialInfo.Timeout = time.Second * time.Duration(m.Timeout)
	}
	m.dbSession, err = mgo.DialWithInfo(dialInfo)

	for err != nil {
		m.Log.WithError(err).WithField("dialinfo", m.dbConf.BaseMongoConf.GetBlurredURL()).Error("Mongo connection failed. Retrying.")
		time.Sleep(5 * time.Second)
		m.dbSession, err = mgo.DialWithInfo(dialInfo)
	}

	if err == nil && m.dbConf.MongoDBType == 0 {
		m.dbConf.MongoDBType = GetMongoType(m.dbSession)
	}
}

func (m *MongoPump) WriteData(ctx context.Context, data []interface{}) error {

	collectionName := m.dbConf.CollectionName
	if collectionName == "" {
		m.Log.Fatal("No collection name!")
	}

	m.Log.Debug("Attempting to write ", len(data), " records...")

	for m.dbSession == nil {
		m.Log.Debug("Connecting to analytics store")
		m.connect()
	}
	accumulateSet := m.AccumulateSet(data)

	errCh := make(chan error, len(accumulateSet))
	for _, dataSet := range accumulateSet {
		go func(dataSet []interface{}, errCh chan error) {
			sess := m.dbSession.Copy()
			defer sess.Close()

			analyticsCollection := sess.DB("").C(collectionName)

			m.Log.WithFields(logrus.Fields{
				"collection":        collectionName,
				"number of records": len(dataSet),
			}).Debug("Attempt to purge records")

			err := analyticsCollection.Insert(dataSet...)
			if err != nil {
				m.Log.WithFields(logrus.Fields{"collection": collectionName, "number of records": len(dataSet)}).Error("Problem inserting to mongo collection: ", err)

				if strings.Contains(strings.ToLower(err.Error()), "closed explicitly") {
					m.Log.Warning("--> Detected connection failure!")
				}
				errCh <- err
			}
			errCh <- nil
			m.Log.WithFields(logrus.Fields{
				"collection":        collectionName,
				"number of records": len(dataSet),
			}).Info("Completed purging the records")
		}(dataSet, errCh)
	}

	for range accumulateSet {
		select {
		case err := <-errCh:
			if err != nil {
				return err
			}
		}
	}
	m.Log.Info("Purged ", len(data), " records...")

	return nil
}

func (m *MongoPump) AccumulateSet(data []interface{}) [][]interface{} {

	accumulatorTotal := 0
	returnArray := make([][]interface{}, 0)
	thisResultSet := make([]interface{}, 0)

	for i, item := range data {
		thisItem := item.(analytics.AnalyticsRecord)
		if thisItem.ResponseCode == -1 {
			continue
		}

		// Add 1 KB for metadata as average
		sizeBytes := len(thisItem.RawRequest) + len(thisItem.RawResponse) + 1024

		m.Log.Debug("Size is: ", sizeBytes)

		if sizeBytes > m.dbConf.MaxDocumentSizeBytes {
			m.Log.Warning("Document too large, not writing raw request and raw response!")

			thisItem.RawRequest = ""
			thisItem.RawResponse = base64.StdEncoding.EncodeToString([]byte("Document too large, not writing raw request and raw response!"))
		}

		if (accumulatorTotal + sizeBytes) <= m.dbConf.MaxInsertBatchSizeBytes {
			accumulatorTotal += sizeBytes
		} else {
			m.Log.Debug("Created new chunk entry")
			if len(thisResultSet) > 0 {
				returnArray = append(returnArray, thisResultSet)
			}

			thisResultSet = make([]interface{}, 0)
			accumulatorTotal = sizeBytes
		}

		m.Log.Debug("Accumulator is: ", accumulatorTotal)
		thisResultSet = append(thisResultSet, thisItem)

		m.Log.Debug(accumulatorTotal, " of ", m.dbConf.MaxInsertBatchSizeBytes)
		// Append the last element if the loop is about to end
		if i == (len(data) - 1) {
			m.Log.Debug("Appending last entry")
			returnArray = append(returnArray, thisResultSet)
		}
	}

	return returnArray
}

// WriteUptimeData will pull the data from the in-memory store and drop it into the specified MongoDB collection
func (m *MongoPump) WriteUptimeData(data []interface{}) {

	for m.dbSession == nil {
		m.Log.Debug("Connecting to mongoDB store")
		m.connect()
	}

	collectionName := "tyk_uptime_analytics"
	sess := m.dbSession.Copy()
	defer sess.Close()

	analyticsCollection := sess.DB("").C(collectionName)

	m.Log.Debug("Uptime Data: ", len(data))

	if len(data) == 0 {
		return
	}

	keys := make([]interface{}, len(data))

	for i, v := range data {
		decoded := analytics.UptimeReportData{}

		if err := msgpack.Unmarshal([]byte(v.(string)), &decoded); err != nil {
			// ToDo: should this work with serializer?
			m.Log.Error("Couldn't unmarshal analytics data:", err)
			continue
		}

		keys[i] = interface{}(decoded)

		m.Log.Debug("Decoded Record: ", decoded)
	}

	m.Log.Debug("Writing data to ", collectionName)

	if err := analyticsCollection.Insert(keys...); err != nil {

		m.Log.Error("Problem inserting to mongo collection: ", err)

		if strings.Contains(err.Error(), "Closed explicitly") || strings.Contains(err.Error(), "EOF") {
			m.Log.Warning("--> Detected connection failure, reconnecting")

			m.connect()
		}
	}
}
