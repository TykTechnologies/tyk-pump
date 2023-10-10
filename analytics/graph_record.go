package analytics

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/buger/jsonparser"

	"github.com/TykTechnologies/graphql-go-tools/pkg/ast"
	"github.com/TykTechnologies/graphql-go-tools/pkg/astnormalization"
	"github.com/TykTechnologies/graphql-go-tools/pkg/astparser"
	gql "github.com/TykTechnologies/graphql-go-tools/pkg/graphql"
	"github.com/TykTechnologies/graphql-go-tools/pkg/operationreport"
	"github.com/TykTechnologies/storage/persistent/model"
)

// GraphSQLTableName should be defined before SQL migration is called on the GraphRecord
// the reason this approach is used to define the table name is due to gorm's inability to
// read values from the fields of the GraphRecord/AnalyticsRecord struct when it is migrating, due to that
// a single static value is going to be returned as TableName and it will be used as the prefix for index/relationship creation no matter the
// value passed to db.Table()
var GraphSQLTableName string

type GraphRecord struct {
	Types map[string][]string `gorm:"types"`

	AnalyticsRecord AnalyticsRecord `bson:",inline" gorm:"embedded;embeddedPrefix:analytics_"`

	OperationType string       `gorm:"column:operation_type"`
	Variables     string       `gorm:"variables"`
	RootFields    []string     `gorm:"root_fields"`
	Errors        []GraphError `gorm:"errors"`
	HasErrors     bool         `gorm:"has_errors"`
}

// TableName is used by both the sql orm and mongo driver the table name and collection name used for operations on this model
// the conditional return is to ensure the right value is used for both the sql and mongo operations
func (g *GraphRecord) TableName() string {
	if GraphSQLTableName == "" {
		return g.AnalyticsRecord.TableName()
	}
	return GraphSQLTableName
}

// GetObjectID is a dummy function to satisfy the interface
func (*GraphRecord) GetObjectID() model.ObjectID {
	return ""
}

// SetObjectID is a dummy function to satisfy the interface
func (*GraphRecord) SetObjectID(model.ObjectID) {
	// empty
}

// parseRequest reads the raw encoded request and schema, extracting the type information
// operation information and root field operations
// if an error is encountered it simply breaks the operation regardless of how far along it is.
func (g *GraphRecord) parseRequest(encodedRequest, encodedSchema string) {
	if encodedRequest == "" || encodedSchema == "" {
		log.Warn("empty request/schema")
		return
	}
	rawRequest, err := base64.StdEncoding.DecodeString(encodedRequest)
	if err != nil {
		log.WithError(err).Error("error decoding raw request")
		return
	}

	schemaBody, err := base64.StdEncoding.DecodeString(encodedSchema)
	if err != nil {
		log.WithError(err).Error("error decoding schema")
		return
	}

	request, schema, operationName, err := generateNormalizedDocuments(rawRequest, schemaBody)
	if err != nil {
		log.WithError(err).Error("error generating document")
		return
	}

	if len(request.Input.Variables) != 0 && string(request.Input.Variables) != "null" {
		g.Variables = base64.StdEncoding.EncodeToString(request.Input.Variables)
	}

	// get the operation ref
	operationRef := 0
	if operationName != "" {
		for i := range request.OperationDefinitions {
			if request.OperationDefinitionNameString(i) == operationName {
				operationRef = i
				break
			}
		}
	} else if len(request.OperationDefinitions) > 1 {
		log.Warn("no operation name specified")
		return
	}

	// get operation type
	switch request.OperationDefinitions[operationRef].OperationType {
	case ast.OperationTypeMutation:
		g.OperationType = string(ast.DefaultMutationTypeName)
	case ast.OperationTypeSubscription:
		g.OperationType = string(ast.DefaultSubscriptionTypeName)
	case ast.OperationTypeQuery:
		g.OperationType = string(ast.DefaultQueryTypeName)
	}

	// get the selection set types to start with
	fieldTypeList, err := extractOperationSelectionSetTypes(operationRef, &g.RootFields, request, schema)
	if err != nil {
		log.WithError(err).Error("error extracting selection set types")
		return
	}
	typesToFieldsMap := make(map[string][]string)
	for fieldRef, typeDefRef := range fieldTypeList {
		if typeDefRef == ast.InvalidRef {
			err = errors.New("invalid selection set field type")
			log.Warn("invalid type found")
			continue
		}
		extractTypesAndFields(fieldRef, typeDefRef, typesToFieldsMap, request, schema)
	}
	g.Types = typesToFieldsMap
}

// parseResponse looks through the encoded response string and parses information like
// the errors
func (g *GraphRecord) parseResponse(encodedResponse string) {
	if encodedResponse == "" {
		log.Warn("empty response body")
		return
	}

	responseDecoded, err := base64.StdEncoding.DecodeString(encodedResponse)
	if err != nil {
		log.WithError(err).Error("error decoding response")
		return
	}
	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(responseDecoded)), nil)
	if err != nil {
		log.WithError(err).Error("error reading raw response")
		return
	}
	defer resp.Body.Close()

	dat, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.WithError(err).Error("error reading response body")
		return
	}
	errBytes, t, _, err := jsonparser.Get(dat, "errors")
	// check if the errors key exists in the response
	if err != nil && err != jsonparser.KeyPathNotFoundError {
		// we got an unexpected error parsing te response
		log.WithError(err).Error("error getting response errors")
		return
	}
	if t != jsonparser.NotExist {
		// errors key exists so unmarshal it
		if err := json.Unmarshal(errBytes, &g.Errors); err != nil {
			log.WithError(err).Error("error parsing graph errors")
			return
		}
		g.HasErrors = true
	}
}

