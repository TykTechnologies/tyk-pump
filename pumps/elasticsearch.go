package pumps

import (
	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/olivere/elastic.v3"
	"time"
)

type ElasticsearchPump struct {
	esClient *elastic.Client
	esConf   *ElasticsearchConf
}

var elasticsearchPrefix string = "elasticsearch-pump"

type ElasticsearchConf struct {
	IndexName        string `mapstructure:"index_name"`
	ElasticsearchURL string `mapstructure:"elasticsearch_url"`
	EnableSniffing   bool   `mapstructure:"use_sniffing"`
	DocumentType     string `mapstructure:"document_type"`
	RollingIndex     bool   `mapstructure:"rolling_index"`
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

	e.esClient, err = elastic.NewClient(elastic.SetURL(e.esConf.ElasticsearchURL), elastic.SetSniff(e.esConf.EnableSniffing))
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": elasticsearchPrefix,
		}).Error("Elasticsearch connection failed: ", err)
		time.Sleep(5)
		e.connect()
	}
}

func (e *ElasticsearchPump) WriteData(data []interface{}) error {
	log.WithFields(logrus.Fields{
		"prefix": elasticsearchPrefix,
	}).Info("Writing ", len(data), " records")

	if e.esClient == nil {
		log.WithFields(logrus.Fields{
			"prefix": elasticsearchPrefix,
		}).Debug("Connecting to analytics store")
		e.connect()
		e.WriteData(data)
	} else {
		if len(data) > 0 {

			var indexName = e.esConf.IndexName

			var currentTime = time.Now()

			if e.esConf.RollingIndex {
				//This formats the date to be YYYY.MM.DD but Golang makes you use a specific date for its date formatting
				indexName = indexName + "-" + currentTime.Format("2006.01.02")
			}

			var index = e.esClient.Index().Index(indexName)

			for dataIndex := range data {
				var record, _ = data[dataIndex].(analytics.AnalyticsRecord)

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

				var _, err = index.BodyJson(mapping).Type(e.esConf.DocumentType).Do()
				if err != nil {
					log.WithFields(logrus.Fields{
						"prefix": elasticsearchPrefix,
					}).Error("Error while writing ", data[dataIndex], err)
				}
			}
		}
	}

	return nil
}
