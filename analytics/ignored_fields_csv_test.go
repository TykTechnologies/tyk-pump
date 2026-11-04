package analytics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ignoredFieldsBaseRecord() AnalyticsRecord {
	return AnalyticsRecord{
		Method:       "GET",
		Host:         "example.com",
		Path:         "/path",
		RawPath:      "/rawpath",
		UserAgent:    "agent",
		Day:          15,
		Month:        time.March,
		Year:         2026,
		Hour:         10,
		ResponseCode: 200,
		APIKey:       "key123",
		TimeStamp:    time.Date(2026, time.March, 15, 10, 0, 0, 0, time.UTC),
		APIVersion:   "v1",
		APIName:      "api_name",
		APIID:        "api123",
		OrgID:        "org123",
		RequestTime:  42,
		IPAddress:    "127.0.0.1",
		Tags:         []string{"tag1"},
		Alias:        "alias",
		ExpireAt:     time.Date(2026, time.April, 15, 10, 0, 0, 0, time.UTC),
		ApiSchema:    "http",
		OriginalPath: "/original",
		ListenPath:   "/listen",
	}
}

// TT-10871: ignoring a field changes only that field's CSV cell, and the row
// stays aligned with the header. Ignored non-month fields keep writing their
// zero value; blanking those is a separate change.
func TestAnalyticsRecord_GetLineValues_IgnoredFieldOutputUnchanged(t *testing.T) {
	const zeroTime = "0001-01-01 00:00:00 +0000 UTC"

	tcs := []struct {
		want   map[string]string // header -> expected cell after ignore
		name   string
		ignore []string
	}{
		{name: "day", ignore: []string{"day"}, want: map[string]string{"Day": "0"}},
		{name: "year", ignore: []string{"year"}, want: map[string]string{"Year": "0"}},
		{name: "hour", ignore: []string{"hour"}, want: map[string]string{"Hour": "0"}},
		{name: "api_key", ignore: []string{"api_key"}, want: map[string]string{"APIKey": ""}},
		{name: "timestamp", ignore: []string{"timestamp"}, want: map[string]string{"TimeStamp": zeroTime}},
		{name: "expireAt", ignore: []string{"expireAt"}, want: map[string]string{"ExpireAt": zeroTime}},
		{name: "month", ignore: []string{"month"}, want: map[string]string{"Month": ""}},
		{
			name:   "month with other fields",
			ignore: []string{"month", "day", "year", "hour", "api_key", "timestamp"},
			want: map[string]string{
				"Month": "", "Day": "0", "Year": "0", "Hour": "0", "APIKey": "", "TimeStamp": zeroTime,
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			base := ignoredFieldsBaseRecord()
			before := base.GetLineValues()

			rec := ignoredFieldsBaseRecord()
			rec.RemoveIgnoredFields(tc.ignore)

			names := rec.GetFieldNames()
			after := rec.GetLineValues()
			require.Len(t, before, len(names), "row must stay aligned with header before ignore")
			require.Len(t, after, len(names), "row must stay aligned with header after ignore")

			for header, want := range tc.want {
				assert.Equal(t, want, csvCellForHeader(t, &rec, header), "ignored column %q", header)
			}

			for i, name := range names {
				if _, ignored := tc.want[name]; ignored {
					continue
				}
				assert.Equal(t, before[i], after[i], "non-ignored column %q must be unchanged", name)
			}

			for i, cell := range after {
				assert.NotContains(t, cell, "%!", "column %q holds a fmt artifact", names[i])
			}
		})
	}
}
