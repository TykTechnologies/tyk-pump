package pumps

import (
	"encoding/json"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/influxdata/influxdb/client/v2"
	"github.com/mitchellh/mapstructure"
)

type InfluxPump struct {
	dbConf *InfluxConf
}

var (
	influxPrefix string = "influx-pump"
	table        string = "analytics"
)

type InfluxConf struct {
	DatabaseName string   `mapstructure:"database_name"`
	Addr         string   `mapstructure:"address"`
	Username     string   `mapstructure:"username"`
	Password     string   `mapstructure:"password"`
	Fields       []string `mapstructure:"fields"`
	Tags         []string `mapstructure:"tags"`
}

func (i *InfluxPump) New() Pump {
	newPump := InfluxPump{}
	return &newPump
}

func (i *InfluxPump) GetName() string {
	return "InfluxDB Pump"
}

func (i *InfluxPump) Init(config interface{}) error {
	i.dbConf = &InfluxConf{}
	err := mapstructure.Decode(config, &i.dbConf)

	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": influxPrefix,
		}).Fatal("Failed to decode configuration: ", err)
	}

	i.connect()

	log.WithFields(logrus.Fields{
		"prefix": influxPrefix,
	}).Debug("Influx DB CS: ", i.dbConf.Addr)

	return nil
}

func (i *InfluxPump) connect() client.Client {
	c, err := client.NewHTTPClient(client.HTTPConfig{
		Addr:     i.dbConf.Addr,
		Username: i.dbConf.Username,
		Password: i.dbConf.Password,
	})

	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": influxPrefix,
		}).Error("Influx connection failed:", err)
		time.Sleep(5)
		i.connect()
	}

	return c
}

func (i *InfluxPump) WriteData(data []interface{}) error {
	c := i.connect()
	defer c.Close()

	bp, _ := client.NewBatchPoints(client.BatchPointsConfig{
		Database:  i.dbConf.DatabaseName,
		Precision: "us",
	})

	var pt *client.Point
	var err error

	//	 Create a point and add to batch
	for _, v := range data {
		// Convert to AnalyticsRecord
		decoded := v.(analytics.AnalyticsRecord)
		mapping := map[string]interface{}{
			"method":        decoded.Method,
			"path":          decoded.Path,
			"response_code": decoded.ResponseCode,
			"api_key":       decoded.APIKey,
			"time_stamp":    decoded.TimeStamp,
			"api_version":   decoded.APIVersion,
			"api_name":      decoded.APIName,
			"api_id":        decoded.APIID,
			"org_id":        decoded.OrgID,
			"oauth_id":      decoded.OauthID,
			"raw_request":   decoded.RawRequest,
			"request_time":  decoded.RequestTime,
			"raw_response":  decoded.RawResponse,
			"ip_address":    decoded.IPAddress,
		}

		var tags map[string]string
		var fields map[string]interface{}

		// Select tags from config
		for _, t := range i.dbConf.Tags {
			var tag string

			b, err := json.Marshal(mapping[t])
			if err != nil {
				tag = ""
			} else {
				tag = string(b)
			}
			tags[t] = tag
		}

		// Select field from config
		for _, f := range i.dbConf.Fields {
			fields[f] = mapping[f]
		}

		// New record
		if pt, err = client.NewPoint(table, tags, fields, time.Now()); err != nil {
			log.Error(err)
			continue
		}

		// Add point to batch point
		bp.AddPoint(pt)
		// Write the batch
		c.Write(bp)
	}

	return nil
}
