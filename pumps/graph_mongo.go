package pumps

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/TykTechnologies/graphql-go-tools/pkg/ast"
	"github.com/TykTechnologies/graphql-go-tools/pkg/astnormalization"
	"github.com/TykTechnologies/graphql-go-tools/pkg/astparser"
	gql "github.com/TykTechnologies/graphql-go-tools/pkg/graphql"
	"github.com/TykTechnologies/graphql-go-tools/pkg/operationreport"
	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/buger/jsonparser"
	"github.com/mitchellh/mapstructure"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"io"
	"net/http"
	"time"
)

const (
	graphMongoPrefix = "graph-mongo-pump"
	dialTimeout      = time.Second * 15
	maxConnectRetry  = 3

	operationTypeQuery        = "query"
	operationTypeMutation     = "mutation"
	operationTypeSubscription = "subscription"
)

type GraphRecord struct {
	ApiID         string
	APIName       string
	Payload       string // encoded encrypted raw query
	Types         map[string][]string
	Variables     string // encoded/encrypted variables
	Response      string // encoded/encrypted response
	ResponseCode  int
	HasErrors     bool
	Day           int
	Month         time.Month
	Year          int
	Hour          int
	OrgID         string
	OauthID       string
	RequestTime   int64
	TimeStamp     time.Time
	Errors        []graphError
	OperationType string
}

type SubgraphRecord struct {
	Name        string
	RequestTime int64
}

type DataSourceRecord struct {
	Name        string
	RequestTime int64
}

type graphError struct {
	Message  string          `json:"message"`
	Path     []interface{}   `json:"path"`
	Location []locationError `json:"locations"`
}

type locationError struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type collectionStore interface {
	Insert(interface{}) error
}

type GraphMongoPump struct {
	dbConf *MongoConf
	log    *logrus.Entry

	collection collectionStore
	client     *mongo.Client
}

func (g *GraphMongoPump) GetName() string {
	return "Graph MongoDB pump"
}
func (g *GraphMongoPump) New() Pump {
	return &GraphMongoPump{}
}

func (g GraphMongoPump) Init(config interface{}) error {
	g.dbConf = &MongoConf{}
	g.log = log.WithField("prefix", graphMongoPrefix)
	if err := mapstructure.Decode(config, &g.dbConf); err != nil {
		return err
	}
	if err := mapstructure.Decode(config, &g.dbConf.BaseMongoConf); err != nil {
		return err
	}
	log.WithFields(logrus.Fields{
		"url":             g.dbConf.GetBlurredURL(),
		"collection_name": g.dbConf.CollectionName,
	}).Info("Init")

	if err := g.connect(); err != nil {
		log.WithError(err).Error("error connecting to mongo")
		return err
	}
	return nil
}

func (g *GraphMongoPump) connect() error {
	var (
		err   error
		tries int
	)
	for tries < maxConnectRetry {
		g.client, err = mongo.Connect(context.Background(), options.Client().ApplyURI(g.dbConf.MongoURL).SetConnectTimeout(dialTimeout))
		if err == nil {
			break
		}
		log.WithError(err).Error("error connecting to mongo db, retrying...")
		tries++
	}
	if err != nil {
		return err
	}
	return nil
}

func (g GraphMongoPump) WriteData(ctx context.Context, data []interface{}) error {
	return nil
}

