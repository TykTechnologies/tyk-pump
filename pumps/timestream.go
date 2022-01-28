package pumps

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/timestreamwrite"
	"github.com/aws/aws-sdk-go-v2/service/timestreamwrite/types"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/net/http2"
)

type TimestreamWriteRecordsAPI interface {
	WriteRecords(ctx context.Context, params *timestreamwrite.WriteRecordsInput, optFns ...func(*timestreamwrite.Options)) (*timestreamwrite.WriteRecordsOutput, error)
}

type TimestreamPump struct {
	client TimestreamWriteRecordsAPI
	config *TimestreamPumpConf
	CommonPumpConfig
}

const (
	timestreamPumpPrefix       = "timestream-pump"
	timestreamPumpName         = "Timestream Pump"
	timestreamDefaultEnv       = PUMPS_ENV_PREFIX + "_TIMESTREAM" + PUMPS_ENV_META_PREFIX
	timestreamVarcharMaxLength = 2048 //https://docs.aws.amazon.com/timestream/latest/developerguide/writes.html
	timestreamMaxRecordsCount  = 100  //https://docs.aws.amazon.com/timestream/latest/developerguide/API_WriteRecords.html
)

type TimestreamPumpConf struct {
	EnvPrefix    string   `mapstructure:"meta_env_prefix"`
	AWSRegion    string   `mapstructure:"aws_region"`
	TableName    string   `mapstructure:"timestream_table_name"`
	DatabaseName string   `mapstructure:"timestream_database_name"`
	Dimensions   []string `mapstructure:"dimensions"`
	Measures     []string `mapstructure:"measures"`
}

func (t *TimestreamPump) New() Pump {
	newPump := TimestreamPump{}
	return &newPump
}

func (t *TimestreamPump) GetName() string {
	return timestreamPumpName
}

func (t *TimestreamPump) GetEnvPrefix() string {
	return t.config.EnvPrefix
}

func (t *TimestreamPump) Init(config interface{}) error {
	t.config = &TimestreamPumpConf{}
	t.log = log.WithField("prefix", timestreamPumpPrefix)

	err := mapstructure.Decode(config, &t.config)
	if err != nil {
		t.log.Fatal("Failed to decode configuration: ", err)
		return err
	}

	processPumpEnvVars(t, t.log, t.config, timestreamDefaultEnv)

	if len(t.config.Measures) == 0 || len(t.config.Dimensions) == 0 {
		return errors.New("missing \"measures\" or \"dimensions\" in pump configuration")
	}

	t.client, err = NewTimesteramWriter(t.config.AWSRegion)
	if err != nil {
		return err
	}
	t.log.Info(t.GetName() + " Initialized")

	return nil
}

func (t *TimestreamPump) WriteData(ctx context.Context, data []interface{}) error {
	t.log.Debug("Attempting to write ", len(data), " records...")

	var records []types.Record

	for next, hasNext := BuildTimestreamInputIterator(t, data); hasNext; {
		records, hasNext = next()
		_, err := t.client.WriteRecords(ctx, &timestreamwrite.WriteRecordsInput{
			DatabaseName: aws.String(t.config.DatabaseName),
			TableName:    aws.String(t.config.TableName),
			Records:      records,
		})
		if err != nil {
			t.log.Errorf("Error writing data to Timestream %+v", err)
			return err
		}
	}

	t.log.Info("Purged ", len(data), " records...")

	return nil
}

func BuildTimestreamInputIterator(t *TimestreamPump, data []interface{}) (func() (records []types.Record, hasNext bool), bool) {
	curr := -1
	max := int(math.Ceil((float64(len(data)) / float64(timestreamMaxRecordsCount)))) - 1

	next := func() (records []types.Record, hasNext bool) {
		curr++
		records = make([]types.Record, 0, timestreamMaxRecordsCount)

		for i := curr * timestreamMaxRecordsCount; i < Min(timestreamMaxRecordsCount*(curr+1), len(data)); i++ {
			decoded := data[i].(analytics.AnalyticsRecord)
			multimeasureRecord := MapAnalyticRecord2TimestreamMultimeasureRecord(t, &decoded)
			records = append(records, multimeasureRecord)
		}
		return records, curr < max
	}
	return next, curr < max
}

