package pumps

import (
	"github.com/Sirupsen/logrus"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/olivere/elastic.v3"
	"time"
)

type ElasticsearchPump struct {
	esClient  *elastic.Client
	esConf    *ElasticsearchConf
}

var elasticsearchPrefix string = "elasticsearch-pump"

type ElasticsearchConf struct {
	IndexName           string `mapstructure:"index_name"`
	ElasticsearchURL    string `mapstructure:"elasticsearch_url"`
	EnableSniffing      bool   `mapstructure:"use_sniffing"`
	DocumentType        string `mapstructure:"document_type"`
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

	e.connect()

	var indexExists, existsError = e.esClient.IndexExists(e.esConf.IndexName).Do()
	if existsError != nil {
		log.WithFields(logrus.Fields{
		"prefix": elasticsearchPrefix,
		}).Error("Error while checking if index exists", existsError)
	} else if indexExists == false {
	    log.WithFields(logrus.Fields{
		"prefix": elasticsearchPrefix,
		}).Info("Creating index: ", e.esConf.IndexName)

		var _, createErr = e.esClient.CreateIndex(e.esConf.IndexName).Do()
		if createErr != nil {
			log.WithFields(logrus.Fields{
			"prefix": elasticsearchPrefix,
			}).Error("Elasticsearch Index faied to create", createErr)
		}
	}

	return nil
}

func (e *ElasticsearchPump) connect() {
	var err error

	e.esClient, err = elastic.NewClient(elastic.SetURL(e.esConf.ElasticsearchURL), elastic.SetSniff(e.esConf.EnableSniffing))
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": elasticsearchPrefix,
		}).Error("Elasticsearch connection failed:", err)
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

			var index = e.esClient.Index().Index(e.esConf.IndexName)

			for dataIndex := range data {
				var _, err = index.BodyJson(data[dataIndex]).Type(e.esConf.DocumentType).Do()
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