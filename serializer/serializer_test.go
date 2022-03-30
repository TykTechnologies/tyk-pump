package serializer

import (
	"testing"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
)

func TestSerializer_Encode(t *testing.T) {
	tcs := []struct {
		testName   string
		serializer AnalyticsSerializer
	}{
		{
			testName:   "msgpack",
			serializer: NewAnalyticsSerializer(MSGP_SERIALIZER),
		},
		{
			testName:   "protobuf",
			serializer: NewAnalyticsSerializer(PROTOBUF_SERIALIZER),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			record := analytics.AnalyticsRecord{
				APIID: "api_1",
				OrgID: "org_1",
			}

			bytes, err := tc.serializer.Encode(&record)

			assert.Equal(t, nil, err)
			assert.NotEqual(t, 0, len(bytes))
		})
	}
}

func TestSerializer_Decode(t *testing.T) {
	tcs := []struct {
		testName   string
		serializer AnalyticsSerializer
	}{
		{
			testName:   "msgpack",
			serializer: NewAnalyticsSerializer(MSGP_SERIALIZER),
		},
		{
			testName:   "protobuf",
			serializer: NewAnalyticsSerializer(PROTOBUF_SERIALIZER),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			record := analytics.AnalyticsRecord{
				APIID: "api_1",
				OrgID: "org_1",
			}

			bytes, _ := tc.serializer.Encode(&record)
			newRecord := &analytics.AnalyticsRecord{}

			err := tc.serializer.Decode(bytes, newRecord
			if err != nil {
				t.Fatal(err)
			}
			assert.ObjectsAreEqualValues(record, newRecord)
		})
	}
}

func TestSerializer_GetSuffix(t *testing.T) {
	tcs := []struct {
		testName       string
		serializer     AnalyticsSerializer
		expectedSuffix string
	}{
		{
			testName:       "msgpack",
			serializer:     NewAnalyticsSerializer(MSGP_SERIALIZER),
			expectedSuffix: "",
		},
		{
			testName:       "protobuf",
			serializer:     NewAnalyticsSerializer(PROTOBUF_SERIALIZER),
			expectedSuffix: "_protobuf",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			assert.Equal(t, tc.expectedSuffix, tc.serializer.GetSuffix())
		})
	}
}