func (g GraphMongoPump) recordToGraphRecord(analyticsRecord analytics.AnalyticsRecord) (GraphRecord, error) {
	rawRequest, err := base64.StdEncoding.DecodeString(analyticsRecord.RawRequest)
	if err != nil {
		return GraphRecord{}, err
	}

	encodedSchema, ok := analyticsRecord.Metadata["graphql-schema"]
	if !ok || encodedSchema == "" {
		return GraphRecord{}, fmt.Errorf("schema not passed along with analytics record")
	}
	schemaBody, err := base64.StdEncoding.DecodeString(encodedSchema)
	if err != nil {
		return GraphRecord{}, err
	}

	// TODO variables
	record := GraphRecord{
		ApiID:        analyticsRecord.APIID,
		APIName:      analyticsRecord.APIName,
		Response:     analyticsRecord.RawResponse,
		ResponseCode: analyticsRecord.ResponseCode,
		Day:          analyticsRecord.Day,
		Month:        analyticsRecord.Month,
		Year:         analyticsRecord.Year,
		Hour:         analyticsRecord.Hour,
		RequestTime:  analyticsRecord.RequestTime,
		TimeStamp:    analyticsRecord.TimeStamp,
		Errors:       make([]graphError, 0),
		Types:        make(map[string][]string),
	}
	if analyticsRecord.ResponseCode >= 400 {
		record.HasErrors = true
	}
	request, schema, operationName, err := g.generateNormalizedDocuments(rawRequest, schemaBody)
	if err != nil {
		return record, err
	}

	record.Payload = base64.StdEncoding.EncodeToString(request.Input.RawBytes)
	record.Variables = base64.StdEncoding.EncodeToString(request.Input.Variables)
	//if operationName is empty use first operation
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
		err := fmt.Errorf("no operations founc")
		g.log.Error(err)
		return record, err
	}
	operationRef = 0
	operation := request.OperationDefinitions[operationRef]
	switch operation.OperationType {
	case ast.OperationTypeQuery:
		record.OperationType = operationTypeQuery
	case ast.OperationTypeMutation:
		record.OperationType = operationTypeMutation
	case ast.OperationTypeSubscription:
		record.OperationType = operationTypeSubscription
	default:
		g.log.Warn("unknown operation type")
	}

	// get types from selection set
	if !operation.HasSelections {
		return record, nil
	}

	selectionSet := request.SelectionSets[operation.SelectionSet]
	typeMap := make(map[string][]string)
	for _, selRef := range selectionSet.SelectionRefs {
		selection := request.Selections[selRef]
		if selection.Kind != ast.SelectionKindField {
			continue
		}
		baseFieldDefinition, err := g.getOperationFieldDefinition(operation.OperationType, request.FieldNameString(selection.Ref), schema)
		if err != nil {
			g.log.WithError(err).Error("error getting operation field definition")
			return record, err
		}
		g.extractTypesAndFields(selection.Ref, baseFieldDefinition, typeMap, request, schema)
	}
	record.Types = typeMap

	// get errors
	responseDecoded, err := base64.StdEncoding.DecodeString(analyticsRecord.RawResponse)
	if err != nil {
		return record, nil
	}
	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(responseDecoded)), nil)
	defer resp.Body.Close()
	if err != nil {
		g.log.WithError(err).Error("error reading raw response")
		return record, err
	}

	dat, err := io.ReadAll(resp.Body)
	if err != nil {
		g.log.WithError(err).Error("error reading response body")
		return record, err
	}
	errBytes, t, _, err := jsonparser.Get(dat, "errors")
	if err != nil && err != jsonparser.KeyPathNotFoundError {
		g.log.WithError(err).Error("error getting response errors")
		return record, err
	}
	if t != jsonparser.NotExist {
		if err := json.Unmarshal(errBytes, &record.Errors); err != nil {
			g.log.WithError(err).Error("error parsing graph errors")
			return record, err
		}
		record.HasErrors = true

	}

	return record, nil
}

func (g GraphMongoPump) getOperationFieldDefinition(operationType ast.OperationType, fieldName string, schema *ast.Document) (int, error) {
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
		return -1, fmt.Errorf("unkown operation")
	}
	if node.Kind != ast.NodeKindObjectTypeDefinition {
		return -1, fmt.Errorf("invalid node type")
	}

	objDef := schema.ObjectTypeDefinitions[node.Ref]
	if !objDef.HasFieldDefinitions {
		return -1, nil
	}

	for _, fieldRef := range objDef.FieldsDefinition.Refs {
		if fieldName == schema.FieldDefinitionNameString(fieldRef) {
			return fieldRef, nil
		}
	}

	// TODO get name using selection index and all
	return -1, fmt.Errorf("field not found")
}

