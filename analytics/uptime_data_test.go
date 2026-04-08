package analytics

import (
	"testing"
	"time"

	"github.com/TykTechnologies/storage/persistent/model"
	"github.com/google/go-cmp/cmp"
	"gorm.io/gorm/clause"

	"github.com/stretchr/testify/assert"
)

func TestUptimeReportData_GetObjectID(t *testing.T) {
	t.Run("should return the ID field", func(t *testing.T) {
		id := model.NewObjectID()
		record := UptimeReportData{
			ID: id,
		}
		assert.Equal(t, id, record.GetObjectID())
	})
}

func TestUptimeReportData_SetObjectID(t *testing.T) {
	t.Run("should set the ID field", func(t *testing.T) {
		id := model.NewObjectID()
		record := UptimeReportData{}
		record.SetObjectID(id)
		assert.Equal(t, id, record.ID)
	})
}

func TestUptimeReportData_TableName(t *testing.T) {
	t.Run("should return the uptime SQL table name", func(t *testing.T) {
		record := UptimeReportData{}
		assert.Equal(t, UptimeSQLTable, record.TableName())
	})
}

func TestUptimeReportAggregateSQL_TableName(t *testing.T) {
	t.Run("should return the uptime aggregate SQL table name", func(t *testing.T) {
		record := UptimeReportAggregateSQL{}
		assert.Equal(t, UptimeSQLTable, record.TableName())
	})
}

func TestUptimeReportAggregate_New(t *testing.T) {
	t.Run("should return a new UptimeReportAggregate", func(t *testing.T) {
		expected := UptimeReportAggregate{}
		expected.URL = make(map[string]*Counter)
		expected.Errors = make(map[string]*Counter)

		actual := UptimeReportAggregate{}.New()

		assert.Equal(t, expected, actual)
	})
}

