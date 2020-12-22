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
	"fmt"
	"io/ioutil"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
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
	dbSession *mgo.Session
	dbConf    *MongoConf
	CommonPumpConfig
}

var mongoPrefix = "mongo-pump"
var mongoPumpPrefix = "PMP_MONGO"

type MongoType int

const (
	StandardMongo MongoType = iota
	AWSDocumentDB
)

type BaseMongoConf struct {
	MongoURL                      string    `json:"mongo_url" mapstructure:"mongo_url"`
	MongoUseSSL                   bool      `json:"mongo_use_ssl" mapstructure:"mongo_use_ssl"`
	MongoSSLInsecureSkipVerify    bool      `json:"mongo_ssl_insecure_skip_verify" mapstructure:"mongo_ssl_insecure_skip_verify"`
	MongoSSLAllowInvalidHostnames bool      `json:"mongo_ssl_allow_invalid_hostnames" mapstructure:"mongo_ssl_allow_invalid_hostnames"`
	MongoSSLCAFile                string    `json:"mongo_ssl_ca_file" mapstructure:"mongo_ssl_ca_file"`
	MongoSSLPEMKeyfile            string    `json:"mongo_ssl_pem_keyfile" mapstructure:"mongo_ssl_pem_keyfile"`
	MongoDBType                   MongoType `json:"mongo_db_type" mapstructure:"mongo_db_type"`
}

func (b *BaseMongoConf) GetBlurredURL() string {
	// mongo uri match with regex ^(mongodb:(?:\/{2})?)((\w+?):(\w+?)@|:?@?)(\S+?):(\d+)(\/(\S+?))?(\?replicaSet=(\S+?))?$
	// but we need only a segment, so regex explanation: https://regex101.com/r/E34wQO/1
	regex := `^(mongodb:(?:\/{2})?)((\w+?):(\w+?)@|:?@?)`
	var re = regexp.MustCompile(regex)

	blurredUrl := re.ReplaceAllString(b.MongoURL, "db_username:db_password@")
	return blurredUrl
}

type MongoConf struct {
	BaseMongoConf

	CollectionName            string `json:"collection_name" mapstructure:"collection_name"`
	MaxInsertBatchSizeBytes   int    `json:"max_insert_batch_size_bytes" mapstructure:"max_insert_batch_size_bytes"`
	MaxDocumentSizeBytes      int    `json:"max_document_size_bytes" mapstructure:"max_document_size_bytes"`
	CollectionCapMaxSizeBytes int    `json:"collection_cap_max_size_bytes" mapstructure:"collection_cap_max_size_bytes"`
	CollectionCapEnable       bool   `json:"collection_cap_enable" mapstructure:"collection_cap_enable"`
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

func mongoType(session *mgo.Session) MongoType {
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

func mongoDialInfo(conf BaseMongoConf) (dialInfo *mgo.DialInfo, err error) {
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
					log.Fatal("Can't load mongo CA certificates: ", err)
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
					log.Fatal("Can't load mongo client certificate: ", err)
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

func (m *MongoPump) Init(config interface{}) error {
	m.dbConf = &MongoConf{}
	err := mapstructure.Decode(config, &m.dbConf)
	if err == nil {
		err = mapstructure.Decode(config, &m.dbConf.BaseMongoConf)
		log.WithFields(logrus.Fields{
			"prefix":          mongoPrefix,
			"url":             m.dbConf.GetBlurredURL(),
			"collection_name": m.dbConf.CollectionName,
		}).Info("Init")
		if err != nil {
			panic(m.dbConf.BaseMongoConf)
		}
	}

	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Fatal("Failed to decode configuration: ", err)
	}

	overrideErr := envconfig.Process(mongoPumpPrefix, m.dbConf)
	if overrideErr != nil {
		log.Error("Failed to process environment variables for mongo pump: ", overrideErr)
	}

	if m.dbConf.MaxInsertBatchSizeBytes == 0 {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Info("-- No max batch size set, defaulting to 10MB")
		m.dbConf.MaxInsertBatchSizeBytes = 10 * MiB
	}

	if m.dbConf.MaxDocumentSizeBytes == 0 {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Info("-- No max document size set, defaulting to 10MB")
		m.dbConf.MaxDocumentSizeBytes = 10 * MiB
	}

	m.connect()

	m.capCollection()

	indexCreateErr := m.ensureIndexes()
	if indexCreateErr != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Error(indexCreateErr)
	}

	log.WithFields(logrus.Fields{
		"prefix": mongoPrefix,
	}).Debug("MongoDB DB CS: ", m.dbConf.GetBlurredURL())
	log.WithFields(logrus.Fields{
		"prefix": mongoPrefix,
	}).Debug("MongoDB Col: ", m.dbConf.CollectionName)

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
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Errorf("Unable to determine if collection (%s) exists. Not capping collection: %s", colName, err.Error())

		return false
	}

	if exists {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Warnf("Collection (%s) already exists. Capping could result in data loss. Ignoring", colName)

		return false
	}

	if strconv.IntSize < 64 {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Warn("Pump running < 64bit architecture. Not capping collection as max size would be 2gb")

		return false
	}

	if colCapMaxSizeBytes == 0 {
		defaultBytes := 5
		colCapMaxSizeBytes = defaultBytes * GiB

		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Infof("-- No max collection size set for %s, defaulting to %d", colName, colCapMaxSizeBytes)
	}

	sess := m.dbSession.Copy()
	defer sess.Close()

	err = m.dbSession.DB("").C(colName).Create(&mgo.CollectionInfo{Capped: true, MaxBytes: colCapMaxSizeBytes})
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Errorf("Unable to create capped collection for (%s). %s", colName, err.Error())

		return false
	}

	log.WithFields(logrus.Fields{
		"prefix": mongoPrefix,
	}).Infof("Capped collection (%s) created. %d bytes", colName, colCapMaxSizeBytes)

	return true
}