func (a *AnalyticsRecord) ToGraphRecord() GraphRecord {
	record := GraphRecord{
		AnalyticsRecord: *a,
		RootFields:      make([]string, 0),
		Types:           make(map[string][]string),
		Errors:          make([]GraphError, 0),
	}
	if a.ResponseCode >= 400 {
		record.HasErrors = true
	}

	record.parseRequest(a.RawRequest, a.ApiSchema)

	record.parseResponse(a.RawResponse)

	return record
}

// extractOperationSelectionSetTypes extracts all type names of the selection sets in the operation
// it returns a map of the FieldRef in the req to the type Definition in the schema
func extractOperationSelectionSetTypes(operationRef int, rootFields *[]string, req, schema *ast.Document) (map[int]int, error) {
	fieldTypeMap := make(map[int]int)
	operationDef := req.OperationDefinitions[operationRef]
	if !operationDef.HasSelections {
		return nil, errors.New("operation has no selection set")
	}

	for _, selRef := range req.SelectionSets[operationDef.SelectionSet].SelectionRefs {
		sel := req.Selections[selRef]
		if sel.Kind != ast.SelectionKindField {
			continue
		}
		// get selection field def
		selFieldDefRef, err := getOperationSelectionFieldDefinition(operationDef.OperationType, req.FieldNameString(sel.Ref), schema)
		if selFieldDefRef == ast.InvalidRef || err != nil {
			if err != nil {
				log.WithError(err).Error("error getting operation field definition")
			}
			return nil, errors.New("error getting selection set")
		}

		*rootFields = append(*rootFields, req.FieldNameString(sel.Ref))

		typeRef := schema.ResolveUnderlyingType(schema.FieldDefinitions[selFieldDefRef].Type)
		if schema.TypeIsScalar(typeRef, schema) || schema.TypeIsEnum(typeRef, schema) {
			continue
		}
		fieldTypeMap[sel.Ref] = getObjectTypeRefWithName(schema.TypeNameString(typeRef), schema)
	}
	return fieldTypeMap, nil
}

// extractTypesAndFields extracts all types and type fields used in this request
func extractTypesAndFields(fieldRef, typeDef int, resp map[string][]string, req, schema *ast.Document) {
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
		if fieldDefRef == ast.InvalidRef {
			continue
		}

		fieldDefType := schema.ResolveUnderlyingType(schema.FieldDefinitions[fieldDefRef].Type)
		if schema.TypeIsScalar(fieldDefType, schema) || schema.TypeIsEnum(fieldDefType, schema) {
			continue
		}

		objTypeRef := getObjectTypeRefWithName(schema.TypeNameString(fieldDefType), schema)
		if objTypeRef == ast.InvalidRef {
			continue
		}

		extractTypesAndFields(sel.Ref, objTypeRef, resp, req, schema)
	}

	objectTypeName := schema.ObjectTypeDefinitionNameString(typeDef)
	_, ok := resp[objectTypeName]
	if ok {
		resp[objectTypeName] = append(resp[objectTypeName], fieldListForType...)
	} else {
		resp[objectTypeName] = fieldListForType
	}

	resp[objectTypeName] = fieldListForType
}

// getObjectFieldRefWithName gets the object field reference from the object type using the name from the schame
func getObjectFieldRefWithName(name string, objTypeRef int, schema *ast.Document) int {
	objectTypeDefinition := schema.ObjectTypeDefinitions[objTypeRef]
	if !objectTypeDefinition.HasFieldDefinitions {
		return ast.InvalidRef
	}
	for _, r := range objectTypeDefinition.FieldsDefinition.Refs {
		if schema.FieldDefinitionNameString(r) == name {
			return r
		}
	}
	return ast.InvalidRef
}

// getObjectTypeRefWithName gets the ref of the type from the schema using the name
func getObjectTypeRefWithName(name string, schema *ast.Document) int {
	n, ok := schema.Index.FirstNodeByNameStr(name)
	if !ok {
		return ast.InvalidRef
	}
	if n.Kind != ast.NodeKindObjectTypeDefinition {
		return ast.InvalidRef
	}
	return n.Ref
}

// generateNormalizedDocuments generates and normalizes the ast documents from the raw request and the raw schema
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
			return ast.InvalidRef, fmt.Errorf("missing query type declaration")
		}
	case ast.OperationTypeMutation:
		node, found = schema.Index.FirstNodeByNameBytes(schema.Index.MutationTypeName)
		if !found {
			return ast.InvalidRef, fmt.Errorf("missing mutation type declaration")
		}
	case ast.OperationTypeSubscription:
		node, found = schema.Index.FirstNodeByNameBytes(schema.Index.SubscriptionTypeName)
		if !found {
			return ast.InvalidRef, fmt.Errorf("missing subscription type declaration")
		}
	default:
		return ast.InvalidRef, fmt.Errorf("unknown operation")
	}
	if node.Kind != ast.NodeKindObjectTypeDefinition {
		return ast.InvalidRef, fmt.Errorf("invalid node type")
	}

	operationObjDefinition := schema.ObjectTypeDefinitions[node.Ref]
	if !operationObjDefinition.HasFieldDefinitions {
		return ast.InvalidRef, nil
	}

	for _, fieldRef := range operationObjDefinition.FieldsDefinition.Refs {
		if opSelectionName == schema.FieldDefinitionNameString(fieldRef) {
			return fieldRef, nil
		}
	}

	return ast.InvalidRef, fmt.Errorf("field not found")
}
