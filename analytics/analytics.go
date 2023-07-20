package analytics

import (
	"bytes"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fatih/structs"
	"github.com/oschwald/maxminddb-golang"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/TykTechnologies/storage/persistent/model"
	analyticsproto "github.com/TykTechnologies/tyk-pump/analytics/proto"

	"github.com/TykTechnologies/tyk-pump/logger"
)

const (
	PredefinedTagGraphAnalytics = "tyk-graph-analytics"
)

var log = logger.GetLogger()

type NetworkStats struct {
	OpenConnections  int64 `json:"open_connections"`
	ClosedConnection int64 `json:"closed_connections"`
	BytesIn          int64 `json:"bytes_in"`
	BytesOut         int64 `json:"bytes_out"`
}

type Latency struct {
	Total    int64 `json:"total"`
	Upstream int64 `json:"upstream"`
}

const SQLTable = "tyk_analytics"

// AnalyticsRecord encodes the details of a request
type AnalyticsRecord struct {
	id            model.ObjectID `bson:"_id" gorm:"-:all"`
	Method        string         `json:"method" gorm:"column:method"`
	Host          string         `json:"host" gorm:"column:host"`
	Path          string         `json:"path" gorm:"column:path"`
	RawPath       string         `json:"raw_path" gorm:"column:rawpath"`
	ContentLength int64          `json:"content_length" gorm:"column:contentlength"`
	UserAgent     string         `json:"user_agent" gorm:"column:useragent"`
	Day           int            `json:"day" sql:"-"`
	Month         time.Month     `json:"month" sql:"-"`
	Year          int            `json:"year" sql:"-"`
	Hour          int            `json:"hour" sql:"-"`
	ResponseCode  int            `json:"response_code" gorm:"column:responsecode;index"`
	APIKey        string         `json:"api_key" gorm:"column:apikey;index"`
	TimeStamp     time.Time      `json:"timestamp" gorm:"column:timestamp;index"`
	APIVersion    string         `json:"api_version" gorm:"column:apiversion"`
	APIName       string         `json:"api_name" sql:"-"`
	APIID         string         `json:"api_id" gorm:"column:apiid;index"`
	OrgID         string         `json:"org_id" gorm:"column:orgid;index"`
	OauthID       string         `json:"oauth_id" gorm:"column:oauthid;index"`
	RequestTime   int64          `json:"request_time" gorm:"column:requesttime"`
	RawRequest    string         `json:"raw_request" gorm:"column:rawrequest"`
	RawResponse   string         `json:"raw_response" gorm:"column:rawresponse"`
	IPAddress     string         `json:"ip_address" gorm:"column:ipaddress"`
	Geo           GeoData        `json:"geo" gorm:"embedded"`
	Network       NetworkStats   `json:"network"`
	Latency       Latency        `json:"latency"`
	Tags          []string       `json:"tags"`
	Alias         string         `json:"alias"`
	TrackPath     bool           `json:"track_path" gorm:"column:trackpath"`
	ExpireAt      time.Time      `bson:"expireAt" json:"expireAt"`
	ApiSchema     string         `json:"api_schema" bson:"-" gorm:"-:all"` //nolint

	CollectionName string `json:"-" bson:"-" gorm:"-:all"`
}

func (a *AnalyticsRecord) TableName() string {
	if a.CollectionName != "" {
		return a.CollectionName
	}
	return SQLTable
}

func (a *AnalyticsRecord) GetObjectID() model.ObjectID {
	return a.id
}

func (a *AnalyticsRecord) SetObjectID(id model.ObjectID) {
	a.id = id
}

type GraphError struct {
	Message string        `json:"message"`
	Path    []interface{} `json:"path"`
}

type Country struct {
	ISOCode string `maxminddb:"iso_code" json:"iso_code"`
}
type City struct {
	GeoNameID uint              `maxminddb:"geoname_id" json:"geoname_id"`
	Names     map[string]string `maxminddb:"names" json:"names"`
}

type Location struct {
	Latitude  float64 `maxminddb:"latitude" json:"latitude"`
	Longitude float64 `maxminddb:"longitude" json:"longitude"`
	TimeZone  string  `maxminddb:"time_zone" json:"time_zone"`
}

type GeoData struct {
	Country  Country  `maxminddb:"country" json:"country"`
	City     City     `maxminddb:"city" json:"city"`
	Location Location `maxminddb:"location" json:"location"`
}

func (n *NetworkStats) GetFieldNames() []string {
	return []string{
		"NetworkStats.OpenConnections",
		"NetworkStats.ClosedConnection",
		"NetworkStats.BytesIn",
		"NetworkStats.BytesOut",
	}
}

