package pumps

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	b64 "encoding/base64"
	"errors"
	"io/ioutil"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/TykTechnologies/tyk-pump/pumps/common"
	mgo2 "github.com/TykTechnologies/tyk-pump/pumps/internal/mgo"
	"github.com/TykTechnologies/tyk-pump/pumps/mongo"
	"github.com/kelseyhightower/envconfig"
	"github.com/lonelycode/mgohacks"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
)

var mongoAggregatePumpPrefix = "PMP_MONGOAGG"
var mongoAggregateDefaultEnv = common.PUMPS_ENV_PREFIX + "_MONGOAGGREGATE" + common.PUMPS_ENV_META_PREFIX

var THRESHOLD_LEN_TAG_LIST = 1000
var COMMON_TAGS_COUNT = 5

type MongoAggregatePump struct {
	dbSession *mgo.Session
	dbConf    *MongoAggregateConf
	common.Pump
}

// @PumpConf MongoAggregate
type MongoAggregateConf struct {
	// TYKCONFIGEXPAND
	mongo.BaseConfig
	// If set to `true` your pump will store analytics to both your organisation defined
	// collections z_tyk_analyticz_aggregate_{ORG ID} and your org-less tyk_analytics_aggregates
	// collection. When set to 'false' your pump will only store analytics to your org defined
	// collection.
	UseMixedCollection bool `json:"use_mixed_collection" mapstructure:"use_mixed_collection"`
	// Specifies if it should store aggregated data for all the endpoints. By default, `false`
	// which means that only store aggregated data for `tracked endpoints`.
	TrackAllPaths bool `json:"track_all_paths" mapstructure:"track_all_paths"`
	// Specifies prefixes of tags that should be ignored.
	IgnoreTagPrefixList []string `json:"ignore_tag_prefix_list" mapstructure:"ignore_tag_prefix_list"`
	// Determines the threshold of amount of tags of an aggregation. If the amount of tags is superior to the threshold,
	// it will print an alert.
	// Defaults to 1000.
	ThresholdLenTagList int `json:"threshold_len_tag_list" mapstructure:"threshold_len_tag_list"`
	// Determines if the aggregations should be made per minute instead of per hour.
	StoreAnalyticsPerMinute bool `json:"store_analytics_per_minute" mapstructure:"store_analytics_per_minute"`
	// This list determines which aggregations are going to be dropped and not stored in the collection.
	// Posible values are: "APIID","errors","versions","apikeys","oauthids","geo","tags","endpoints","keyendpoints",
	// "oauthendpoints", and "apiendpoints".
	IgnoreAggregationsList []string `json:"ignore_aggregations" mapstructure:"ignore_aggregations"`
}

func getListOfCommonPrefix(list []string) []string {
	count := make(map[string]int)
	result := make([]string, 0)
	length := len(list)

	if length == 0 || length == 1 {
		return list
	}

	for i := 0; i < length-1; i++ {
		for j := i + 1; j < length; j++ {
			var prefLen int
			str1 := list[i]
			str2 := list[j]

			if len(str1) > len(str2) {
				prefLen = len(str2)
			} else {
				prefLen = len(str1)
			}

			k := 0
			for k = 0; k < prefLen; k++ {
				if str1[k] != str2[k] {
					if k != 0 {
						count[str1[:k]]++
					}
					break
				}
			}
			if k == prefLen {
				count[str1[:prefLen]]++
			}
		}
	}

	for k := range count {
		result = append(result, k)
	}

	sort.Slice(result, func(i, j int) bool { return count[result[i]] > count[result[j]] })

	return result
}

func (m *MongoAggregatePump) printAlert(doc analytics.AnalyticsRecordAggregate, thresholdLenTagList int) {
	var listofTags []string

	for k := range doc.Tags {
		listofTags = append(listofTags, k)
	}

	listOfCommonPrefix := getListOfCommonPrefix(listofTags)

	// list 5 common tag prefix
	l := len(listOfCommonPrefix)
	if l > COMMON_TAGS_COUNT {
		l = COMMON_TAGS_COUNT
	}

	m.Log.Warnf("WARNING: Found more than %v tag entries per document, which may cause performance issues with aggregate logs. List of most common tag-prefix: [%v]. You can ignore these tags using ignore_tag_prefix_list option", thresholdLenTagList, strings.Join(listOfCommonPrefix[:l], ", "))
}

func (m *MongoAggregatePump) doHash(in string) string {
	sEnc := b64.StdEncoding.EncodeToString([]byte(in))
	search := strings.TrimRight(sEnc, "=")
	return search
}

func (m *MongoAggregatePump) GetName() string {
	return "MongoDB Aggregate Pump"
}

func (m *MongoAggregatePump) GetEnvPrefix() string {
	return m.dbConf.EnvPrefix
}