// collectionExists checks to see if a collection name exists in the db.
func (m *MongoPump) collectionExists(name string) (bool, error) {
	sess := m.dbSession.Copy()
	defer sess.Close()

	colNames, err := sess.DB("").CollectionNames()
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Error("Unable to get column names: ", err)

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

	dialInfo, err = mongoDialInfo(m.dbConf.BaseMongoConf)
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Panic("Mongo URL is invalid: ", err)
	}

	if m.timeout > 0 {
		dialInfo.Timeout = time.Second * time.Duration(m.timeout)
	}
	m.dbSession, err = mgo.DialWithInfo(dialInfo)

	for err != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).WithError(err).WithField("dialinfo", dialInfo).Error("Mongo connection failed. Retrying.")
		time.Sleep(5 * time.Second)
		m.dbSession, err = mgo.DialWithInfo(dialInfo)
	}

	if err == nil && m.dbConf.MongoDBType == 0 {
		m.dbConf.MongoDBType = mongoType(m.dbSession)
	}
}

func (m *MongoPump) WriteData(ctx context.Context, data []interface{}) error {

	collectionName := m.dbConf.CollectionName
	if collectionName == "" {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Fatal("No collection name!")
	}

	log.WithFields(logrus.Fields{
		"prefix": mongoPrefix,
	}).Debug("Writing ", len(data), " records")

	for m.dbSession == nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Debug("Connecting to analytics store")
		m.connect()
	}
	accumulateSet := m.AccumulateSet(data)

	errCh := make(chan error, len(accumulateSet))
	for _, dataSet := range accumulateSet {
		go func(dataSet []interface{}, errCh chan error) {
			sess := m.dbSession.Copy()
			defer sess.Close()

			analyticsCollection := sess.DB("").C(collectionName)

			log.WithFields(logrus.Fields{
				"prefix": mongoPrefix,
			}).Info("Purging ", len(dataSet), " records")

			err := analyticsCollection.Insert(dataSet...)
			if err != nil {
				log.Error("Problem inserting to mongo collection: ", err)
				if strings.Contains(strings.ToLower(err.Error()), "closed explicitly") {
					log.Warning("--> Detected connection failure!")
				}
				errCh <- err
			}
			errCh <- nil
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

		log.Debug("Size is: ", sizeBytes)

		if sizeBytes > m.dbConf.MaxDocumentSizeBytes {
			log.WithFields(logrus.Fields{
				"prefix": mongoPrefix,
			}).Warning("Document too large, not writing raw request and raw response!")

			thisItem.RawRequest = ""
			thisItem.RawResponse = base64.StdEncoding.EncodeToString([]byte("Document too large, not writing raw request and raw response!"))
		}

		if (accumulatorTotal + sizeBytes) <= m.dbConf.MaxInsertBatchSizeBytes {
			accumulatorTotal += sizeBytes
		} else {
			log.Debug("Created new chunk entry")
			if len(thisResultSet) > 0 {
				returnArray = append(returnArray, thisResultSet)
			}

			thisResultSet = make([]interface{}, 0)
			accumulatorTotal = sizeBytes
		}

		log.Debug("Accumulator is: ", accumulatorTotal)
		thisResultSet = append(thisResultSet, thisItem)

		log.Debug(accumulatorTotal, " of ", m.dbConf.MaxInsertBatchSizeBytes)
		// Append the last element if the loop is about to end
		if i == (len(data) - 1) {
			log.Debug("Appending last entry")
			returnArray = append(returnArray, thisResultSet)
		}
	}

	return returnArray
}

// WriteUptimeData will pull the data from the in-memory store and drop it into the specified MongoDB collection
func (m *MongoPump) WriteUptimeData(data []interface{}) {

	for m.dbSession == nil {
		log.Debug("Connecting to mongoDB store")
		m.connect()
	}

	collectionName := "tyk_uptime_analytics"
	sess := m.dbSession.Copy()
	defer sess.Close()

	analyticsCollection := sess.DB("").C(collectionName)

	log.WithFields(logrus.Fields{
		"prefix": mongoPrefix,
	}).Debug("Uptime Data: ", len(data))

	if len(data) == 0 {
		return
	}

	keys := make([]interface{}, len(data))

	for i, v := range data {
		decoded := analytics.UptimeReportData{}

		if err := msgpack.Unmarshal([]byte(v.(string)), &decoded); err != nil {
			log.WithFields(logrus.Fields{
				"prefix": mongoPrefix,
			}).Error("Couldn't unmarshal analytics data:", err)

			continue
		}

		keys[i] = interface{}(decoded)

		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Debug("Decoded Record: ", decoded)
	}

	log.WithFields(logrus.Fields{
		"prefix": mongoPrefix,
	}).Debug("Writing data to ", collectionName)

	if err := analyticsCollection.Insert(keys...); err != nil {

		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Error("Problem inserting to mongo collection: ", err)

		if strings.Contains(err.Error(), "Closed explicitly") || strings.Contains(err.Error(), "EOF") {
			log.WithFields(logrus.Fields{
				"prefix": mongoPrefix,
			}).Warning("--> Detected connection failure, reconnecting")

			m.connect()
		}
	}
}
