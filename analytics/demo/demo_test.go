package demo

import (
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/gocraft/health"
	"github.com/stretchr/testify/assert"
)

func TestGenerateDemoData(t *testing.T) {
	type args struct {
		writer         func([]interface{}, *health.Job, time.Time, int)
		orgID          string
		days           int
		hours          int
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
		{
			name: "generating demo data for 5 hours, 10 records per hour -> 50 records",
			args: args{
				hours:          5,
				recordsPerHour: 10,
				orgID:          "test",
				trackPath:      false,
				futureData:     false,
				writer:         func([]interface{}, *health.Job, time.Time, int) {},
			},
		},
		{
			name: "generating demo data for 12 hours (future), 5 records per hour -> 60 records",
			args: args{
				hours:          12,
				recordsPerHour: 5,
				orgID:          "test",
				trackPath:      true,
				futureData:     true,
				writer:         func([]interface{}, *health.Job, time.Time, int) {},
			},
		},
		{
			name: "hours overrides days: 3 hours, 2 days set, 1 record per hour -> 3 records",
			args: args{
				days:           2,
				hours:          3,
				recordsPerHour: 1,
				orgID:          "test",
				trackPath:      false,
				futureData:     false,
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
					now := time.Now().UTC()
					var hourStart int
					if tt.args.hours > 0 {
						hourStart = now.Hour() // Compare against the start of the current hour
					}
					compareTime := time.Date(now.Year(), now.Month(), now.Day(), hourStart, 0, 0, 0, time.UTC)

					if tt.args.futureData {
						// Future data should be >= the start of current period
						val := analyticsRecord.TimeStamp.After(compareTime) || analyticsRecord.TimeStamp.Equal(compareTime)
						assert.True(t, val)
					} else {
						// Past data should be < the start of current period
						assert.True(t, analyticsRecord.TimeStamp.Before(compareTime))
					}
					assert.Equal(t, tt.args.trackPath, analyticsRecord.TrackPath)
				}
			}

			GenerateDemoData(tt.args.days, tt.args.hours, tt.args.recordsPerHour, tt.args.orgID, tt.args.futureData, tt.args.trackPath, tt.args.writer)

			// If hours is set, calculate expected count based on hours
			if tt.args.hours > 0 {
				if tt.args.recordsPerHour == 0 {
					isValid := counter >= 300*tt.args.hours || counter <= 500*tt.args.hours
					assert.True(t, isValid)
					return
				}
				assert.Equal(t, tt.args.hours*tt.args.recordsPerHour, counter)
				return
			}

			// Otherwise use days logic
			if tt.args.recordsPerHour == 0 {
				isValid := counter >= 300*tt.args.days || counter <= 500*tt.args.days
				assert.True(t, isValid)
				return
			}
			assert.Equal(t, tt.args.days*24*tt.args.recordsPerHour, counter)
		})
	}
}
