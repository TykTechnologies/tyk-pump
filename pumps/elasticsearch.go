package pumps

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	elasticv3 "gopkg.in/olivere/elastic.v3"
	elasticv5 "gopkg.in/olivere/elastic.v5"
	elasticv6 "gopkg.in/olivere/elastic.v6"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/murmur3"
	"github.com/TykTechnologies/tyk-pump/analytics"
)

type ElasticsearchPump struct {
	operator ElasticsearchOperator
	esConf   *ElasticsearchConf
	CommonPumpConfig
}

const DEFAULT_BULK_SIZE = 1000

var elasticsearchPrefix = "elasticsearch-pump"
var elasticsearchDefaultENV = PUMPS_ENV_PREFIX + "_ELASTICSEARCH" + PUMPS_ENV_META_PREFIX

type ElasticsearchConf struct {
	EnvPrefix          string                  `mapstructure:"meta_env_prefix"`
	IndexName          string                  `mapstructure:"index_name"`
	ElasticsearchURL   string                  `mapstructure:"elasticsearch_url"`
	EnableSniffing     bool                    `mapstructure:"use_sniffing"`
	DocumentType       string                  `mapstructure:"document_type"`
	RollingIndex       bool                    `mapstructure:"rolling_index"`
	ExtendedStatistics bool                    `mapstructure:"extended_stats"`
	GenerateID         bool                    `mapstructure:"generate_id"`
	DecodeBase64       bool                    `mapstructure:"decode_base64"`
	Version            string                  `mapstructure:"version"`
	DisableBulk        bool                    `mapstructure:"disable_bulk"`
	BulkConfig         ElasticsearchBulkConfig `mapstructure:"bulk_config"`
	AuthAPIKeyID       string                  `mapstructure:"auth_api_key_id"`
	AuthAPIKey         string                  `mapstructure:"auth_api_key"`
	Username           string                  `mapstructure:"auth_basic_username"`
	Password           string                  `mapstructure:"auth_basic_password"`
}

type ElasticsearchBulkConfig struct {
	Workers       int `mapstructure:"workers"`
	FlushInterval int `mapstructure:"flush_interval"`
	BulkActions   int `mapstructure:"bulk_actions"`
	BulkSize      int `mapstructure:"bulk_size"`
}

type ElasticsearchOperator interface {
	processData(ctx context.Context, data []interface{}, esConf *ElasticsearchConf) error
	flushRecords() error
}

type Elasticsearch3Operator struct {
	esClient      *elasticv3.Client
	bulkProcessor *elasticv3.BulkProcessor
	log           *logrus.Entry
}

type Elasticsearch5Operator struct {
	esClient      *elasticv5.Client
	bulkProcessor *elasticv5.BulkProcessor
	log           *logrus.Entry
}

type Elasticsearch6Operator struct {
	esClient      *elasticv6.Client
	bulkProcessor *elasticv6.BulkProcessor
	log           *logrus.Entry
}

type ApiKeyTransport struct {
	APIKey   string
	APIKeyID string
}

//RoundTrip for ApiKeyTransport auth
func (t *ApiKeyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	auth := t.APIKeyID + ":" + t.APIKey
	key := base64.StdEncoding.EncodeToString([]byte(auth))

	r.Header.Set("Authorization", "ApiKey "+key)

	return http.DefaultTransport.RoundTrip(r)
}

