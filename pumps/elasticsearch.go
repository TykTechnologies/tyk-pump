package pumps

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	elasticv8 "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esutil"
	"github.com/mitchellh/mapstructure"
	elasticv7 "github.com/olivere/elastic/v7"
	elasticv3 "gopkg.in/olivere/elastic.v3"
	elasticv5 "gopkg.in/olivere/elastic.v5"
	elasticv6 "gopkg.in/olivere/elastic.v6"

	"github.com/TykTechnologies/murmur3"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/sirupsen/logrus"
)

type ElasticsearchPump struct {
	operator ElasticsearchOperator
	esConf   *ElasticsearchConf
	CommonPumpConfig
}

var elasticsearchPrefix = "elasticsearch-pump"
var elasticsearchDefaultENV = PUMPS_ENV_PREFIX + "_ELASTICSEARCH" + PUMPS_ENV_META_PREFIX

// @PumpConf Elasticsearch
type ElasticsearchConf struct {
	// The prefix for the environment variables that will be used to override the configuration.
	// Defaults to `TYK_PMP_PUMPS_ELASTICSEARCH_META`
	EnvPrefix string `mapstructure:"meta_env_prefix"`
	// The name of the index that all the analytics data will be placed in. Defaults to
	// "tyk_analytics".
	IndexName string `json:"index_name" mapstructure:"index_name"`
	// If sniffing is disabled, the URL that all data will be sent to. Defaults to
	// "http://localhost:9200".
	ElasticsearchURL string `json:"elasticsearch_url" mapstructure:"elasticsearch_url"`
	// If sniffing is enabled, the "elasticsearch_url" will be used to make a request to get a
	// list of all the nodes in the cluster, the returned addresses will then be used. Defaults to
	// `false`.
	EnableSniffing bool `json:"use_sniffing" mapstructure:"use_sniffing"`
	// The type of the document that is created in ES. Defaults to "tyk_analytics".
	DocumentType string `json:"document_type" mapstructure:"document_type"`
	// Appends the date to the end of the index name, so each days data is split into a different
	// index name. E.g. tyk_analytics-2016.02.28. Defaults to `false`.
	RollingIndex bool `json:"rolling_index" mapstructure:"rolling_index"`
	// If set to `true` will include the following additional fields: Raw Request, Raw Response and
	// User Agent.
	ExtendedStatistics bool `json:"extended_stats" mapstructure:"extended_stats"`
	// When enabled, generate _id for outgoing records. This prevents duplicate records when
	// retrying ES.
	GenerateID bool `json:"generate_id" mapstructure:"generate_id"`
	// Allows for the base64 bits to be decode before being passed to ES.
	DecodeBase64 bool `json:"decode_base64" mapstructure:"decode_base64"`
	// Specifies the ES version. Use "3" for ES 3.X, "5" for ES 5.X, "6" for ES 6.X, "7" for ES
	// 7.X . Defaults to "3".
	Version string `json:"version" mapstructure:"version"`
	// Disable batch writing. Defaults to false.
	DisableBulk bool `json:"disable_bulk" mapstructure:"disable_bulk"`
	// Batch writing trigger configuration. Each option is an OR with eachother:
	BulkConfig ElasticsearchBulkConfig `json:"bulk_config" mapstructure:"bulk_config"`
	// API Key ID used for APIKey auth in ES. It's send to ES in the Authorization header as ApiKey base64(auth_api_key_id:auth_api_key)
	AuthAPIKeyID string `json:"auth_api_key_id" mapstructure:"auth_api_key_id"`
	// API Key used for APIKey auth in ES. It's send to ES in the Authorization header as ApiKey base64(auth_api_key_id:auth_api_key)
	AuthAPIKey string `json:"auth_api_key" mapstructure:"auth_api_key"`
	// Basic auth username. It's send to ES in the Authorization header as username:password encoded in base64.
	Username string `json:"auth_basic_username" mapstructure:"auth_basic_username"`
	// Basic auth password. It's send to ES in the Authorization header as username:password encoded in base64.
	Password string `json:"auth_basic_password" mapstructure:"auth_basic_password"`
	// Enables SSL connection.
	UseSSL bool `json:"use_ssl" mapstructure:"use_ssl"`
	// Controls whether the pump client verifies the Elastic Search server's certificate chain and hostname.
	SSLInsecureSkipVerify bool `json:"ssl_insecure_skip_verify" mapstructure:"ssl_insecure_skip_verify"`
	// Can be used to set custom certificate file for authentication with Elastic Search.
	SSLCertFile string `json:"ssl_cert_file" mapstructure:"ssl_cert_file"`
	// Can be used to set custom key file for authentication with Elastic Search.
	SSLKeyFile string `json:"ssl_key_file" mapstructure:"ssl_key_file"`
}

