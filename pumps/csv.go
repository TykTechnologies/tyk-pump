package pumps

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

type CSVPump struct {
	csvConf      *CSVConf
	wroteHeaders bool
	CommonPumpConfig
}

// @PumpConf CSV
type CSVConf struct {
	EnvPrefix string `mapstructure:"meta_env_prefix"`
	// The directory and the filename where the CSV data will be stored.
	CSVDir string `json:"csv_dir" mapstructure:"csv_dir"`
}

var csvPrefix = "csv-pump"
var csvDefaultENV = PUMPS_ENV_PREFIX + "_CSV" + PUMPS_ENV_META_PREFIX

func (c *CSVPump) New() Pump {
	newPump := CSVPump{}
	return &newPump
}

func (c *CSVPump) GetName() string {
	return "CSV Pump"
}

func (c *CSVPump) GetEnvPrefix() string {
	return c.csvConf.EnvPrefix
}

func (c *CSVPump) Init(conf interface{}) error {
	c.csvConf = &CSVConf{}
	c.log = log.WithField("prefix", csvPrefix)

	err := mapstructure.Decode(conf, &c.csvConf)
	if err != nil {
		c.log.Fatal("Failed to decode configuration: ", err)
	}

	processPumpEnvVars(c, c.log, c.csvConf, csvDefaultENV)

	ferr := os.MkdirAll(c.csvConf.CSVDir, 0777)
	if ferr != nil {
		c.log.Error(ferr.Error() + " dir: " + c.csvConf.CSVDir)
	}

	c.log.Info(c.GetName() + " Initialized")
	return nil
}

func (c *CSVPump) WriteData(ctx context.Context, data []interface{}) error {
	c.log.Debug("Attempting to write ", len(data), " records...")

	curtime := time.Now()
	fname := fmt.Sprintf("%d-%s-%d-%d.csv", curtime.Year(), curtime.Month().String(), curtime.Day(), curtime.Hour())
	fname = path.Join(c.csvConf.CSVDir, fname)

	var outfile *os.File
	var appendHeader bool

	if _, err := os.Stat(fname); os.IsNotExist(err) {
		var createErr error
		outfile, createErr = os.Create(fname)
		if createErr != nil {
			c.log.Error("Failed to create new CSV file: ", createErr)
		}
		appendHeader = true
	} else {
		var appendErr error
		outfile, appendErr = os.OpenFile(fname, os.O_APPEND|os.O_WRONLY, 0600)
		if appendErr != nil {
			c.log.Error("Failed to open CSV file: ", appendErr)
		}
	}

	defer outfile.Close()
	writer := csv.NewWriter(outfile)

	if appendHeader {
		startRecord := analytics.AnalyticsRecord{}
		var headers = startRecord.GetFieldNames()

		err := writer.Write(headers)
		if err != nil {
			c.log.Error("Failed to write file headers: ", err)
			return err

		}
	}

	for _, v := range data {
		decoded, ok := v.(analytics.AnalyticsRecord)
		if !ok {
			return fmt.Errorf("couldn't convert %v to analytics.AnalyticsRecord", v)
		}

		toWrite := decoded.GetLineValues()
		// toWrite := []string{
		// 	decoded.Method,
		// 	decoded.Path,
		// 	strconv.FormatInt(decoded.ContentLength, 10),
		// 	decoded.UserAgent,
		// 	strconv.Itoa(decoded.Day),
		// 	decoded.Month.String(),
		// 	strconv.Itoa(decoded.Year),
		// 	strconv.Itoa(decoded.Hour),
		// 	strconv.Itoa(decoded.ResponseCode),
		// 	decoded.APIName,
		// 	decoded.APIVersion}
		err := writer.Write(toWrite)
		if err != nil {
			c.log.Error("File write failed:", err)
			return err
		}

	}
	writer.Flush()
	c.log.Info("Purged ", len(data), " records...")
	return nil
}
