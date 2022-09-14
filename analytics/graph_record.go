package analytics

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/TykTechnologies/graphql-go-tools/pkg/ast"
	"github.com/TykTechnologies/graphql-go-tools/pkg/astnormalization"
	"github.com/TykTechnologies/graphql-go-tools/pkg/astparser"
	gql "github.com/TykTechnologies/graphql-go-tools/pkg/graphql"
	"github.com/TykTechnologies/graphql-go-tools/pkg/operationreport"
	"github.com/buger/jsonparser"
)

type GraphRecord struct {
	AnalyticsRecord `bson:",inline"`

	Errors        []graphError
	Types         map[string][]string
	OperationType string
	Variables     string
	HasErrors     bool
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
		return record, fmt.Errorf("error generating documents: %w", err)
	}
	if len(request.Input.Variables) != 0 && string(request.Input.Variables) != "null" {
		record.Variables = base64.StdEncoding.EncodeToString(request.Input.Variables)
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
	} else {
		if len(request.OperationDefinitions) > 1 {
			return record, errors.New("no operation name specified")
		}
	}

	// get operation type
	switch request.OperationDefinitions[operationRef].OperationType {
	case ast.OperationTypeMutation:
		record.OperationType = operationTypeMutation
	case ast.OperationTypeSubscription:
		record.OperationType = operationTypeSubscription
	case ast.OperationTypeQuery:
		record.OperationType = operationTypeQuery
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
	if err != nil {
		log.WithError(err).Error("error reading raw response")
		return record, err
	}
	defer resp.Body.Close()

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