type ElasticsearchBulkConfig struct {
	// Number of workers. Defaults to 1.
	Workers int `json:"workers" mapstructure:"workers"`
	// Specifies the time in seconds to flush the data and send it to ES. Default disabled.
	FlushInterval int `json:"flush_interval" mapstructure:"flush_interval"`
	// Specifies the number of requests needed to flush the data and send it to ES. Defaults to
	// 1000 requests. If it is needed, can be disabled with -1.
	BulkActions int `json:"bulk_actions" mapstructure:"bulk_actions"`
	// Specifies the size (in bytes) needed to flush the data and send it to ES. Defaults to 5MB.
	// If it is needed, can be disabled with -1.
	BulkSize int `json:"bulk_size" mapstructure:"bulk_size"`
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

type Elasticsearch7Operator struct {
	esClient      *elasticv7.Client
	bulkProcessor *elasticv7.BulkProcessor
	log           *logrus.Entry
}

type Elasticsearch8Operator struct {
	conf        *ElasticsearchConf
	esClient    *elasticv8.Client
	bulkIndexer esutil.BulkIndexer
	log         *logrus.Entry
}

type ApiKeyTransport struct {
	APIKey   string
	APIKeyID string
}

// RoundTrip for ApiKeyTransport auth
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

	var httpTransport http.RoundTripper = nil
	httpClient := http.DefaultClient
	if conf.AuthAPIKey != "" && conf.AuthAPIKeyID != "" {
		conf.Username = ""
		conf.Password = ""
		httpTransport = &ApiKeyTransport{APIKey: conf.AuthAPIKey, APIKeyID: conf.AuthAPIKeyID}
		httpClient = &http.Client{Transport: httpTransport}
	}

	if conf.UseSSL {
		tlsConf, err := e.GetTLSConfig()
		if err != nil {
			e.log.WithError(err).Error("Failed to get TLS config")
			return nil, err
		}
		httpTransport = &http.Transport{TLSClientConfig: tlsConf}
		httpClient = &http.Client{Transport: httpTransport}
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

		if conf.BulkConfig.BulkSize != 0 {
			p = p.BulkSize(conf.BulkConfig.BulkSize)
		}

		if !conf.DisableBulk {
			// After execute a bulk commit call his function to print how many records were purged
			purgerLogger := func(executionId int64, requests []elasticv3.BulkableRequest, response *elasticv3.BulkResponse, err error) {
				printPurgedBulkRecords(len(requests), err, op.log)
			}
			p = p.After(purgerLogger)
		}

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

		if conf.BulkConfig.BulkSize != 0 {
			p = p.BulkSize(conf.BulkConfig.BulkSize)
		}

		if !conf.DisableBulk {
			// After execute a bulk commit call his function to print how many records were purged
			purgerLogger := func(executionId int64, requests []elasticv5.BulkableRequest, response *elasticv5.BulkResponse, err error) {
				printPurgedBulkRecords(len(requests), err, op.log)
			}
			p = p.After(purgerLogger)
		}

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

		if conf.BulkConfig.BulkSize != 0 {
			p = p.BulkSize(conf.BulkConfig.BulkSize)
		}

		if !conf.DisableBulk {
			// After execute a bulk commit call his function to print how many records were purged
			purgerLogger := func(executionId int64, requests []elasticv6.BulkableRequest, response *elasticv6.BulkResponse, err error) {
				printPurgedBulkRecords(len(requests), err, op.log)
			}
			p = p.After(purgerLogger)
		}

		op.bulkProcessor, err = p.Do(context.Background())
		op.log = e.log
		return op, err
	case "7":
		op := new(Elasticsearch7Operator)

		op.esClient, err = elasticv7.NewClient(elasticv7.SetURL(urls...), elasticv7.SetSniff(conf.EnableSniffing), elasticv7.SetBasicAuth(conf.Username, conf.Password), elasticv7.SetHttpClient(httpClient))

		if err != nil {
			return op, err
		}
		// Setup a bulk processor
		p := op.esClient.BulkProcessor().Name("TykPumpESv7BackgroundProcessor")
		if conf.BulkConfig.Workers != 0 {
			p = p.Workers(conf.BulkConfig.Workers)
		}

		if conf.BulkConfig.FlushInterval != 0 {
			p = p.FlushInterval(time.Duration(conf.BulkConfig.FlushInterval) * time.Second)
		}

		if conf.BulkConfig.BulkActions != 0 {
			p = p.BulkActions(conf.BulkConfig.BulkActions)
		}

		if conf.BulkConfig.BulkSize != 0 {
			p = p.BulkSize(conf.BulkConfig.BulkSize)
		}

		if !conf.DisableBulk {
			// After execute a bulk commit call his function to print how many records were purged
			purgerLogger := func(executionId int64, requests []elasticv7.BulkableRequest, response *elasticv7.BulkResponse, err error) {
				printPurgedBulkRecords(len(requests), err, op.log)
			}
			p = p.After(purgerLogger)
		}

		op.bulkProcessor, err = p.Do(context.Background())
		op.log = e.log
		return op, err
	case "8":
		op := &Elasticsearch8Operator{
			conf: &conf,
		}

		cfg := elasticv8.Config{
			Addresses: urls,
		}
		if conf.Username != "" || conf.Password != "" {
			cfg.Username = conf.Username
			cfg.Password = conf.Password
		}
		if httpTransport != nil {
			cfg.Transport = httpTransport
		}

		op.esClient, err = elasticv8.NewClient(cfg)

		if err != nil {
			return op, err
		}

		op.bulkIndexer, err = setupElasticsearch8BulkIndexer(op)

		if err != nil {
			return op, err
		}

		op.log = e.log
		return op, err
	default:
		// shouldn't get this far, but hey never hurts to check assumptions
		e.log.Fatal("Invalid version: ")
	}

	return nil, err
}