func (m *MongoAggregatePump) GetCollectionName(orgid string) (string, error) {
	if orgid == "" {
		return "", errors.New("OrgID cannot be empty")
	}

	return "z_tyk_analyticz_aggregate_" + orgid, nil
}

func (m *MongoAggregatePump) Init(config interface{}) error {
	m.dbConf = &MongoAggregateConf{}
	m.Log = log.WithField("prefix", analytics.MongoAggregatePrefix)

	err := mapstructure.Decode(config, &m.dbConf)
	if err == nil {
		err = mapstructure.Decode(config, &m.dbConf.BaseConfig)
	}

	if err != nil {
		m.Log.Fatal("Failed to decode configuration: ", err)
	}

	m.ProcessEnvVars(m.Log, m.dbConf, mongoAggregateDefaultEnv)

	//we keep this env check for backward compatibility
	overrideErr := envconfig.Process(mongoAggregatePumpPrefix, m.dbConf)
	if overrideErr != nil {
		m.Log.Error("Failed to process environment variables for mongo aggregate pump: ", overrideErr)
	}

	if m.dbConf.ThresholdLenTagList == 0 {
		m.dbConf.ThresholdLenTagList = THRESHOLD_LEN_TAG_LIST
	}

	m.connect()

	m.Log.Debug("MongoDB DB CS: ", m.dbConf.GetBlurredURL())
	m.Log.Info(m.GetName() + " Initialized")

	return nil
}

func (m *MongoAggregatePump) connect() {

	var err error
	var dialInfo *mgo.DialInfo

	dialInfo, err = mongoDialInfo(m.dbConf.BaseConfig)
	if err != nil {
		m.Log.Panic("Mongo URL is invalid: ", err)
	}

	if m.Timeout > 0 {
		dialInfo.Timeout = time.Second * time.Duration(m.Timeout)
	}

	m.dbSession, err = mgo.DialWithInfo(dialInfo)

	for err != nil {
		m.Log.WithError(err).WithField("dialinfo", m.dbConf.BaseConfig.GetBlurredURL()).Error("Mongo connection failed. Retrying.")
		time.Sleep(5 * time.Second)
		m.dbSession, err = mgo.DialWithInfo(dialInfo)
	}

	if err == nil && m.dbConf.MongoDBType == 0 {
		sessManager := mgo2.NewSessionManager(m.dbSession)
		m.dbConf.MongoDBType = mongo.GetMongoType(sessManager)
	}
}

