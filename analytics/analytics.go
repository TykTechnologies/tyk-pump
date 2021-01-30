package analytics

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/logger"
)

var log = logger.GetLogger()
var analyticsRecordPrefix = "analyticsRecord"

type NetworkStats struct {
	OpenConnections  int64
	ClosedConnection int64
	BytesIn          int64
	BytesOut         int64
}

type Latency struct {
	Total    int64
	Upstream int64
}

// AnalyticsRecord encodes the details of a request
type AnalyticsRecord struct {
	Method        string
	Host          string
	Path          string
	RawPath       string
	ContentLength int64
	UserAgent     string
	Day           int
	Month         time.Month
	Year          int
	Hour          int
	ResponseCode  int
	APIKey        string
	TimeStamp     time.Time
	APIVersion    string
	APIName       string
	APIID         string
	OrgID         string
	OauthID       string
	RequestTime   int64
	RawRequest    string
	RawResponse   string
	IPAddress     string
	Geo           GeoData
	Network       NetworkStats
	Latency       Latency
	Tags          []string
	Alias         string
	TrackPath     bool
	ExpireAt      time.Time `bson:"expireAt" json:"expireAt"`
}

type GeoData struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`

	City struct {
		GeoNameID uint              `maxminddb:"geoname_id"`
		Names     map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`

	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
		TimeZone  string  `maxminddb:"time_zone"`
	} `maxminddb:"location"`
}

func (a *AnalyticsRecord) GetFieldNames() []string {
	val := reflect.ValueOf(a).Elem()
	fields := []string{}

	for i := 0; i < val.NumField(); i++ {
		typeField := val.Type().Field(i)
		fields = append(fields, typeField.Name)
	}

	return fields
}

func (a *AnalyticsRecord) GetLineValues() []string {
	val := reflect.ValueOf(a).Elem()
	fields := []string{}

	for i := 0; i < val.NumField(); i++ {
		valueField := val.Field(i)
		typeField := val.Type().Field(i)
		thisVal := ""
		switch typeField.Type.String() {
		case "int":
			thisVal = strconv.Itoa(int(valueField.Int()))
		case "int64":
			thisVal = strconv.Itoa(int(valueField.Int()))
		case "[]string":
			tmpVal := valueField.Interface().([]string)
			thisVal = strings.Join(tmpVal, ";")
		case "time.Time":
			tmpVal := valueField.Interface().(time.Time)
			thisVal = tmpVal.String()
		case "time.Month":
			tmpVal := valueField.Interface().(time.Month)
			thisVal = tmpVal.String()
		default:
			thisVal = valueField.String()
		}

		fields = append(fields, thisVal)
	}

	return fields
}

//change name - obfuscateAndDecode request
func (a *AnalyticsRecord) ObfuscateKey() {

	if a.APIKey == "" {
		return
	}

	fullApiKey := a.APIKey
	a.APIKey = ObfuscateString(a.APIKey) //Obfuscate the key field

	if a.RawRequest == "" {
		return
	}

	decodeRequest, err := base64.StdEncoding.DecodeString(a.RawRequest)
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix":          analyticsRecordPrefix,
			"apiId":           a.APIID,
			"apiName:version": a.APIName + ":" + a.APIVersion,
			"error":           err,
		}).Error("Error while decoding raw request ", a.RawRequest)

		return
	}

	sDecodedRequest := string(decodeRequest)
	log.WithFields(logrus.Fields{
		"prefix":          analyticsRecordPrefix,
		"apiId":           a.APIID,
		"apiName:version": a.APIName + ":" + a.APIVersion,
		"encoded request": a.RawRequest,
		"decoded request": sDecodedRequest,
	}).Debug("")

	//todo bearer!

	//Lookup the key and use it as the separator. Once the string is split, we obfuscate the key anf concatenate back
	//the 2 parts with the obfuscated key in the middle
	//
	//Example:
	// GET ...Authorization: 59d27324b8125f000137663e2c650c3576b348bfbe1490fef5db0c49 ...\r\n
	// GET ...Authorization: ****0c49 ...\r\n
	fmt.Println(" before a.APIKey  " + a.APIKey)
	requestWithoutKey := strings.Split(sDecodedRequest, fullApiKey)
	if len(requestWithoutKey) != 2 {
		log.WithFields(logrus.Fields{
			"prefix":          analyticsRecordPrefix,
			"apiId":           a.APIID,
			"apiName:version": a.APIName + ":" + a.APIVersion,
			"encoded request": a.RawRequest,
			"decoded request": sDecodedRequest,
		}).Error("Authorization key hasn't been found in the decoded string")
		log.Debug("Authorization key hasn't been found, split array length:", len(requestWithoutKey))

		return
	}
	reqWithObfuscatedKey := requestWithoutKey[0] + a.APIKey + requestWithoutKey[1]

	a.RawRequest = base64.StdEncoding.EncodeToString([]byte(reqWithObfuscatedKey))
	log.Debug("Encoded RawRequest:", a.RawRequest)
}

func ObfuscateString(keyName string) string {

	if keyName == "" {
		return ""
	}

	if len(keyName) > 4 {
		return "****" + keyName[len(keyName)-4:]
	}
	return "----"
}
