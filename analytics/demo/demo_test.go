package demo

import (
	"errors"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/gocraft/health"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
)

type failingUUIDGenerator struct{}

func (failingUUIDGenerator) NewV1() (uuid.UUID, error) {
	return uuid.Nil, nil
}

func (failingUUIDGenerator) NewV3(ns uuid.UUID, name string) uuid.UUID {
	return uuid.NewV3(ns, name)
}

func (failingUUIDGenerator) NewV4() (uuid.UUID, error) {
	return uuid.Nil, errors.New("uuid entropy unavailable")
}

func (failingUUIDGenerator) NewV5(ns uuid.UUID, name string) uuid.UUID {
	return uuid.NewV5(ns, name)
}

// Verifies: SW-REQ-009
func TestGenerateDemoData(t *testing.T) {
	type args struct {
		writer         func([]interface{}, *health.Job, time.Time, int)
		orgID          string
		days           int
		recordsPerHour int
		trackPath      bool
		futureData     bool
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "generating demo data for 1 day, 1 record per hour -> 24 records",
			args: args{
				days:           1,
				recordsPerHour: 1,
				orgID:          "test",
				trackPath:      false,
				futureData:     true,
				writer: func(data []interface{}, job *health.Job, ts time.Time, n int) {
				},
			},
		},
		{
			name: "generating demo data for 2 days, 1 record per hour -> 48 records",
			args: args{
				days:           2,
				recordsPerHour: 1,
				orgID:          "test",
				trackPath:      true,
				writer:         func([]interface{}, *health.Job, time.Time, int) {},
			},
		},
		{
			name: "generating demo data for 1 day, 2 records per hour -> 48 records",
			args: args{
				days:           1,
				recordsPerHour: 2,
				orgID:          "test",
				trackPath:      false,
				writer:         func([]interface{}, *health.Job, time.Time, int) {},
			},
		},
		{
			name: "generating demo data for 2 days, 2 records per hour -> 96 records",
			args: args{
				days:           2,
				recordsPerHour: 2,
				orgID:          "test",
				trackPath:      true,
				writer:         func([]interface{}, *health.Job, time.Time, int) {},
			},
		},
		{
			name: "generating demo data for 0 days, 100 records per hour -> 0 records",
			args: args{
				days:           0,
				recordsPerHour: 100,
				orgID:          "test",
				trackPath:      false,
				writer:         func([]interface{}, *health.Job, time.Time, int) {},
			},
		},
		{
			name: "generating demo data for 1 day, 0 records per hour -> 0 records",
			args: args{
				days:           1,
				recordsPerHour: 0,
				orgID:          "test",
				trackPath:      true,
				writer:         func([]interface{}, *health.Job, time.Time, int) {},
			},
		},
		{
			name: "generating demo data for 10 days, from 300 to 500 records per hour",
			args: args{
				days:           10,
				recordsPerHour: 0,
				orgID:          "test",
				trackPath:      false,
				writer:         func([]interface{}, *health.Job, time.Time, int) {},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := 0
			tt.args.writer = func(data []interface{}, job *health.Job, ts time.Time, n int) {
				counter += len(data)
				for _, d := range data {
					analyticsRecord, ok := d.(analytics.AnalyticsRecord)
					if !ok {
						t.Errorf("unexpected type: %T", d)
					}
					// checking timestamp:
					// if futureData is true, then timestamp should be in the present and future
					// if futureData is false, then timestamp should be in the past
					ts := time.Now()
					if tt.args.futureData {
						val := analyticsRecord.TimeStamp.After(time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, time.UTC)) || analyticsRecord.TimeStamp.Equal(time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, time.UTC))
						assert.True(t, val)
					} else {
						assert.True(t, analyticsRecord.TimeStamp.Before(time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, time.UTC)))
					}
					assert.Equal(t, tt.args.trackPath, analyticsRecord.TrackPath)
				}
			}

			GenerateDemoData(tt.args.days, tt.args.recordsPerHour, tt.args.orgID, tt.args.futureData, tt.args.trackPath, tt.args.writer)
			if tt.args.recordsPerHour == 0 {
				isValid := counter >= 300*tt.args.days || counter <= 500*tt.args.days
				assert.True(t, isValid)
				return
			}
			assert.Equal(t, tt.args.days*24*tt.args.recordsPerHour, counter)
		})
	}
}

// Verifies: SW-REQ-009
func TestDemoInitDefaultsVersion(t *testing.T) {
	DemoInit("org", "api", "")

	assert.Equal(t, "api", apiID)
	assert.Equal(t, "Default", apiVersion)
	assert.Len(t, apiKeys, 50)
}

// Verifies: SW-REQ-009
func TestDemoInitPreservesVersion(t *testing.T) {
	DemoInit("org", "api", "v1")

	assert.Equal(t, "api", apiID)
	assert.Equal(t, "v1", apiVersion)
	assert.Len(t, apiKeys, 50)
}

// Verifies: SW-REQ-009
func TestRandomAPIUsesConfiguredID(t *testing.T) {
	previousAPIID := apiID
	t.Cleanup(func() {
		apiID = previousAPIID
	})

	apiID = "configured-api"

	name, id := randomAPI()

	assert.Equal(t, "Foo Bar", name)
	assert.Equal(t, "configured-api", id)
}

// Verifies: SW-REQ-009
func TestGenerateAPIKeyLogsUUIDErrorAndUsesNilUUID(t *testing.T) {
	previousGenerator := uuid.DefaultGenerator
	uuid.DefaultGenerator = failingUUIDGenerator{}
	t.Cleanup(func() {
		uuid.DefaultGenerator = previousGenerator
	})

	assert.Equal(t, "org00000000000000000000000000000000", generateAPIKey("org"))
}