func MapAnalyticRecord2TimestreamMultimeasureRecord(t *TimestreamPump, decoded *analytics.AnalyticsRecord) types.Record {
	timestramDimensions := GetAnalyticsRecordDimensions(t, decoded)
	timestreamMeasures := GetAnalyticsRecordMeasures(t, decoded)
	multimeasureRecord := types.Record{
		Dimensions:       timestramDimensions,
		MeasureName:      aws.String("request_metrics"),
		MeasureValueType: types.MeasureValueTypeMulti,
		MeasureValues:    timestreamMeasures,
		Time:             aws.String(strconv.FormatInt(decoded.TimeStamp.UnixMilli(), 10)),
		TimeUnit:         types.TimeUnitMilliseconds,
	}
	return multimeasureRecord
}

func GetAnalyticsRecordMeasures(t *TimestreamPump, decoded *analytics.AnalyticsRecord) (measureValues []types.MeasureValue) {

	measureFieldsMapping := map[string]types.MeasureValue{
		"GeoData.City.GeoNameID": {
			Name:  aws.String("GeoData.City.GeoNameID"),
			Value: aws.String(strconv.FormatUint(uint64(decoded.Geo.City.GeoNameID), 10)),
			Type:  types.MeasureValueTypeBigint,
		},
		"GeoData.Location.Latitude": {
			Name:  aws.String("GeoData.Location.Latitude"),
			Value: aws.String(strconv.FormatFloat(decoded.Geo.Location.Latitude, 'f', -1, 64)),
			Type:  types.MeasureValueTypeDouble,
		},
		"GeoData.Location.Longitude": {
			Name:  aws.String("GeoData.Location.Longitude"),
			Value: aws.String(strconv.FormatFloat(decoded.Geo.Location.Longitude, 'f', -1, 64)),
			Type:  types.MeasureValueTypeDouble,
		},
	}

	var intMeasures = map[string]int64{
		"ContentLength":                 decoded.ContentLength,
		"ResponseCode":                  int64(decoded.ResponseCode),
		"RequestTime":                   decoded.RequestTime,
		"NetworkStats.OpenConnections":  decoded.Network.OpenConnections,
		"NetworkStats.ClosedConnection": decoded.Network.ClosedConnection,
		"NetworkStats.BytesIn":          decoded.Network.BytesIn,
		"NetworkStats.BytesOut":         decoded.Network.BytesOut,
		"Latency.Total":                 decoded.Latency.Total,
		"Latency.Upstream":              decoded.Latency.Upstream,
	}
	for key, value := range intMeasures {
		measureFieldsMapping[key] = types.MeasureValue{
			Name:  aws.String(key),
			Value: aws.String(strconv.FormatInt(value, 10)),
			Type:  types.MeasureValueTypeBigint,
		}
	}

	var stringMeasures = map[string]string{
		"UserAgent":                 decoded.UserAgent,
		"RawRequest":                decoded.RawRequest,
		"IPAddress":                 decoded.IPAddress,
		"GeoData.Country.ISOCode":   decoded.Geo.Country.ISOCode,
		"GeoData.City.Names":        mapToVarChar(decoded.Geo.City.Names),
		"GeoData.Location.TimeZone": decoded.Geo.Location.TimeZone,
	}

	//timestream can't ingest empty strings
	for key, value := range stringMeasures {
		if value != "" {
			measureFieldsMapping[key] = types.MeasureValue{
				Name:  aws.String(key),
				Value: aws.String(value),
				Type:  types.MeasureValueTypeVarchar,
			}
		}
	}

	var includeRawResponse = false //special case raw response

	//filter measures according to config
	for _, key := range t.config.Measures {
		includeRawResponse = includeRawResponse || key == "RawResponse"
		//skip if configuration key not present in measure fields
		if value, ok := measureFieldsMapping[key]; ok {
			measureValues = append(measureValues, value)
		}
	}

	//rawResponse needs special treatment because timestream varchar has a 2KB size limit
	if includeRawResponse {
		chunks := chunkString(decoded.RawResponse, timestreamVarcharMaxLength)

		measureValues = append(measureValues, types.MeasureValue{
			Name:  aws.String("RawResponseSize"),
			Value: aws.String(strconv.FormatInt(int64(len(chunks)), 10)),
			Type:  types.MeasureValueTypeBigint,
		})
		for i, chunk := range chunks {
			name := fmt.Sprintf("RawResponse%s", strconv.FormatUint(uint64(i), 10))
			measureValues = append(measureValues, types.MeasureValue{
				Name:  aws.String(name),
				Value: aws.String(chunk),
				Type:  types.MeasureValueTypeVarchar,
			})
		}
	}

	return measureValues
}
func Min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

