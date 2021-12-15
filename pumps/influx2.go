package pumps

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"github.com/mitchellh/mapstructure"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

type Influx2Pump struct {
	dbConf *Influx2Conf
	client influxdb2.Client
	CommonPumpConfig
}

var (
	influx2Prefix      = "influx2-pump"
	influx2DefaultENV  = PUMPS_ENV_PREFIX + "_INFLUX2" + PUMPS_ENV_META_PREFIX
	influx2Measurement = "analytics"
)

// @PumpConf Influx2
type Influx2Conf struct {
	EnvPrefix string `mapstructure:"meta_env_prefix"`
	// InfluxDB2 pump bucket name.
	BucketName string `mapstructure:"bucket" json:"bucket"`
	// InfluxDB2 pump organization name.
	OrgName string `mapstructure:"organization" json:"organization"`
	// InfluxDB2 pump host.
	Addr string `mapstructure:"address" json:"address"`
	// InfluxDB2 pump database token.
	Token string `mapstructure:"token" json:"token"`
	// Define which Analytics fields should be sent to InfluxDB2. Check the available
	// fields in the example below. Default value is `["method",
	// "path", "response_code", "api_key", "time_stamp", "api_version", "api_name", "api_id",
	// "org_id", "oauth_id", "raw_request", "request_time", "raw_response", "ip_address"]`.
	Fields []string `mapstructure:"fields" json:"fields"`
	// List of tags to be added to the metric.
	Tags []string `mapstructure:"tags" json:"tags"`
	// Flush data to InfluxDB2 as soon as the pump receives it
	Flush bool `mapstructure:"flush" json:"flush"`
}

func (i *Influx2Pump) New() Pump {
	newPump := Influx2Pump{}
	return &newPump
}

func (i *Influx2Pump) GetName() string {
	return "InfluxDB2 Pump"
}

func (i *Influx2Pump) GetEnvPrefix() string {
	return i.dbConf.EnvPrefix
}

func (i *Influx2Pump) Init(config interface{}) error {
	i.dbConf = &Influx2Conf{}
	i.log = log.WithField("prefix", influx2Prefix)

	err := mapstructure.Decode(config, &i.dbConf)
	if err != nil {
		i.log.Fatal("Failed to decode configuration: ", err)
	}

	processPumpEnvVars(i, i.log, i.dbConf, influx2DefaultENV)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	i.client = i.connect()
	rdy, err := i.client.Ready(ctx)
	if err != nil {
		return err
	}
	if *rdy.Status != domain.ReadyStatusReady {
		return fmt.Errorf("InfluxDB2 server is not ready: %s", *rdy.Status)
	}

	org, err := i.client.OrganizationsAPI().FindOrganizationByName(ctx, i.dbConf.OrgName)
	if err != nil {
		return fmt.Errorf("error looking up InfluxDB2 organization: %v", err)
	}
	i.log.Debugf(
		"InfluxDB2 found organization for name %s with ID: %s",
		i.dbConf.OrgName,
		*org.Id,
	)

	bucket, err := i.client.BucketsAPI().FindBucketByName(ctx, i.dbConf.BucketName)
	if err != nil {
		return fmt.Errorf("error looking up InfluxDB2 bucket: %v", err)
	}
	i.log.Debugf(
		"InfluxDB2 found bucket for name %s with ID: %s",
		i.dbConf.BucketName,
		*bucket.Id,
	)

	i.log.Debug("InfluxDB2 CS: ", i.dbConf.Addr)
	i.log.Info(i.GetName() + " Initialized")

	return nil
}

func (i *Influx2Pump) Shutdown() error {
	i.client.WriteAPI(i.dbConf.OrgName, i.dbConf.BucketName).Flush()
	i.client.Close()
	return nil
}

func (i *Influx2Pump) connect() influxdb2.Client {
	opts := influxdb2.DefaultOptions()
	opts = opts.SetPrecision(time.Microsecond)
	return influxdb2.NewClientWithOptions(i.dbConf.Addr, i.dbConf.Token, opts)
}

func (i *Influx2Pump) WriteData(ctx context.Context, data []interface{}) error {
	i.log.Debug("Attempting to write ", len(data), " records...")

	writeApi := i.client.WriteAPI(i.dbConf.OrgName, i.dbConf.BucketName)

	for _, v := range data {
		ar := v.(analytics.AnalyticsRecord)
		mapping := map[string]interface{}{
			"method":        ar.Method,
			"path":          ar.Path,
			"response_code": ar.ResponseCode,
			"api_key":       ar.APIKey,
			"time_stamp":    ar.TimeStamp,
			"api_version":   ar.APIVersion,
			"api_name":      ar.APIName,
			"api_id":        ar.APIID,
			"org_id":        ar.OrgID,
			"oauth_id":      ar.OauthID,
			"raw_request":   ar.RawRequest,
			"request_time":  ar.RequestTime,
			"raw_response":  ar.RawResponse,
			"ip_address":    ar.IPAddress,
		}
		tags := make(map[string]string)
		fields := make(map[string]interface{})

		var tag string
		// Select tags from config
		for _, t := range i.dbConf.Tags {
			b, err := json.Marshal(mapping[t])
			if err != nil {
				tag = ""
			} else {
				// convert and remove surrounding quotes from tag value
				tag = strings.Trim(string(b), `"`)
			}
			tags[t] = tag
		}

		// Select field from config
		for _, f := range i.dbConf.Fields {
			fields[f] = mapping[f]
		}

		// Add a new Point for the InfluxDB2 client to batch and send soon
		writeApi.WritePoint(
			influxdb2.NewPoint(influx2Measurement, tags, fields, time.Now()),
		)
	}

	// Flush the InfluxDB2 client's send queue if configured to do so
	if i.dbConf.Flush {
		writeApi.Flush()
	}
	i.log.Info("Purged ", len(data), " records...")

	return nil
}