func setupElasticsearch8BulkIndexer(op *Elasticsearch8Operator) (esutil.BulkIndexer, error) {
	// Setup a bulk indexer
	bulkCfg := esutil.BulkIndexerConfig{
		Index:  getIndexName(op.conf),
		Client: op.esClient,
	}

	if op.conf.BulkConfig.Workers != 0 {
		bulkCfg.NumWorkers = op.conf.BulkConfig.Workers
	}

	if op.conf.BulkConfig.FlushInterval != 0 {
		bulkCfg.FlushInterval = time.Duration(op.conf.BulkConfig.FlushInterval) * time.Second
	}

	// op.conf.BulkConfig.BulkActions not supported

	// op.conf.BulkConfig.BulkSize not supported

	return esutil.NewBulkIndexer(bulkCfg)
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
	case "3", "5", "6", "7", "8":
	default:
		err := errors.New("Only 3, 5, 6, 7, 8 are valid values for this field")
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

// GetTLSConfig sets the TLS config for the pump
func (e *ElasticsearchPump) GetTLSConfig() (*tls.Config, error) {
	var tlsConfig *tls.Config
	// If the user has not specified a CA file nor a key file, we'll use a tls config with no certs
	if e.esConf.SSLCertFile == "" && e.esConf.SSLKeyFile == "" {
		// #nosec G402
		tlsConfig = &tls.Config{
			InsecureSkipVerify: e.esConf.SSLInsecureSkipVerify,
		}
		return tlsConfig, nil
	}

	// If the user has specified both a SSL cert file and a key file, we'll use them to create a tls config
	if e.esConf.SSLCertFile != "" && e.esConf.SSLKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(e.esConf.SSLCertFile, e.esConf.SSLKeyFile)
		if err != nil {
			return tlsConfig, err
		}
		// #nosec G402
		tlsConfig = &tls.Config{
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: e.esConf.SSLInsecureSkipVerify,
		}
		return tlsConfig, nil
	}

	// If the user has specified a SSL cert file or a key file, but not both, we'll return an error
	err := errors.New("only one of ssl_cert_file and ssl_cert_key configuration option is setted, you should set both to enable mTLS")
	return tlsConfig, err
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
	if esConf.DisableBulk {
		e.log.Info("Purged ", len(data), " records...")
	}
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
	if esConf.DisableBulk {
		e.log.Info("Purged ", len(data), " records...")
	}
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

func (e Elasticsearch7Operator) processData(ctx context.Context, data []interface{}, esConf *ElasticsearchConf) error {
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
			r := elasticv7.NewBulkIndexRequest().Index(getIndexName(esConf)).Id(id).Doc(mapping)
			e.bulkProcessor.Add(r)
		} else {
			_, err := index.BodyJson(mapping).Id(id).Do(ctx)
			if err != nil {
				e.log.Error("Error while writing ", data[dataIndex], err)
			}
		}
	}
	if esConf.DisableBulk {
		e.log.Info("Purged ", len(data), " records...")
	}

	return nil
}