func (e *ElasticsearchPump) getOperator() (ElasticsearchOperator, error) {
	conf := *e.esConf
	var err error

	urls := strings.Split(conf.ElasticsearchURL, ",")

	httpClient := http.DefaultClient
	if conf.AuthAPIKey != "" && conf.AuthAPIKeyID != "" {
		conf.Username = ""
		conf.Password = ""
		httpClient = &http.Client{Transport: &ApiKeyTransport{APIKey: conf.AuthAPIKey, APIKeyID: conf.AuthAPIKeyID}}
	}

	switch conf.Version {
	case "3":
		op := new(Elasticsearch3Operator)
		op.esClient, err = elasticv3.NewClient(elasticv3.SetURL(urls...), elasticv3.SetSniff(conf.EnableSniffing), elasticv3.SetBasicAuth(conf.Username, conf.Password), elasticv3.SetHttpClient(httpClient))

		if err != nil {
			return op, err
		}

		// Setup a bulk processor
		p := op.esClient.BulkProcessor().Name("TykPumpESv3BackgroundProcessor")
		if conf.BulkConfig.Workers != 0 {
			p = p.Workers(conf.BulkConfig.Workers)
		}

		if conf.BulkConfig.FlushInterval != 0 {
			p = p.FlushInterval(time.Duration(conf.BulkConfig.FlushInterval) * time.Second)
		}

		if conf.BulkConfig.BulkActions != 0 {
			p = p.BulkActions(conf.BulkConfig.BulkActions)
		}

		bulkSize := DEFAULT_BULK_SIZE
		if conf.BulkConfig.BulkSize != 0 {
			bulkSize = conf.BulkConfig.BulkSize
		}
		p = p.BulkSize(bulkSize)
		// After execute a bulk commit call his function to print how many records were purged
		purgerLogger := func(executionId int64, requests []elasticv3.BulkableRequest, response *elasticv3.BulkResponse, err error) {
			printPurgedBulkRecords(bulkSize, err, op.log)
		}
		p.After(purgerLogger)

		op.bulkProcessor, err = p.Do()
		op.log = e.log
		return op, err
	case "5":
		op := new(Elasticsearch5Operator)

		op.esClient, err = elasticv5.NewClient(elasticv5.SetURL(urls...), elasticv5.SetSniff(conf.EnableSniffing), elasticv5.SetBasicAuth(conf.Username, conf.Password), elasticv5.SetHttpClient(httpClient))

		if err != nil {
			return op, err
		}
		// Setup a bulk processor
		p := op.esClient.BulkProcessor().Name("TykPumpESv5BackgroundProcessor")
		if conf.BulkConfig.Workers != 0 {
			p = p.Workers(conf.BulkConfig.Workers)
		}

		if conf.BulkConfig.FlushInterval != 0 {
			p = p.FlushInterval(time.Duration(conf.BulkConfig.FlushInterval) * time.Second)
		}

		if conf.BulkConfig.BulkActions != 0 {
			p = p.BulkActions(conf.BulkConfig.BulkActions)
		}

		bulkSize := DEFAULT_BULK_SIZE
		if conf.BulkConfig.BulkSize != 0 {
			bulkSize = conf.BulkConfig.BulkSize
		}
		p = p.BulkSize(bulkSize)

		// After execute a bulk commit call his function to print how many records were purged
		purgerLogger := func(executionId int64, requests []elasticv5.BulkableRequest, response *elasticv5.BulkResponse, err error) {
			printPurgedBulkRecords(bulkSize, err, op.log)
		}
		p.After(purgerLogger)

		op.bulkProcessor, err = p.Do(context.Background())
		op.log = e.log
		return op, err
	case "6":
		op := new(Elasticsearch6Operator)

		op.esClient, err = elasticv6.NewClient(elasticv6.SetURL(urls...), elasticv6.SetSniff(conf.EnableSniffing), elasticv6.SetBasicAuth(conf.Username, conf.Password), elasticv6.SetHttpClient(httpClient))

		if err != nil {
			return op, err
		}
		// Setup a bulk processor
		p := op.esClient.BulkProcessor().Name("TykPumpESv6BackgroundProcessor")
		if conf.BulkConfig.Workers != 0 {
			p = p.Workers(conf.BulkConfig.Workers)
		}

		if conf.BulkConfig.FlushInterval != 0 {
			p = p.FlushInterval(time.Duration(conf.BulkConfig.FlushInterval) * time.Second)
		}

		if conf.BulkConfig.BulkActions != 0 {
			p = p.BulkActions(conf.BulkConfig.BulkActions)
		}

		bulkSize := DEFAULT_BULK_SIZE
		if conf.BulkConfig.BulkSize != 0 {
			bulkSize = conf.BulkConfig.BulkSize
		}
		p = p.BulkSize(bulkSize)

		// After execute a bulk commit call his function to print how many records were purged
		purgerLogger := func(executionId int64, requests []elasticv6.BulkableRequest, response *elasticv6.BulkResponse, err error) {
			printPurgedBulkRecords(bulkSize, err, op.log)
		}
		p.After(purgerLogger)

		op.bulkProcessor, err = p.Do(context.Background())
		op.log = e.log
		return op, err
	default:
		// shouldn't get this far, but hey never hurts to check assumptions
		e.log.Fatal("Invalid version: ")
	}

	return nil, err
}

func (e *ElasticsearchPump) New() Pump {
	newPump := ElasticsearchPump{}
	return &newPump
}

func (e *ElasticsearchPump) GetName() string {
	return "Elasticsearch Pump"
}

func (e *ElasticsearchPump) GetEnvPrefix() string {
	return e.esConf.EnvPrefix
}