func (l *Latency) GetFieldNames() []string {
	return []string{
		"Latency.Total",
		"Latency.Upstream",
	}
}

func (g *GeoData) GetFieldNames() []string {
	return []string{
		"GeoData.Country.ISOCode",
		"GeoData.City.GeoNameID",
		"GeoData.City.Names",
		"GeoData.Location.Latitude",
		"GeoData.Location.Longitude",
		"GeoData.Location.TimeZone",
	}
}

func (a *AnalyticsRecord) GetFieldNames() []string {
	fields := []string{
		"Method",
		"Host",
		"Path",
		"RawPath",
		"ContentLength",
		"UserAgent",
		"Day",
		"Month",
		"Year",
		"Hour",
		"ResponseCode",
		"APIKey",
		"TimeStamp",
		"APIVersion",
		"APIName",
		"APIID",
		"OrgID",
		"OauthID",
		"RequestTime",
		"RawRequest",
		"RawResponse",
		"IPAddress",
	}
	fields = append(fields, a.Geo.GetFieldNames()...)
	fields = append(fields, a.Network.GetFieldNames()...)
	fields = append(fields, a.Latency.GetFieldNames()...)
	return append(fields, "Tags", "Alias", "TrackPath", "ExpireAt", "ApiSchema")
}

func (n *NetworkStats) GetLineValues() []string {
	fields := []string{}
	fields = append(fields, strconv.FormatUint(uint64(n.OpenConnections), 10))
	fields = append(fields, strconv.FormatUint(uint64(n.ClosedConnection), 10))
	fields = append(fields, strconv.FormatUint(uint64(n.BytesIn), 10))
	return append(fields, strconv.FormatUint(uint64(n.BytesOut), 10))
}

func (l *Latency) GetLineValues() []string {
	fields := []string{}
	fields = append(fields, strconv.FormatUint(uint64(l.Total), 10))
	return append(fields, strconv.FormatUint(uint64(l.Upstream), 10))
}