func (g GraphMongoPump) extractTypesAndFields(fieldRef, fieldDefinitionRef int, typeMap map[string][]string, request, schema *ast.Document) {
	field := request.Fields[fieldRef]
	if schema.TypeIsScalar(fieldDefinitionRef, schema) || schema.TypeIsEnum(fieldDefinitionRef, schema) {
		return
	}
	fieldType := schema.ResolveTypeNameString(schema.FieldDefinitionType(fieldDefinitionRef))
	typeMap[fieldType] = make([]string, 0)
	obj, found := schema.Index.FirstNodeByNameStr(fieldType)
	if !found {
		return
	}
	if obj.Kind != ast.NodeKindObjectTypeDefinition {
		return
	}
	objDefinition := schema.ObjectTypeDefinitions[obj.Ref]
	// if the field doesn't have selection set or the fieldDefinition has no children then skip
	if !field.HasSelections {
		return
	}
	if !objDefinition.HasFieldDefinitions {
		return
	}

	selectionSet := request.SelectionSets[field.SelectionSet]
selectionLoop:
	for _, selectionRef := range selectionSet.SelectionRefs {
		selection := request.Selections[selectionRef]
		if selection.Kind != ast.SelectionKindField {
			continue
		}
		selectionName := request.FieldNameString(selection.Ref)
		for _, fieldDefRef := range objDefinition.FieldsDefinition.Refs {
			fieldDefinitionName := schema.FieldDefinitionNameString(fieldDefRef)
			if fieldDefinitionName != selectionName {
				continue
			}
			typeMap[fieldType] = append(typeMap[fieldType], fieldDefinitionName)
			// TODO List later
			fieldDefinition := schema.FieldDefinitions[fieldDefRef]
			if schema.TypeIsScalar(fieldDefinition.Type, schema) || schema.TypeIsEnum(fieldDefinition.Type, schema) {
				continue selectionLoop
			}
			if request.Fields[selection.Ref].HasSelections {
				g.extractTypesAndFields(selection.Ref, fieldDefRef, typeMap, request, schema)
			}
		}

	}
}

func (g GraphMongoPump) generateNormalizedDocuments(requestRaw, schemaRaw []byte) (r, s *ast.Document, operationName string, err error) {
	httpRequest, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(requestRaw)))
	if err != nil {
		g.log.WithError(err).Error("error parsing request")
		return
	}
	var gqlRequest gql.Request
	err = gql.UnmarshalRequest(httpRequest.Body, &gqlRequest)
	if err != nil {
		g.log.WithError(err).Error("error unmarshalling request")
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
		g.log.WithError(err).Error("error parsing request document")
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
		g.log.WithError(report).Error("error normalizing")
		err = report
		return
	}
	return
}

//func (g GraphMongoPump) generateASTDocument(in []byte) (*ast.Document, error) {
//	doc, errReport := astparser.ParseGraphqlDocumentString(in)
//	if errReport.HasErrors() {
//		return nil, errReport
//	}
//}

func (g GraphMongoPump) SetFilters(analytics.AnalyticsFilters) {

}

func (g GraphMongoPump) GetFilters() analytics.AnalyticsFilters {
	return analytics.AnalyticsFilters{}
}

func (g GraphMongoPump) SetTimeout(timeout int) {

}

func (g GraphMongoPump) GetTimeout() int {
	return 0
}

func (g GraphMongoPump) SetOmitDetailedRecording(bool) {

}

func (g GraphMongoPump) GetOmitDetailedRecording() bool {
	return false
}

func (g GraphMongoPump) GetEnvPrefix() string {
	return ""
}

func (g GraphMongoPump) Shutdown() error {
	return nil
}

func (g GraphMongoPump) SetMaxRecordSize(size int) {

}
func (g GraphMongoPump) GetMaxRecordSize() int {
	return 0
}
