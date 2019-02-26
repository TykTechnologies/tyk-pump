package pumps

import (
	"context"
	"errors"
	"time"

	"github.com/mitchellh/mapstructure"
	elasticv3 "gopkg.in/olivere/elastic.v3"
	elasticv5 "gopkg.in/olivere/elastic.v5"
	elasticv6 "gopkg.in/olivere/elastic.v6"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
)

type ElasticsearchPump struct {
	operator ElasticsearchOperator
	esConf   *ElasticsearchConf
}

var elasticsearchPrefix = "elasticsearch-pump"

type ElasticsearchConf struct {
	IndexName          string `mapstructure:"index_name"`
	ElasticsearchURL   string `mapstructure:"elasticsearch_url"`
	EnableSniffing     bool   `mapstructure:"use_sniffing"`
	DocumentType       string `mapstructure:"document_type"`
	RollingIndex       bool   `mapstructure:"rolling_index"`
	ExtendedStatistics bool   `mapstructure:"extended_stats"`
	Version            string `mapstructure:"version"`
}

type ElasticsearchOperator interface {
	processData(data []interface{}, esConf *ElasticsearchConf) error
}

type Elasticsearch3Operator struct {
	esClient *elasticv3.Client
}

type Elasticsearch5Operator struct {
	esClient *elasticv5.Client
}

type Elasticsearch6Operator struct {
	esClient *elasticv6.Client
}

func getOperator(version string, url string, setSniff bool) (ElasticsearchOperator, error) {
	var err error

	switch version {
	case "3":
		e := new(Elasticsearch3Operator)
		e.esClient, err = elasticv3.NewClient(elasticv3.SetURL(url), elasticv3.SetSniff(setSniff))
		return e, err
	case "5":
		e := new(Elasticsearch5Operator)
		e.esClient, err = elasticv5.NewClient(elasticv5.SetURL(url), elasticv5.SetSniff(setSniff))
		return e, err
	case "6":
		e := new(Elasticsearch6Operator)
		e.esClient, err = elasticv6.NewClient(elasticv6.SetURL(url), elasticv6.SetSniff(setSniff))
		return e, err
	default:
		// shouldn't get this far, but hey never hurts to check assumptions
		log.WithFields(logrus.Fields{
			"prefix": elasticsearchPrefix,
		}).Fatal("Invalid version: ")
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

func (e *ElasticsearchPump) Init(config interface{}) error {
	e.esConf = &ElasticsearchConf{}
	loadConfigErr := mapstructure.Decode(config, &e.esConf)

	if loadConfigErr != nil {
		log.WithFields(logrus.Fields{
			"prefix": elasticsearchPrefix,
		}).Fatal("Failed to decode configuration: ", loadConfigErr)
	}

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
		log.WithFields(logrus.Fields{
			"prefix": elasticsearchPrefix,
		}).Info("Version not specified, defaulting to 3. If you are importing to Elasticsearch 5, please specify \"version\" = \"5\"")
	case "3", "5":
	default:
		err := errors.New("Only 3 or 5 are valid values for this field")
		log.WithFields(logrus.Fields{
			"prefix": elasticsearchPrefix,
		}).Fatal("Invalid version: ", err)
	}

	log.WithFields(logrus.Fields{
		"prefix": elasticsearchPrefix,
	}).Info("Elasticsearch URL: ", e.esConf.ElasticsearchURL)
	log.WithFields(logrus.Fields{
		"prefix": elasticsearchPrefix,
	}).Info("Elasticsearch Index: ", e.esConf.IndexName)
	if e.esConf.RollingIndex {
		log.WithFields(logrus.Fields{
			"prefix": elasticsearchPrefix,
		}).Info("Index will have date appended to it in the format ", e.esConf.IndexName, "-YYYY.MM.DD")
	}

	e.connect()

	return nil
}

func (e *ElasticsearchPump) connect() {
	var err error

	e.operator, err = getOperator(e.esConf.Version, e.esConf.ElasticsearchURL, e.esConf.EnableSniffing)
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": elasticsearchPrefix,
		}).Error("Elasticsearch connection failed: ", err)
		time.Sleep(5 * time.Second)
		e.connect()
	}
}