func mongoDialInfo(conf mongo.BaseConfig) (dialInfo *mgo.DialInfo, err error) {
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
				cert, err := mongo.LoadCertficateAndKeyFromFile(conf.MongoSSLPEMKeyfile)
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

func (m *MongoAggregatePump) ensureIndexes(c *mgo.Collection) error {
	if m.dbConf.OmitIndexCreation {
		m.Log.Debug("omit_index_creation set to true, omitting index creation..")
		return nil
	}

	//We are going to check if the collection exists only when the DB Type is MongoDB. The mgo CollectionNames func leaks cursors on DocDB.
	if m.dbConf.MongoDBType == mongo.StandardMongo {
		exists, errExists := m.collectionExists(c.Name)
		if errExists == nil && exists {
			m.Log.Debug("Collection ", c.Name, " exists, omitting index creation")
			return nil
		}
	}

	var err error
	ttlIndex := mgo.Index{
		Key:         []string{"expireAt"},
		ExpireAfter: 0,
		Background:  m.dbConf.MongoDBType == mongo.StandardMongo,
	}

	err = mgohacks.EnsureTTLIndex(c, ttlIndex)
	if err != nil {
		return err
	}

	apiIndex := mgo.Index{
		Key:        []string{"timestamp"},
		Background: m.dbConf.MongoDBType == mongo.StandardMongo,
	}

	err = c.EnsureIndex(apiIndex)
	if err != nil {
		return err
	}

	orgIndex := mgo.Index{
		Key:        []string{"orgid"},
		Background: m.dbConf.MongoDBType == mongo.StandardMongo,
	}

	return c.EnsureIndex(orgIndex)
}

func (m *MongoAggregatePump) WriteData(ctx context.Context, data []interface{}) error {
	m.Log.Debug("Attempting to write ", len(data), " records")
	if m.dbSession == nil {
		m.Log.Debug("Connecting to analytics store")
		m.connect()
		m.WriteData(ctx, data)
	} else {
		// calculate aggregates
		analyticsPerOrg := analytics.AggregateData(data, m.dbConf.TrackAllPaths, m.dbConf.IgnoreTagPrefixList, m.dbConf.StoreAnalyticsPerMinute)

		// put aggregated data into MongoDB
		for orgID, filteredData := range analyticsPerOrg {
			err := m.DoAggregatedWriting(ctx, orgID, filteredData)
			if err != nil {
				return err
			}

			m.Log.Debug("Processed aggregated data for ", orgID)
		}
	}
	m.Log.Info("Purged ", len(data), " records...")

	return nil
}

func (m *MongoAggregatePump) doMixedWrite(changeDoc analytics.AnalyticsRecordAggregate, query bson.M) {
	thisSession := m.dbSession.Copy()
	defer thisSession.Close()
	analyticsCollection := thisSession.DB("").C(analytics.AgggregateMixedCollectionName)
	m.ensureIndexes(analyticsCollection)

	avgChange := mgo.Change{
		Update:    changeDoc,
		ReturnNew: true,
		Upsert:    true,
	}

	m.Log.WithFields(logrus.Fields{
		"collection": analytics.AgggregateMixedCollectionName,
	}).Debug("Attempt to upsert aggregated doc")

	final := analytics.AnalyticsRecordAggregate{}
	_, avgErr := analyticsCollection.Find(query).Apply(avgChange, &final)

	if avgErr != nil {
		m.Log.WithFields(logrus.Fields{
			"collection": analytics.AgggregateMixedCollectionName,
		}).Error("Mixed coll upsert failure: ", avgErr)
		m.HandleWriteErr(avgErr)
	}
	m.Log.WithFields(logrus.Fields{
		"collection": analytics.AgggregateMixedCollectionName,
	}).Info("Completed upserting")
}

func (m *MongoAggregatePump) HandleWriteErr(err error) error {
	if err != nil {
		m.Log.Error("Problem inserting or updating to mongo collection: ", err)
		if strings.Contains(err.Error(), "Closed explicitly") || strings.Contains(err.Error(), "EOF") {
			m.Log.Warning("--> Detected connection failure, reconnecting")
			m.connect()
		}
	}
	return err
}

func (m *MongoAggregatePump) DoAggregatedWriting(ctx context.Context, orgID string, filteredData analytics.AnalyticsRecordAggregate) error {
	collectionName, collErr := m.GetCollectionName(orgID)
	if collErr != nil {
		m.Log.Info("No OrgID for AnalyticsRecord, skipping")
		return nil
	}

	thisSession := m.dbSession.Copy()
	defer thisSession.Close()

	analyticsCollection := thisSession.DB("").C(collectionName)
	indexCreateErr := m.ensureIndexes(analyticsCollection)

	if indexCreateErr != nil {
		m.Log.Error(indexCreateErr)
	}

	query := bson.M{
		"orgid":     filteredData.OrgID,
		"timestamp": filteredData.TimeStamp,
	}

	if len(m.dbConf.IgnoreAggregationsList) > 0 {
		filteredData.DiscardAggregations(m.dbConf.IgnoreAggregationsList)
	}

	updateDoc := filteredData.AsChange()

	change := mgo.Change{
		Update:    updateDoc,
		ReturnNew: true,
		Upsert:    true,
	}

	doc := analytics.AnalyticsRecordAggregate{}
	_, err := analyticsCollection.Find(query).Apply(change, &doc)

	if err != nil {
		m.Log.WithField("query", query).Error("UPSERT Failure: ", err)
		return m.HandleWriteErr(err)
	}

	// We have the new doc back, lets fix the averages
	avgUpdateDoc := doc.AsTimeUpdate()
	avgChange := mgo.Change{
		Update:    avgUpdateDoc,
		ReturnNew: true,
	}
	withTimeUpdate := analytics.AnalyticsRecordAggregate{}
	_, avgErr := analyticsCollection.Find(query).Apply(avgChange, &withTimeUpdate)

	if m.dbConf.ThresholdLenTagList != -1 && (len(withTimeUpdate.Tags) > m.dbConf.ThresholdLenTagList) {
		m.printAlert(withTimeUpdate, m.dbConf.ThresholdLenTagList)
	}

	if avgErr != nil {
		m.Log.WithField("query", query).Error("AvgUpdate Failure: ", avgErr)
		return m.HandleWriteErr(avgErr)
	}

	if m.dbConf.UseMixedCollection {
		thisData := analytics.AnalyticsRecordAggregate{}
		err := analyticsCollection.Find(query).One(&thisData)
		if err != nil {
			m.Log.WithField("query", query).Error("Couldn't find query doc:", err)
		} else {
			m.doMixedWrite(thisData, query)
		}
	}
	return nil
}

// collectionExists checks to see if a collection name exists in the db.
func (m *MongoAggregatePump) collectionExists(name string) (bool, error) {
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

// WriteUptimeData will pull the data from the in-memory store and drop it into the specified MongoDB collection
func (m *MongoAggregatePump) WriteUptimeData(data []interface{}) {
	m.Log.Warning("Mongo Aggregate should not be writing uptime data!")
}
