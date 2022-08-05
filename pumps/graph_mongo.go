package pumps

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"github.com/TykTechnologies/graphql-go-tools/pkg/ast"
	"github.com/TykTechnologies/graphql-go-tools/pkg/astnormalization"
	"github.com/TykTechnologies/graphql-go-tools/pkg/astparser"
	gql "github.com/TykTechnologies/graphql-go-tools/pkg/graphql"
	"github.com/TykTechnologies/graphql-go-tools/pkg/operationreport"
	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/mitchellh/mapstructure"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"net/http"
	"time"
)

const (
	graphMongoPrefix = "graph-mongo-pump"
	dialTimeout      = time.Second * 15
	maxConnectRetry  = 3
)

type GraphRecord struct {
	ApiID        string
	APIName      string
	Payload      []byte // encoded encrypted raw query
	Types        map[string][]string
	Variables    []byte // encoded/encrypted variables
	Response     string // encoded/encrypted response
	ResponseCode int
	HasErrors    bool
	Day          int
	Month        time.Month
	Year         int
	Hour         int
	OrgID        string
	OauthID      string
	RequestTime  int64
	TimeStamp    time.Time
	Errors       []GraphError
}

type SubgraphRecord struct {
	Name        string
	RequestTime int64
}

type DataSourceRecord struct {
	Name        string
	RequestTime int64
}

type GraphError struct {
	message string
	path    []string
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
	}
	request, schema, operationName, err := g.generateNormalizedDocuments(rawRequest, schemaBody)
	if err != nil {
		return record, err
	}
	fmt.Println(request, schema, operationName)

	return record, nil
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