func (e *ElasticsearchPump) WriteData(data []interface{}) error {
	log.WithFields(logrus.Fields{
		"prefix": elasticsearchPrefix,
	}).Info("Writing ", len(data), " records")

	if e.operator == nil {
		log.WithFields(logrus.Fields{
			"prefix": elasticsearchPrefix,
		}).Debug("Connecting to analytics store")
		e.connect()
		e.WriteData(data)
	} else {
		if len(data) > 0 {
			e.operator.processData(data, e.esConf)
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

func getMapping(datum analytics.AnalyticsRecord, extendedStatistics bool) map[string]interface{} {
	record := datum

	mapping := map[string]interface{}{
		"@timestamp":      record.TimeStamp,
		"http_method":     record.Method,
		"request_uri":     record.Path,
		"response_code":   record.ResponseCode,
		"ip_address":      record.IPAddress,
		"api_key":         record.APIKey,
		"api_version":     record.APIVersion,
		"api_name":        record.APIName,
		"api_id":          record.APIID,
		"org_id":          record.OrgID,
		"oauth_id":        record.OauthID,
		"request_time_ms": record.RequestTime,
	}

	if extendedStatistics {
		mapping["raw_request"] = record.RawRequest
		mapping["raw_response"] = record.RawResponse
		mapping["user_agent"] = record.UserAgent
	}
	return mapping
}

func (e Elasticsearch3Operator) processData(data []interface{}, esConf *ElasticsearchConf) error {
	index := e.esClient.Index().Index(getIndexName(esConf))

	for dataIndex := range data {
		d, ok := data[dataIndex].(analytics.AnalyticsRecord)
		if !ok {
			log.WithFields(logrus.Fields{
				"prefix": elasticsearchPrefix,
			}).Error("Error while writing ", data[dataIndex], ": data not of type analytics.AnalyticsRecord")
			continue
		}

		mapping := getMapping(d, esConf.ExtendedStatistics)

		_, err := index.BodyJson(mapping).Type(esConf.DocumentType).Do()
		if err != nil {
			log.WithFields(logrus.Fields{
				"prefix": elasticsearchPrefix,
			}).Error("Error while writing ", data[dataIndex], err)
		}
	}

	return nil
}

func (e Elasticsearch5Operator) processData(data []interface{}, esConf *ElasticsearchConf) error {
	index := e.esClient.Index().Index(getIndexName(esConf))

	for dataIndex := range data {
		d, ok := data[dataIndex].(analytics.AnalyticsRecord)
		if !ok {
			log.WithFields(logrus.Fields{
				"prefix": elasticsearchPrefix,
			}).Error("Error while writing ", data[dataIndex], ": data not of type analytics.AnalyticsRecord")
			continue
		}

		mapping := getMapping(d, esConf.ExtendedStatistics)

		_, err := index.BodyJson(mapping).Type(esConf.DocumentType).Do(context.TODO())
		if err != nil {
			log.WithFields(logrus.Fields{
				"prefix": elasticsearchPrefix,
			}).Error("Error while writing ", data[dataIndex], err)
		}
	}

	return nil
}

func (e Elasticsearch6Operator) processData(data []interface{}, esConf *ElasticsearchConf) error {
	index := e.esClient.Index().Index(getIndexName(esConf))

	for dataIndex := range data {
		d, ok := data[dataIndex].(analytics.AnalyticsRecord)
		if !ok {
			log.WithFields(logrus.Fields{
				"prefix": elasticsearchPrefix,
			}).Error("Error while writing ", data[dataIndex], ": data not of type analytics.AnalyticsRecord")
			continue
		}

		mapping := getMapping(d, esConf.ExtendedStatistics)

		_, err := index.BodyJson(mapping).Type(esConf.DocumentType).Do(context.Background())
		if err != nil {
			log.WithFields(logrus.Fields{
				"prefix": elasticsearchPrefix,
			}).Error("Error while writing ", data[dataIndex], err)
		}
	}

	return nil
}
