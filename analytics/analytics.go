package analytics

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/TykTechnologies/graphql-go-tools/pkg/ast"
	"github.com/TykTechnologies/graphql-go-tools/pkg/astnormalization"
	"github.com/TykTechnologies/graphql-go-tools/pkg/astparser"
	gql "github.com/TykTechnologies/graphql-go-tools/pkg/graphql"
	"github.com/TykTechnologies/graphql-go-tools/pkg/operationreport"
	"github.com/buger/jsonparser"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/oschwald/maxminddb-golang"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsproto "github.com/TykTechnologies/tyk-pump/analytics/proto"

	"github.com/TykTechnologies/tyk-pump/logger"
)

const (
	PredefinedTagGraphAnalytics = "tyk-graph-analytics"
)

const (
	operationTypeQuery        = "query"
	operationTypeMutation     = "mutation"
	operationTypeSubscription = "subscription"
	operationTypeUnknown      = "unknown"
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
	Method        string       `json:"method" gorm:"column:method"`
	Host          string       `json:"host" gorm:"column:host"`
	Path          string       `json:"path" gorm:"column:path"`
	RawPath       string       `json:"raw_path" gorm:"column:rawpath"`
	ContentLength int64        `json:"content_length" gorm:"column:contentlength"`
	UserAgent     string       `json:"user_agent" gorm:"column:useragent"`
	Day           int          `json:"day" sql:"-"`
	Month         time.Month   `json:"month" sql:"-"`
	Year          int          `json:"year" sql:"-"`
	Hour          int          `json:"hour" sql:"-"`
	ResponseCode  int          `json:"response_code" gorm:"column:responsecode;index"`
	APIKey        string       `json:"api_key" gorm:"column:apikey;index"`
	TimeStamp     time.Time    `json:"timestamp" gorm:"column:timestamp;index"`
	APIVersion    string       `json:"api_version" gorm:"column:apiversion"`
	APIName       string       `json:"api_name" sql:"-"`
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
	ApiSchema     string       `json:"api_schema" bson:"-" gorm:"-"`
}

func (ar *AnalyticsRecord) TableName() string {
	return SQLTable
}

type GraphRecord struct {
	AnalyticsRecord

	Types         map[string][]string
	HasErrors     bool
	Errors        []graphError
	OperationType string
	Variables     string
}

type graphError struct {
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

func (a *AnalyticsRecord) ToGraphRecord() (GraphRecord, error) {
	record := GraphRecord{
		AnalyticsRecord: *a,
	}
	if a.ResponseCode >= 400 {
		record.HasErrors = true
	}
	rawRequest, err := base64.StdEncoding.DecodeString(a.RawRequest)
	if err != nil {
		return record, fmt.Errorf("error decoding raw request: %w", err)
	}

	schemaBody, err := base64.StdEncoding.DecodeString(a.ApiSchema)
	if err != nil {
		return record, fmt.Errorf("error decoding schema: %w", err)
	}

	request, schema, operationName, err := generateNormalizedDocuments(rawRequest, schemaBody)
	if err != nil {
		return record, err
	}
	record.Variables = base64.StdEncoding.EncodeToString(request.Input.Variables)

	// get the operation ref
	operationRef := -1
	if operationName != "" {
		for i := range request.OperationDefinitions {
			if request.OperationDefinitionNameString(i) == operationName {
				operationRef = i
				break
			}
		}
	}
	if len(request.OperationDefinitions) < 1 {
		log.Warn("no operations found")
		return record, err
	}
	operationRef = 0

	// get operation type
	switch request.OperationDefinitions[operationRef].OperationType {
	case ast.OperationTypeMutation:
		record.OperationType = operationTypeMutation
	case ast.OperationTypeSubscription:
		record.OperationType = operationTypeSubscription
	case ast.OperationTypeQuery:
		record.OperationType = operationTypeQuery
	case ast.OperationTypeUnknown:
		fallthrough
	default:
		record.OperationType = operationTypeUnknown
	}

	// get the selection set types to start with
	fieldTypeList, err := extractTypesOfSelectionSet(operationRef, request, schema)
	if err != nil {
		log.WithError(err).Error("error extracting selection set types")
		return record, err
	}
	typesToFieldsMap := make(map[string][]string)
	for fieldRef, typeDefRef := range fieldTypeList {
		recursivelyExtractTypesAndFields(fieldRef, typeDefRef, typesToFieldsMap, request, schema)
	}
	record.Types = typesToFieldsMap

	// get errors
	responseDecoded, err := base64.StdEncoding.DecodeString(a.RawResponse)
	if err != nil {
		return record, nil
	}
	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(responseDecoded)), nil)
	defer resp.Body.Close()
	if err != nil {
		log.WithError(err).Error("error reading raw response")
		return record, err
	}

	dat, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithError(err).Error("error reading response body")
		return record, err
	}
	errBytes, t, _, err := jsonparser.Get(dat, "errors")
	if err != nil && err != jsonparser.KeyPathNotFoundError {
		log.WithError(err).Error("error getting response errors")
		return record, err
	}
	if t != jsonparser.NotExist {
		if err := json.Unmarshal(errBytes, &record.Errors); err != nil {
			log.WithError(err).Error("error parsing graph errors")
			return record, err
		}
		record.HasErrors = true
	}

	return record, nil
}

