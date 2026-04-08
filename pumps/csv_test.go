package pumps

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics/demo"
	"github.com/stretchr/testify/assert"
)

func TestCSVPump_New(t *testing.T) {
	type fields struct {
		csvConf          *CSVConf
		CommonPumpConfig CommonPumpConfig
		wroteHeaders     bool
	}
	tests := []struct {
		want   Pump
		name   string
		fields fields
	}{
		{
			name: "TestCSVPump_New",
			fields: fields{
				csvConf: &CSVConf{},
				CommonPumpConfig: CommonPumpConfig{
					log: log.WithField("prefix", csvPrefix),
				},
			},
			want: &CSVPump{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CSVPump{
				csvConf:          tt.fields.csvConf,
				wroteHeaders:     tt.fields.wroteHeaders,
				CommonPumpConfig: tt.fields.CommonPumpConfig,
			}
			if got := c.New(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CSVPump.New() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCSVPump_GetName(t *testing.T) {
	type fields struct {
		csvConf          *CSVConf
		CommonPumpConfig CommonPumpConfig
		wroteHeaders     bool
	}
	tests := []struct {
		name   string
		want   string
		fields fields
	}{
		{
			name: "TestCSVPump_GetName",
			fields: fields{
				csvConf: &CSVConf{},
				CommonPumpConfig: CommonPumpConfig{
					log: log.WithField("prefix", csvPrefix),
				},
			},
			want: "CSV Pump",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CSVPump{
				csvConf:          tt.fields.csvConf,
				wroteHeaders:     tt.fields.wroteHeaders,
				CommonPumpConfig: tt.fields.CommonPumpConfig,
			}
			if got := c.GetName(); got != tt.want {
				t.Errorf("CSVPump.GetName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCSVPump_Init(t *testing.T) {
	type fields struct {
		csvConf          *CSVConf
		CommonPumpConfig CommonPumpConfig
		wroteHeaders     bool
	}
	type args struct {
		conf interface{}
	}
	tests := []struct {
		args    args
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "TestCSVPump_Init",
			args: args{
				conf: &CSVConf{
					CSVDir: "testingDirectory",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CSVPump{
				csvConf:          tt.fields.csvConf,
				wroteHeaders:     tt.fields.wroteHeaders,
				CommonPumpConfig: tt.fields.CommonPumpConfig,
			}
			if err := c.Init(tt.args.conf); (err != nil) != tt.wantErr {
				t.Errorf("CSVPump.Init() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			defer os.Remove(c.csvConf.CSVDir)

			_, err := os.Stat(c.csvConf.CSVDir)
			if err != nil {
				t.Errorf("CSVPump.Init() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCSVPump_WriteData(t *testing.T) {
	type fields struct {
		csvConf      *CSVConf
		wroteHeaders bool
	}
	type args struct {
		numberOfRecords int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "writing 1 record",
			fields: fields{
				csvConf: &CSVConf{
					CSVDir: "testingDirectory",
				},
			},
			args: args{
				numberOfRecords: 1,
			},
		},
		{
			name: "writing 10 records",
			fields: fields{
				csvConf: &CSVConf{
					CSVDir: "testingDirectory",
				},
			},
			args: args{
				numberOfRecords: 10,
			},
		},
		{
			name: "trying to write invalid records",
			fields: fields{
				csvConf: &CSVConf{
					CSVDir: "testingDirectory",
				},
			},
			args: args{
				numberOfRecords: 0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// creating and initializing csv pump
			c := &CSVPump{
				csvConf:      tt.fields.csvConf,
				wroteHeaders: tt.fields.wroteHeaders,
			}
			err := c.Init(tt.fields.csvConf)
			assert.Nil(t, err)
			// when init, a directory is created, so we need to remove it after the test
			defer os.RemoveAll(c.csvConf.CSVDir)
			// generating random data to write in the csv file
			var records []interface{}
			if !tt.wantErr {
				for i := 0; i < tt.args.numberOfRecords; i++ {
					records = append(records, demo.GenerateRandomAnalyticRecord("orgid", false))
				}
			} else {
				records = append(records, "invalid record")
			}
			// writing data
			if err := c.WriteData(context.Background(), records); (err != nil) != tt.wantErr {
				t.Errorf("CSVPump.WriteData() error = %v, wantErr %v", err, tt.wantErr)
			}

			// getting the file name
			curtime := time.Now()
			fname := fmt.Sprintf("%d-%s-%d-%d.csv", curtime.Year(), curtime.Month().String(), curtime.Day(), curtime.Hour())
			file, totalRows, err := GetFileAndRows(fname)
			assert.Nil(t, err)
			defer file.Close()

			if tt.wantErr {
				assert.Equal(t, tt.args.numberOfRecords, totalRows)
				return
			}
			assert.Equal(t, tt.args.numberOfRecords+1, totalRows)

			// trying to append data to an existing file
			if err := c.WriteData(context.Background(), records); (err != nil) != tt.wantErr {
				t.Errorf("CSVPump.WriteData() error = %v, wantErr %v", err, tt.wantErr)
			}

			file, totalRows, err = GetFileAndRows(fname)
			assert.Nil(t, err)
			defer file.Close()
			assert.Equal(t, tt.args.numberOfRecords*2+1, totalRows)
		})
	}
}

func GetFileAndRows(fname string) (*os.File, int, error) {
	// checking if the file exists
	openfile, err := os.Open("./testingDirectory/" + fname)
	if err != nil {
		return nil, 0, err
	}
	defer openfile.Close()
	filedata, err := csv.NewReader(openfile).ReadAll()
	if err != nil {
		return nil, 0, err
	}

	// checking if the file contains the right number of records (number of records +1 because of the header)
	totalRows := len(filedata)
	return openfile, totalRows, nil
}
