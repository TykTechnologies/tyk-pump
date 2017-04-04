package pumps

import (
	"encoding/csv"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
)

type CSVPump struct {
	csvConf      *CSVConf
	wroteHeaders bool
}

type CSVConf struct {
	CSVDir string `mapstructure:"csv_dir"`
}

var csvPrefix = "csv-pump"

func (c *CSVPump) New() Pump {
	newPump := CSVPump{}
	return &newPump
}

func (c *CSVPump) GetName() string {
	return "CSV Pump"
}

func (c *CSVPump) Init(conf interface{}) error {
	c.csvConf = &CSVConf{}
	err := mapstructure.Decode(conf, &c.csvConf)

	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Fatal("Failed to decode configuration: ", err)
	}

	ferr := os.MkdirAll(c.csvConf.CSVDir, 0777)
	if ferr != nil {
		log.WithFields(logrus.Fields{
			"prefix": csvPrefix,
		}).Error(ferr)
	}

	log.WithFields(logrus.Fields{
		"prefix": csvPrefix,
	}).Debug("CSV Initialized")
	return nil
}

func (c *CSVPump) WriteData(data []interface{}) error {
	curtime := time.Now()
	fname := fmt.Sprintf("%d-%s-%d-%d.csv", curtime.Year(), curtime.Month().String(), curtime.Day(), curtime.Hour())
	fname = path.Join(c.csvConf.CSVDir, fname)

	var outfile *os.File
	var appendHeader bool

	if _, err := os.Stat(fname); os.IsNotExist(err) {
		var createErr error
		outfile, createErr = os.Create(fname)
		if createErr != nil {
			log.WithFields(logrus.Fields{
				"prefix": csvPrefix,
			}).Error("Failed to create new CSV file: ", createErr)
		}
		appendHeader = true
	} else {
		var appendErr error
		outfile, appendErr = os.OpenFile(fname, os.O_APPEND|os.O_WRONLY, 0600)
		if appendErr != nil {
			log.WithFields(logrus.Fields{
				"prefix": csvPrefix,
			}).Error("Failed to open CSV file: ", appendErr)
		}
	}

	defer outfile.Close()
	writer := csv.NewWriter(outfile)

	if appendHeader {
		startRecord := analytics.AnalyticsRecord{}
		var headers = startRecord.GetFieldNames()

		err := writer.Write(headers)
		if err != nil {
			log.WithFields(logrus.Fields{
				"prefix": csvPrefix,
			}).Error("Failed to write file headers: ", err)
			return err

		}
	}

	for _, v := range data {
		decoded := v.(analytics.AnalyticsRecord)

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
			log.Error("File write failed!")
			log.Error(err)
		}

	}
	writer.Flush()
	return nil
}
