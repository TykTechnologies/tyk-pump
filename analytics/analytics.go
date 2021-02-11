package analytics

import (
	"encoding/base64"
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
	OpenConnections  int64 `json:"open_conn"`
	ClosedConnection int64 `json:"close_conn"`
	BytesIn          int64 `json:"bytes_in"`
	BytesOut         int64 `json:"bytes_out"`
}

type Latency struct {
	Total    int64 `json:"total"`
	Upstream int64 `json:"upstream"`
}

// AnalyticsRecord encodes the details of a request
type AnalyticsRecord struct {
	Method        string       `json:"method"`
	Host          string       `json:"host"`
	Path          string       `json:"path"`
	RawPath       string       `json:"raw_path" gorm:"column:rawpath"`
	ContentLength int64        `json:"content_length" gorm:"column:contentlength"`
	UserAgent     string       `json:"user_agent" gorm:"column:useragent"`
	Day           int          `json:"day" gorm:"-"`
	Month         time.Month   `json:"month" gorm:"-"`
	Year          int          `json:"year" gorm:"-"`
	Hour          int          `json:"hour" gorm:"-"`
	ResponseCode  int          `json:"response_code" gorm:"column:responsecode;index"`
	APIKey        string       `json:"api_key" gorm:"column:apikey;index"`
	TimeStamp     time.Time    `json:"timestamp" gorm:"column:timestamp;index"`
	APIVersion    string       `json:"api_version" gorm:"column:apiversion"`
	APIName       string       `json:"api_name" gorm:"-"`
	APIID         string       `json:"api_id" gorm:"column:apiid;index"`
	OrgID         string       `json:"org_id" gorm:"column:orgid;index"`
	OauthID       string       `json:"oauth_id" gorm:"column:oauthid;index"`
	RequestTime   int64        `json:"request_time" gorm:"column:requesttime"`
	RawRequest    string       `json:"raw_request" gorm:"column:rawrequest"`
	RawResponse   string       `json:"raw_response" gorm:"column:rawresponse"`
	IPAddress     string       `json:"ip_address" gorm:"column:ipaddress"`
	Geo           GeoData      `json:"geo" gorm:"embedded"`
	Network       NetworkStats `json:"network"`
	Latency       Latency      `json:"latency"`
	Tags          []string     `json:"tags"`
	Alias         string       `json:"alias"`
	TrackPath     bool         `json:"track_path" gorm:"column:trackpath"`
	ExpireAt      time.Time    `bson:"expireAt" json:"expireAt"`
}

func (ar *AnalyticsRecord) TableName() string {
	return "tyk_analytics"
}

type GeoData struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code" json:"iso_code"`
	} `maxminddb:"country" json:"country"`

	City struct {
		GeoNameID uint              `maxminddb:"geoname_id" json:"geoname_id"`
		Names     map[string]string `maxminddb:"names" json:"names"`
	} `maxminddb:"city" json:"city"`

	Location struct {
		Latitude  float64 `maxminddb:"latitude" json:"latitude"`
		Longitude float64 `maxminddb:"longitude" json:"longitude"`
		TimeZone  string  `maxminddb:"time_zone" json:"time_zone"`
	} `maxminddb:"location" json:"location"`
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

	//Algorithm: Lookup the key and use it as the separator. Once the string is split, we obfuscate the key anf concatenate back
	//the 2 parts with the obfuscated key in the middle
	//
	//Example:
	// GET ...Authorization: 59d27324b8125f000137663e2c650c3576b348bfbe1490fef5db0c49 ...\r\n
	// GET ...Authorization: ****0c49 ...\r\n
	requestWithoutKey := strings.Split(sDecodedRequest, fullApiKey)
	if len(requestWithoutKey) != 2 {
		log.WithFields(logrus.Fields{
			"prefix":          analyticsRecordPrefix,
			"apiId":           a.APIID,
			"apiName:version": a.APIName + ":" + a.APIVersion,
			"encoded request": a.RawRequest,
			"decoded request": sDecodedRequest,
		}).Error("Authorization key hasn't been found in the decoded string")

		return
	}
	reqWithObfuscatedKey := requestWithoutKey[0] + a.APIKey + requestWithoutKey[1]

	a.RawRequest = base64.StdEncoding.EncodeToString([]byte(reqWithObfuscatedKey))
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