func (e Elasticsearch7Operator) flushRecords() error {
	return e.bulkProcessor.Flush()
}

func (e *Elasticsearch8Operator) processData(ctx context.Context, data []interface{}, esConf *ElasticsearchConf) error {
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
		bs, err := json.Marshal(mapping)
		if err != nil {
			e.log.Error("Error while writing ", data[dataIndex], ": failed to marshal into JSON: ", err)
			continue
		}
		body := bytes.NewReader(bs)

		if !esConf.DisableBulk {
			err = e.bulkIndexer.Add(
				ctx,
				esutil.BulkIndexerItem{
					Action:     "index",
					Body:       body,
					Index:      getIndexName(esConf),
					DocumentID: id,

					OnSuccess: func(ctx context.Context, item esutil.BulkIndexerItem, resp esutil.BulkIndexerResponseItem) {
						e.log.Info("Purged 1 record...")
					},
					OnFailure: func(ctx context.Context, item esutil.BulkIndexerItem, resp esutil.BulkIndexerResponseItem, err error) {
						e.log.Error("Error while writing ", data[dataIndex], err)
					},
				},
			)
			if err != nil {
				e.log.Error("Error while adding ", data[dataIndex], " to BulkIndexer: ", err)
			}
		} else {
			e.esClient.Index(
				getIndexName(esConf),
				body,
				e.esClient.Index.WithDocumentID(id),
				e.esClient.Index.WithContext(ctx),
			)
			if err != nil {
				e.log.Error("Error while writing ", data[dataIndex], err)
			}
		}
	}
	if esConf.DisableBulk {
		e.log.Info("Purged ", len(data), " records...")
	}

	return nil
}

func (e *Elasticsearch8Operator) flushRecords() error {
	err := e.bulkIndexer.Close(context.Background())
	if err != nil {
		return err
	}
	e.log.Info("Purged ", e.bulkIndexer.Stats().NumFlushed, " records in this bulk...")
	e.bulkIndexer, err = setupElasticsearch8BulkIndexer(e)
	return err
}

// printPurgedBulkRecords print the purged records = bulk size when bulk is enabled
func printPurgedBulkRecords(bulkSize int, err error, logger *logrus.Entry) {
	if err != nil {
		logger.WithError(err).Errorf("Error Purging %+v records", bulkSize)
		return
	}
	logger.Infof("Purged %+v records", bulkSize)
}

func (e *ElasticsearchPump) Shutdown() error {
	if !e.esConf.DisableBulk {
		e.log.Info("Flushing bulked records...")
		return e.operator.flushRecords()
	}
	return nil
}