func (e *ElasticsearchPump) Init(config interface{}) error {
	e.esConf = &ElasticsearchConf{}
	e.log = log.WithField("prefix", elasticsearchPrefix)

	loadConfigErr := mapstructure.Decode(config, &e.esConf)
	if loadConfigErr != nil {
		e.log.Fatal("Failed to decode configuration: ", loadConfigErr)
	}

	processPumpEnvVars(e, e.log, e.esConf, elasticsearchDefaultENV)

	if "" == e.esConf.IndexName {
		e.esConf.IndexName = "tyk_analytics"
	}

	if "" == e.esConf.ElasticsearchURL {
		e.esConf.ElasticsearchURL = "http://localhost:9200"
	}

	if "" == e.esConf.DocumentType {
		e.esConf.DocumentType = "tyk_analytics"
	}

	switch e.esConf.Version {
	case "":
		e.esConf.Version = "3"
		log.Info("Version not specified, defaulting to 3. If you are importing to Elasticsearch 5, please specify \"version\" = \"5\"")
	case "3", "5", "6":
	default:
		err := errors.New("Only 3, 5, 6 are valid values for this field")
		e.log.Fatal("Invalid version: ", err)
	}

	var re = regexp.MustCompile(`(.*)\/\/(.*):(.*)\@(.*)`)
	printableURL := re.ReplaceAllString(e.esConf.ElasticsearchURL, `$1//***:***@$4`)

	e.log.Info("Elasticsearch URL: ", printableURL)
	e.log.Info("Elasticsearch Index: ", e.esConf.IndexName)
	if e.esConf.RollingIndex {
		e.log.Info("Index will have date appended to it in the format ", e.esConf.IndexName, "-YYYY.MM.DD")
	}

	e.connect()

	e.log.Info(e.GetName() + " Initialized")
	return nil
}

func (e *ElasticsearchPump) connect() {
	var err error

	e.operator, err = e.getOperator()
	if err != nil {
		e.log.Error("Elasticsearch connection failed: ", err)
		time.Sleep(5 * time.Second)
		e.connect()
	}
}

func (e *ElasticsearchPump) WriteData(ctx context.Context, data []interface{}) error {
	e.log.Debug("Attempting to write ", len(data), " records...")

	if e.operator == nil {
		e.log.Debug("Connecting to analytics store")
		e.connect()
		e.WriteData(ctx, data)
	} else {
		if len(data) > 0 {
			e.operator.processData(ctx, data, e.esConf)
		}
	}
	return nil
}

func getIndexName(esConf *ElasticsearchConf) string {
	indexName := esConf.IndexName

	if esConf.RollingIndex {
		currentTime := time.Now()
		//This formats the date to be YYYY.MM.DD but Golang makes you use a specific date for its date formatting
		indexName += "-" + currentTime.Format("2006.01.02")
	}
	return indexName
}

func getMapping(datum analytics.AnalyticsRecord, extendedStatistics bool, generateID bool, decodeBase64 bool) (map[string]interface{}, string) {
	record := datum

	mapping := map[string]interface{}{
		"@timestamp":       record.TimeStamp,
		"http_method":      record.Method,
		"request_uri":      record.Path,
		"request_uri_full": record.RawPath,
		"response_code":    record.ResponseCode,
		"ip_address":       record.IPAddress,
		"api_key":          record.APIKey,
		"api_version":      record.APIVersion,
		"api_name":         record.APIName,
		"api_id":           record.APIID,
		"org_id":           record.OrgID,
		"oauth_id":         record.OauthID,
		"request_time_ms":  record.RequestTime,
		"alias":            record.Alias,
		"content_length":   record.ContentLength,
		"tags":             record.Tags,
	}

	if extendedStatistics {
		if decodeBase64 {
			rawRequest, _ := base64.StdEncoding.DecodeString(record.RawRequest)
			mapping["raw_request"] = string(rawRequest)
			rawResponse, _ := base64.StdEncoding.DecodeString(record.RawResponse)
			mapping["raw_response"] = string(rawResponse)
		} else {
			mapping["raw_request"] = record.RawRequest
			mapping["raw_response"] = record.RawResponse
		}
		mapping["user_agent"] = record.UserAgent
	}

	if generateID {
		hasher := murmur3.New64()
		hasher.Write([]byte(fmt.Sprintf("%d%s%s%s%s%s%d%s", record.TimeStamp.UnixNano(), record.Method, record.Path, record.IPAddress, record.APIID, record.OauthID, record.RequestTime, record.Alias)))

		return mapping, string(hasher.Sum(nil))
	}

	return mapping, ""
}