// extractTypesOfSelectionSet extracts all type names of the selection sets in the operation
// it returns a map of the FieldRef in the req to the type Definition in the schema
func extractTypesOfSelectionSet(operationRef int, req, schema *ast.Document) (map[int]int, error) {
	fieldTypeMap := make(map[int]int)
	operationDef := req.OperationDefinitions[operationRef]
	if !operationDef.HasSelections {
		return nil, nil
	}

	for _, selRef := range req.SelectionSets[operationDef.SelectionSet].SelectionRefs {
		sel := req.Selections[selRef]
		if sel.Kind != ast.SelectionKindField {
			continue
		}
		// get selection field def
		selFieldDefRef, err := getOperationSelectionFieldDefinition(operationDef.OperationType, req.FieldNameString(sel.Ref), schema)
		if selFieldDefRef == -1 || err != nil {
			if err != nil {
				log.WithError(err).Error("error getting operation field definition")
			}
			return nil, errors.New("error getting selection set")
		}

		typeRef := schema.ResolveUnderlyingType(schema.FieldDefinitions[selFieldDefRef].Type)
		if schema.TypeIsScalar(typeRef, schema) || schema.TypeIsEnum(typeRef, schema) {
			continue
		}
		fieldTypeMap[sel.Ref] = getObjectTypeRefWithName(schema.TypeNameString(typeRef), schema)
	}
	return fieldTypeMap, nil
}

func recursivelyExtractTypesAndFields(fieldRef, typeDef int, resp map[string][]string, req, schema *ast.Document) {
	field := req.Fields[fieldRef]
	fieldListForType := make([]string, 0)

	if !field.HasSelections {
		return
	}
	for _, selRef := range req.SelectionSets[field.SelectionSet].SelectionRefs {
		sel := req.Selections[selRef]
		if sel.Kind != ast.SelectionKindField {
			continue
		}
		fieldListForType = append(fieldListForType, req.FieldNameString(sel.Ref))

		// get the field definition and run this function on it
		fieldDefRef := getObjectFieldRefWithName(req.FieldNameString(sel.Ref), typeDef, schema)
		if fieldDefRef == -1 {
			continue
		}

		fieldDefType := schema.ResolveUnderlyingType(schema.FieldDefinitions[fieldDefRef].Type)
		if schema.TypeIsScalar(fieldDefType, schema) || schema.TypeIsEnum(fieldDefType, schema) {
			continue
		}

		objTypeRef := getObjectTypeRefWithName(schema.TypeNameString(fieldDefType), schema)
		if objTypeRef == -1 {
			continue
		}

		recursivelyExtractTypesAndFields(sel.Ref, objTypeRef, resp, req, schema)

	}

	objectTypeName := schema.ObjectTypeDefinitionNameString(typeDef)
	_, ok := resp[objectTypeName]
	if ok {
		resp[objectTypeName] = append(resp[objectTypeName], fieldListForType...)
	} else {
		resp[objectTypeName] = fieldListForType
	}

	resp[objectTypeName] = fieldListForType
	return
}