func TestUptimeReportAggregate_Dimensions(t *testing.T) {
	tcs := []struct {
		testName string
		input    UptimeReportAggregate
		expected []Dimension
	}{
		{
			testName: "should return the dimensions",
			input: UptimeReportAggregate{
				URL: map[string]*Counter{
					"foo": {},
				},
				Errors: map[string]*Counter{
					"bar": {},
				},
				Total: Counter{},
			},
			expected: []Dimension{
				{
					Name:    "url",
					Value:   "foo",
					Counter: &Counter{},
				},
				{
					Name:    "errors",
					Value:   "bar",
					Counter: &Counter{},
				},
				{
					Name:    "",
					Value:   "total",
					Counter: &Counter{},
				},
			},
		},
		{
			testName: "no extra dimensions",
			input:    UptimeReportAggregate{},
			expected: []Dimension{
				{
					Name:    "",
					Value:   "total",
					Counter: &Counter{},
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			actual := tc.input.Dimensions()

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestAggregateUptimeData(t *testing.T) {
	currentTime := time.Date(2023, 0o4, 0o4, 10, 0, 0, 0, time.UTC)

	tcs := []struct {
		testName string
		expected map[string]UptimeReportAggregate
		input    []UptimeReportData
	}{
		{
			testName: "empty input",
			input:    []UptimeReportData{},
			expected: map[string]UptimeReportAggregate{},
		},
		{
			testName: "single record",
			input: []UptimeReportData{
				{
					OrgID:        "org123",
					APIID:        "api123",
					URL:          "/get",
					ResponseCode: 200,
					RequestTime:  100,
					TimeStamp:    currentTime,
					ExpireAt:     currentTime,
				},
			},
			expected: map[string]UptimeReportAggregate{
				"org123": {
					OrgID:     "org123",
					ExpireAt:  currentTime,
					LastTime:  currentTime,
					TimeStamp: currentTime,
					TimeID: struct {
						Year  int
						Month int
						Day   int
						Hour  int
					}{
						Year:  currentTime.Year(),
						Month: int(currentTime.Month()),
						Day:   currentTime.Day(),
						Hour:  currentTime.Hour(),
					},
					URL: map[string]*Counter{
						"/get": {
							Hits:             1,
							TotalRequestTime: 100,
							Success:          1,
							ErrorTotal:       0,
							RequestTime:      100,
							Identifier:       "/get",
							HumanIdentifier:  "",
							LastTime:         currentTime,
							ErrorMap:         map[string]int{"200": 1},
							ErrorList:        []ErrorData{},
						},
					},
					Errors: map[string]*Counter{},
					Total: Counter{
						Hits:             1,
						TotalRequestTime: 100,
						Success:          1,
						ErrorTotal:       0,
						RequestTime:      100,
						Identifier:       "",
						HumanIdentifier:  "",
						ErrorMap:         map[string]int{"200": 1},
					},
				},
			},
		},
		{
			testName: "single record - response code -1",
			input: []UptimeReportData{
				{
					OrgID:        "org123",
					APIID:        "api123",
					URL:          "/get",
					ResponseCode: -1,
					RequestTime:  100,
					TimeStamp:    currentTime,
					ExpireAt:     currentTime,
				},
			},
			expected: map[string]UptimeReportAggregate{
				"org123": {
					OrgID:     "org123",
					ExpireAt:  currentTime,
					LastTime:  currentTime,
					TimeStamp: currentTime,
					TimeID: struct {
						Year  int
						Month int
						Day   int
						Hour  int
					}{
						Year:  currentTime.Year(),
						Month: int(currentTime.Month()),
						Day:   currentTime.Day(),
						Hour:  currentTime.Hour(),
					},
					URL: map[string]*Counter{
						"/get": {
							Identifier: "/get",
						},
					},
					Errors: map[string]*Counter{},
					Total: Counter{
						ErrorMap: map[string]int{},
					},
				},
			},
		},
		{
			testName: "multi record",
			input: []UptimeReportData{
				{
					OrgID:        "org123",
					APIID:        "api123",
					URL:          "/get",
					ResponseCode: 200,
					RequestTime:  100,
					TimeStamp:    currentTime,
					ExpireAt:     currentTime,
				},
				{
					OrgID:        "org123",
					APIID:        "api123",
					URL:          "/get",
					ResponseCode: 200,
					RequestTime:  100,
					TimeStamp:    currentTime,
					ExpireAt:     currentTime,
				},
				{
					OrgID:        "org123",
					APIID:        "api123",
					URL:          "/get",
					ResponseCode: 500,
					RequestTime:  100,
					TimeStamp:    currentTime,
					ExpireAt:     currentTime,
				},
			},
			expected: map[string]UptimeReportAggregate{
				"org123": {
					OrgID:     "org123",
					ExpireAt:  currentTime,
					LastTime:  currentTime,
					TimeStamp: currentTime,
					TimeID: struct {
						Year  int
						Month int
						Day   int
						Hour  int
					}{
						Year:  currentTime.Year(),
						Month: int(currentTime.Month()),
						Day:   currentTime.Day(),
						Hour:  currentTime.Hour(),
					},
					URL: map[string]*Counter{
						"/get": {
							Hits:             3,
							TotalRequestTime: 300,
							Success:          2,
							ErrorTotal:       1,
							RequestTime:      100,
							Identifier:       "/get",
							HumanIdentifier:  "",
							LastTime:         currentTime,
							ErrorMap:         map[string]int{"200": 2, "500": 1},
							ErrorList:        []ErrorData{},
						},
					},
					Errors: map[string]*Counter{
						"500": {
							Hits:             1,
							TotalRequestTime: 100,
							Success:          0,
							ErrorTotal:       1,
							RequestTime:      100,
							Identifier:       "500",
							HumanIdentifier:  "",
							LastTime:         currentTime,
							ErrorMap:         map[string]int{"500": 1},
							ErrorList:        []ErrorData{},
						},
					},
					Total: Counter{
						Hits:             3,
						TotalRequestTime: 300,
						Success:          2,
						ErrorTotal:       1,
						RequestTime:      100,
						Identifier:       "",
						HumanIdentifier:  "",
						ErrorMap:         map[string]int{"200": 2, "500": 1},
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			actual := AggregateUptimeData(tc.input)

			if !cmp.Equal(tc.expected, actual) {
				t.Errorf("AggregateUptimeData() mismatch (-want +got):\n%s", cmp.Diff(tc.expected, actual))
			}
		})
	}
}

func TestOnConflictUptimeAssignments(t *testing.T) {
	assignments := OnConflictAssignments("uptime_reports", "excluded")

	expectedAssignmets := map[string]interface{}{
		"code_1x":                        clause.Expr{SQL: "uptime_reports.code_1x + excluded.code_1x"},
		"code_200":                       clause.Expr{SQL: "uptime_reports.code_200 + excluded.code_200"},
		"code_201":                       clause.Expr{SQL: "uptime_reports.code_201 + excluded.code_201"},
		"code_2x":                        clause.Expr{SQL: "uptime_reports.code_2x + excluded.code_2x"},
		"code_301":                       clause.Expr{SQL: "uptime_reports.code_301 + excluded.code_301"},
		"code_302":                       clause.Expr{SQL: "uptime_reports.code_302 + excluded.code_302"},
		"code_303":                       clause.Expr{SQL: "uptime_reports.code_303 + excluded.code_303"},
		"code_304":                       clause.Expr{SQL: "uptime_reports.code_304 + excluded.code_304"},
		"code_3x":                        clause.Expr{SQL: "uptime_reports.code_3x + excluded.code_3x"},
		"code_400":                       clause.Expr{SQL: "uptime_reports.code_400 + excluded.code_400"},
		"code_401":                       clause.Expr{SQL: "uptime_reports.code_401 + excluded.code_401"},
		"code_403":                       clause.Expr{SQL: "uptime_reports.code_403 + excluded.code_403"},
		"code_404":                       clause.Expr{SQL: "uptime_reports.code_404 + excluded.code_404"},
		"code_429":                       clause.Expr{SQL: "uptime_reports.code_429 + excluded.code_429"},
		"code_4x":                        clause.Expr{SQL: "uptime_reports.code_4x + excluded.code_4x"},
		"code_500":                       clause.Expr{SQL: "uptime_reports.code_500 + excluded.code_500"},
		"code_501":                       clause.Expr{SQL: "uptime_reports.code_501 + excluded.code_501"},
		"code_502":                       clause.Expr{SQL: "uptime_reports.code_502 + excluded.code_502"},
		"code_503":                       clause.Expr{SQL: "uptime_reports.code_503 + excluded.code_503"},
		"code_504":                       clause.Expr{SQL: "uptime_reports.code_504 + excluded.code_504"},
		"code_5x":                        clause.Expr{SQL: "uptime_reports.code_5x + excluded.code_5x"},
		"counter_bytes_in":               clause.Expr{SQL: "uptime_reports.counter_bytes_in + excluded.counter_bytes_in"},
		"counter_bytes_out":              clause.Expr{SQL: "uptime_reports.counter_bytes_out + excluded.counter_bytes_out"},
		"counter_closed_connections":     clause.Expr{SQL: "uptime_reports.counter_closed_connections + excluded.counter_closed_connections"},
		"counter_error":                  clause.Expr{SQL: "uptime_reports.counter_error + excluded.counter_error"},
		"counter_hits":                   clause.Expr{SQL: "uptime_reports.counter_hits + excluded.counter_hits"},
		"counter_last_time":              clause.Expr{SQL: "excluded.counter_last_time"},
		"counter_latency":                clause.Expr{SQL: "(uptime_reports.counter_total_latency  +excluded.counter_total_latency)/CAST( uptime_reports.counter_hits + excluded.counter_hits AS REAL)"},
		"counter_max_latency":            clause.Expr{SQL: "0.5 * ((uptime_reports.counter_max_latency + excluded.counter_max_latency) + ABS(uptime_reports.counter_max_latency - excluded.counter_max_latency))"},
		"counter_max_upstream_latency":   clause.Expr{SQL: "0.5 * ((uptime_reports.counter_max_upstream_latency + excluded.counter_max_upstream_latency) + ABS(uptime_reports.counter_max_upstream_latency - excluded.counter_max_upstream_latency))"},
		"counter_min_latency":            clause.Expr{SQL: "0.5 * ((uptime_reports.counter_min_latency + excluded.counter_min_latency) - ABS(uptime_reports.counter_min_latency - excluded.counter_min_latency)) "},
		"counter_min_upstream_latency":   clause.Expr{SQL: "0.5 * ((uptime_reports.counter_min_upstream_latency + excluded.counter_min_upstream_latency) - ABS(uptime_reports.counter_min_upstream_latency - excluded.counter_min_upstream_latency)) "},
		"counter_open_connections":       clause.Expr{SQL: "uptime_reports.counter_open_connections + excluded.counter_open_connections"},
		"counter_request_time":           clause.Expr{SQL: "(uptime_reports.counter_total_request_time  +excluded.counter_total_request_time)/CAST( uptime_reports.counter_hits + excluded.counter_hits AS REAL)"},
		"counter_success":                clause.Expr{SQL: "uptime_reports.counter_success + excluded.counter_success"},
		"counter_total_latency":          clause.Expr{SQL: "uptime_reports.counter_total_latency + excluded.counter_total_latency"},
		"counter_total_request_time":     clause.Expr{SQL: "uptime_reports.counter_total_request_time + excluded.counter_total_request_time"},
		"counter_total_upstream_latency": clause.Expr{SQL: "uptime_reports.counter_total_upstream_latency + excluded.counter_total_upstream_latency"},
		"counter_upstream_latency":       clause.Expr{SQL: "(uptime_reports.counter_total_upstream_latency  +excluded.counter_total_upstream_latency)/CAST( uptime_reports.counter_hits + excluded.counter_hits AS REAL)"},
	}

	assert.Equal(t, expectedAssignmets, assignments)
}