func (g *GeoData) GetLineValues() []string {
	fields := []string{}
	fields = append(fields, g.Country.ISOCode)
	fields = append(fields, strconv.FormatUint(uint64(g.City.GeoNameID), 10))
	keys := make([]string, 0, len(g.City.Names))
	for k := range g.City.Names {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var cityNames string
	first := true
	for _, key := range keys {
		keyval := g.City.Names[key]
		if first {
			first = false
			cityNames = fmt.Sprintf("%s:%s", key, keyval)
		} else {
			cityNames = fmt.Sprintf("%s;%s:%s", cityNames, key, keyval)
		}
	}
	fields = append(fields, cityNames)
	fields = append(fields, strconv.FormatUint(uint64(g.Location.Latitude), 10))
	fields = append(fields, strconv.FormatUint(uint64(g.Location.Longitude), 10))
	return append(fields, g.Location.TimeZone)
}

func (a *AnalyticsRecord) GetLineValues() []string {
	fields := []string{}
	fields = append(fields, a.Method, a.Host, a.Path, a.RawPath)
	fields = append(fields, strconv.FormatUint(uint64(a.ContentLength), 10))
	fields = append(fields, a.UserAgent)
	fields = append(fields, strconv.FormatUint(uint64(a.Day), 10))
	fields = append(fields, a.Month.String())
	fields = append(fields, strconv.FormatUint(uint64(a.Year), 10))
	fields = append(fields, strconv.FormatUint(uint64(a.Hour), 10))
	fields = append(fields, strconv.FormatUint(uint64(a.ResponseCode), 10))
	fields = append(fields, a.APIKey)
	fields = append(fields, a.TimeStamp.String())
	fields = append(fields, a.APIVersion, a.APIName, a.APIID, a.OrgID, a.OauthID)
	fields = append(fields, strconv.FormatUint(uint64(a.RequestTime), 10))
	fields = append(fields, a.RawRequest, a.RawResponse, a.IPAddress)
	fields = append(fields, a.Geo.GetLineValues()...)
	fields = append(fields, a.Network.GetLineValues()...)
	fields = append(fields, a.Latency.GetLineValues()...)
	fields = append(fields, strings.Join(a.Tags[:], ";"))
	fields = append(fields, a.Alias)
	fields = append(fields, strconv.FormatBool(a.TrackPath))
	fields = append(fields, a.ExpireAt.String())
	fields = append(fields, a.ApiSchema)

	return fields
}

func (a *AnalyticsRecord) TrimRawData(size int) {
	// trim RawResponse
	a.RawResponse = trimString(size, a.RawResponse)

	// trim RawRequest
	a.RawRequest = trimString(size, a.RawRequest)
}

func (n *NetworkStats) Flush() NetworkStats {
	s := NetworkStats{
		OpenConnections:  atomic.LoadInt64(&n.OpenConnections),
		ClosedConnection: atomic.LoadInt64(&n.ClosedConnection),
		BytesIn:          atomic.LoadInt64(&n.BytesIn),
		BytesOut:         atomic.LoadInt64(&n.BytesOut),
	}
	atomic.StoreInt64(&n.OpenConnections, 0)
	atomic.StoreInt64(&n.ClosedConnection, 0)
	atomic.StoreInt64(&n.BytesIn, 0)
	atomic.StoreInt64(&n.BytesOut, 0)
	return s
}

func (a *AnalyticsRecord) SetExpiry(expiresInSeconds int64) {
	expiry := time.Duration(expiresInSeconds) * time.Second
	if expiresInSeconds == 0 {
		// Expiry is set to 100 years
		expiry = (24 * time.Hour) * (365 * 100)
	}

	t := time.Now()
	t2 := t.Add(expiry)
	a.ExpireAt = t2
}

func trimString(size int, value string) string {
	trimBuffer := bytes.Buffer{}
	defer trimBuffer.Reset()

	trimBuffer.Write([]byte(value))
	if trimBuffer.Len() < size {
		size = trimBuffer.Len()
	}
	trimBuffer.Truncate(size)

	return string(trimBuffer.Bytes())
}

// TimestampToProto will process timestamps and assign them to the proto record
// protobuf converts all timestamps to UTC so we need to ensure that we keep
// the same original location, in order to do so, we store the location
func (a *AnalyticsRecord) TimestampToProto(newRecord *analyticsproto.AnalyticsRecord) {
	// save original location
	newRecord.TimeStamp = timestamppb.New(a.TimeStamp)
	newRecord.ExpireAt = timestamppb.New(a.ExpireAt)
	newRecord.TimeZone = a.TimeStamp.Location().String()
}

func (a *AnalyticsRecord) TimeStampFromProto(protoRecord analyticsproto.AnalyticsRecord) {
	// get timestamp in original location
	loc, err := time.LoadLocation(protoRecord.TimeZone)
	if err != nil {
		log.Error(err)
		return
	}

	// assign timestamp in original location
	a.TimeStamp = protoRecord.TimeStamp.AsTime().In(loc)
	a.ExpireAt = protoRecord.ExpireAt.AsTime().In(loc)
}

func (a *AnalyticsRecord) GetGeo(ipStr string, GeoIPDB *maxminddb.Reader) {
	// Not great, tightly coupled
	if GeoIPDB == nil {
		return
	}

	geo, err := GeoIPLookup(ipStr, GeoIPDB)
	if err != nil {
		log.Error("GeoIP Failure (not recorded): ", err)
		return
	}
	if geo == nil {
		return
	}

	log.Debug("ISO Code: ", geo.Country.ISOCode)
	log.Debug("City: ", geo.City.Names["en"])
	log.Debug("Lat: ", geo.Location.Latitude)
	log.Debug("Lon: ", geo.Location.Longitude)
	log.Debug("TZ: ", geo.Location.TimeZone)

	a.Geo.Location = geo.Location
	a.Geo.Country = geo.Country
	a.Geo.City = geo.City
}

func GeoIPLookup(ipStr string, GeoIPDB *maxminddb.Reader) (*GeoData, error) {
	if ipStr == "" {
		return nil, nil
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address %q", ipStr)
	}
	record := new(GeoData)
	if err := GeoIPDB.Lookup(ip, record); err != nil {
		return nil, fmt.Errorf("geoIPDB lookup of %q failed: %v", ipStr, err)
	}
	return record, nil
}

func (a *AnalyticsRecord) IsGraphRecord() bool {
	if len(a.Tags) == 0 {
		return false
	}

	for _, tag := range a.Tags {
		if tag == PredefinedTagGraphAnalytics {
			return true
		}
	}

	return false
}

func (a *AnalyticsRecord) RemoveIgnoredFields(ignoreFields []string) {
	for _, fieldToIgnore := range ignoreFields {
		found := false
		for _, field := range structs.Fields(a) {
			fieldTag := field.Tag("json")
			if fieldTag == fieldToIgnore {
				// setting field to default value
				err := field.Zero()
				if err != nil {
					log.Error("Unable to ignore "+field.Name()+" field: ", err)
				}
				found = true
				continue
			}
		}
		if !found {
			log.Error("Error looking for field + ", fieldToIgnore+" in AnalyticsRecord struct: not found.")
		}
	}
}