func getObjectFieldRefWithName(name string, objTypeRef int, schema *ast.Document) int {
	objectTypeDefinition := schema.ObjectTypeDefinitions[objTypeRef]
	if !objectTypeDefinition.HasFieldDefinitions {
		return -1
	}
	for _, r := range objectTypeDefinition.FieldsDefinition.Refs {
		if schema.FieldDefinitionNameString(r) == name {
			return r
		}
	}
	return -1
}

func getObjectTypeRefWithName(name string, schema *ast.Document) int {
	n, ok := schema.Index.FirstNodeByNameStr(name)
	if !ok {
		return -1
	}
	if n.Kind != ast.NodeKindObjectTypeDefinition {
		return -1
	}
	return n.Ref
}

func generateNormalizedDocuments(requestRaw, schemaRaw []byte) (r, s *ast.Document, operationName string, err error) {
	httpRequest, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(requestRaw)))
	if err != nil {
		log.WithError(err).Error("error parsing request")
		return
	}
	var gqlRequest gql.Request
	err = gql.UnmarshalRequest(httpRequest.Body, &gqlRequest)
	if err != nil {
		log.WithError(err).Error("error unmarshalling request")
		return
	}
	operationName = gqlRequest.OperationName

	schema, err := gql.NewSchemaFromString(string(schemaRaw))
	if err != nil {
		return
	}
	schemaDoc, operationReport := astparser.ParseGraphqlDocumentBytes(schema.Document())
	if operationReport.HasErrors() {
		err = operationReport
		return
	}
	s = &schemaDoc

	requestDoc, operationReport := astparser.ParseGraphqlDocumentString(gqlRequest.Query)
	if operationReport.HasErrors() {
		err = operationReport
		log.WithError(err).Error("error parsing request document")
		return
	}
	r = &requestDoc
	r.Input.Variables = gqlRequest.Variables
	normalizer := astnormalization.NewWithOpts(
		astnormalization.WithRemoveFragmentDefinitions(),
	)

	var report operationreport.Report
	if operationName != "" {
		normalizer.NormalizeNamedOperation(r, s, []byte(operationName), &report)
	} else {
		normalizer.NormalizeOperation(r, s, &report)
	}
	if report.HasErrors() {
		log.WithError(report).Error("error normalizing")
		err = report
		return
	}
	return
}

// getOperationSelectionFieldDefinition gets the schema's field definition ref for the selection set of the operation type in question
func getOperationSelectionFieldDefinition(operationType ast.OperationType, opSelectionName string, schema *ast.Document) (int, error) {
	var (
		node  ast.Node
		found bool
	)
	switch operationType {
	case ast.OperationTypeQuery:
		node, found = schema.Index.FirstNodeByNameBytes(schema.Index.QueryTypeName)
		if !found {
			return -1, fmt.Errorf("missing query type declaration")
		}
	case ast.OperationTypeMutation:
		node, found = schema.Index.FirstNodeByNameBytes(schema.Index.MutationTypeName)
		if !found {
			return -1, fmt.Errorf("missing mutation type declaration")
		}
	case ast.OperationTypeSubscription:
		node, found = schema.Index.FirstNodeByNameBytes(schema.Index.SubscriptionTypeName)
		if !found {
			return -1, fmt.Errorf("missing subscription type declaration")
		}
	default:
		return -1, fmt.Errorf("unknown operation")
	}
	if node.Kind != ast.NodeKindObjectTypeDefinition {
		return -1, fmt.Errorf("invalid node type")
	}

	operationObjDefinition := schema.ObjectTypeDefinitions[node.Ref]
	if !operationObjDefinition.HasFieldDefinitions {
		return -1, nil
	}

	for _, fieldRef := range operationObjDefinition.FieldsDefinition.Refs {
		if opSelectionName == schema.FieldDefinitionNameString(fieldRef) {
			return fieldRef, nil
		}
	}

	// TODO get name using selection index and all
	return -1, fmt.Errorf("field not found")
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