func (e Elasticsearch3Operator) processData(ctx context.Context, data []interface{}, esConf *ElasticsearchConf) error {
	index := e.esClient.Index().Index(getIndexName(esConf))

	for dataIndex := range data {
		if ctxErr := ctx.Err(); ctxErr != nil {
			continue
		}

		d, ok := data[dataIndex].(analytics.AnalyticsRecord)
		if !ok {
			e.log.Error("Error while writing ", data[dataIndex], ": data not of type analytics.AnalyticsRecord")
			continue
		}

		mapping, id := getMapping(d, esConf.ExtendedStatistics, esConf.GenerateID, esConf.DecodeBase64)

		if !esConf.DisableBulk {
			r := elasticv3.NewBulkIndexRequest().Index(getIndexName(esConf)).Type(esConf.DocumentType).Id(id).Doc(mapping)
			e.bulkProcessor.Add(r)
		} else {
			_, err := index.BodyJson(mapping).Type(esConf.DocumentType).Id(id).DoC(ctx)
			if err != nil {
				e.log.Error("Error while writing ", data[dataIndex], err)
			}
		}
	}
	e.log.Info("Purged ", len(data), " records...")

	return nil
}

func (e Elasticsearch3Operator) flushRecords() error {
	return e.bulkProcessor.Flush()
}

func (e Elasticsearch5Operator) processData(ctx context.Context, data []interface{}, esConf *ElasticsearchConf) error {
	index := e.esClient.Index().Index(getIndexName(esConf))

	for dataIndex := range data {
		if ctxErr := ctx.Err(); ctxErr != nil {
			continue
		}

		d, ok := data[dataIndex].(analytics.AnalyticsRecord)
		if !ok {
			e.log.Error("Error while writing ", data[dataIndex], ": data not of type analytics.AnalyticsRecord")
			continue
		}

		mapping, id := getMapping(d, esConf.ExtendedStatistics, esConf.GenerateID, esConf.DecodeBase64)

		if !esConf.DisableBulk {
			r := elasticv5.NewBulkIndexRequest().Index(getIndexName(esConf)).Type(esConf.DocumentType).Id(id).Doc(mapping)
			e.bulkProcessor.Add(r)
		} else {
			_, err := index.BodyJson(mapping).Type(esConf.DocumentType).Id(id).Do(ctx)
			if err != nil {
				e.log.Error("Error while writing ", data[dataIndex], err)
			}
		}
	}
	e.log.Info("Purged ", len(data), " records...")

	return nil
}

func (e Elasticsearch5Operator) flushRecords() error {
	return e.bulkProcessor.Flush()
}

func (e Elasticsearch6Operator) processData(ctx context.Context, data []interface{}, esConf *ElasticsearchConf) error {
	index := e.esClient.Index().Index(getIndexName(esConf))

	for dataIndex := range data {
		if ctxErr := ctx.Err(); ctxErr != nil {
			continue
		}

		d, ok := data[dataIndex].(analytics.AnalyticsRecord)
		if !ok {
			e.log.Error("Error while writing ", data[dataIndex], ": data not of type analytics.AnalyticsRecord")
			continue
		}

		mapping, id := getMapping(d, esConf.ExtendedStatistics, esConf.GenerateID, esConf.DecodeBase64)

		if !esConf.DisableBulk {
			r := elasticv6.NewBulkIndexRequest().Index(getIndexName(esConf)).Type(esConf.DocumentType).Id(id).Doc(mapping)
			e.bulkProcessor.Add(r)
		} else {
			_, err := index.BodyJson(mapping).Type(esConf.DocumentType).Id(id).Do(ctx)
			if err != nil {
				e.log.Error("Error while writing ", data[dataIndex], err)
			}
		}
	}

	// when bulk disabled then print the number of records
	// for bulk ops a bulkAfterFunc has been set
	if esConf.DisableBulk {
		e.log.Info("Purged ", len(data), " records...")
	}

	return nil
}

func (e Elasticsearch6Operator) flushRecords() error {
	return e.bulkProcessor.Flush()
}

// printPurgedBulkRecords print the purged records = bulk size when bulk is enabled
func printPurgedBulkRecords(bulkSize int, err error, logger *logrus.Entry) {
	if err != nil {
		logger.WithError(err).Errorf("Purging %+v  records", bulkSize)
	}
	logger.Infof("Purging %+v records", bulkSize)
}

func (e *ElasticsearchPump) Shutdown() error {
	return e.operator.flushRecords()
}
