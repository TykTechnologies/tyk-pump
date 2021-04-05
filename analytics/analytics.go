package analytics

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/TykTechnologies/tyk-pump/logger"
)

var log = logger.GetLogger()

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

func (n *NetworkStats) GetFieldNames() []string {
	return []string{
		"OpenConnections",
		"ClosedConnection",
		"BytesIn",
		"BytesOut",
	}
}

func (l *Latency) GetFieldNames() []string {
	return []string{
		"Total",
		"Upstream",
	}
}

func (g *GeoData) GetFieldNames() []string {
	return []string{
		"ISOCode",
		"GeoNameID",
		"Names",
		"Latitude",
		"Longitude",
		"TimeZone",
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
	return append(fields, "Tags", "Alias", "TrackPath", "ExpireAt")
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
	return fields
}