func chunkString(chars string, chunkSize int) []string {
	if chunkSize <= 0 {
		return []string{chars}
	}

	chunkCount := int(math.Ceil((float64(len(chars)) / float64(chunkSize))))
	output := make([]string, chunkCount)

	for i := 0; i < chunkCount; i++ {
		output[i] = chars[i*chunkSize : Min(i*chunkSize+chunkSize, len(chars))]
	}
	return output
}

func mapToVarChar(dictionary map[string]string) string {
	keys := make([]string, 0, len(dictionary))
	for k := range dictionary {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var output string
	first := true
	for _, key := range keys {
		keyval := dictionary[key]
		if first {
			first = false
			output = fmt.Sprintf("%s:%s", key, keyval)
		} else {
			output = fmt.Sprintf("%s;%s:%s", output, key, keyval)
		}
	}
	return output
}

func GetAnalyticsRecordDimensions(t *TimestreamPump, decoded *analytics.AnalyticsRecord) (dimensions []types.Dimension) {

	var dimensionFields = map[string]string{
		"Method":     decoded.Method,
		"Host":       decoded.Host,
		"Path":       decoded.Path,
		"RawPath":    decoded.RawPath,
		"APIKey":     decoded.APIKey,
		"APIVersion": decoded.APIVersion,
		"APIName":    decoded.APIName,
		"APIID":      decoded.APIID,
		"OrgID":      decoded.OrgID,
		"OauthID":    decoded.OauthID,
	}

	for key, value := range dimensionFields {
		//timestream can't ingest empty strings
		if value == "" {
			delete(dimensionFields, key)
		}
	}
	dimensions = make([]types.Dimension, 0, len(dimensionFields))

	//filter dimensions according to config
	for _, key := range t.config.Dimensions {
		//skip if configuration key not present in dimension fields
		if value, ok := dimensionFields[key]; ok {
			dimensions = append(dimensions, types.Dimension{
				Name:               aws.String(key),
				Value:              aws.String(value),
				DimensionValueType: types.DimensionValueTypeVarchar,
			})
		}
	}

	return dimensions
}

func NewTimesteramWriter(awsRegion string) (c *timestreamwrite.Client, err error) {
	tr := &http.Transport{
		ResponseHeaderTimeout: 20 * time.Second,
		// Using DefaultTransport values for other parameters: https://golang.org/pkg/net/http/#RoundTripper
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			KeepAlive: 30 * time.Second,
			DualStack: true,
			Timeout:   30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	// So client makes HTTP/2 requests
	http2.ConfigureTransport(tr)

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithHTTPClient(&http.Client{
			Transport: tr,
			Timeout:   30 * time.Second,
		}),
		config.WithRegion(awsRegion),
		config.WithRetryer(func() aws.Retryer {
			return retry.AddWithMaxAttempts(retry.NewStandard(), 10)
		}))
	if err != nil {
		return nil, err
	}
	writeSvc := timestreamwrite.NewFromConfig(cfg)
	return writeSvc, nil
}
