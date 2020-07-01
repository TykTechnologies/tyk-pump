package analytics

import (
	"testing"
)

func TestObfuscateKeys(t *testing.T) {

	cases := []struct {
		testName    string
		record      AnalyticsRecord
		expectedKey string
	}{
		{
			"Record with an empty key",
			AnalyticsRecord{APIKey: ""},
			"----",
		},
		{
			"Record with regular key",
			AnalyticsRecord{APIKey: "59d27324b8125f000137663e2c650c3576b348bfbe1490fef5db0c49"},
			"****0c49", // Been obfuscated
		},
		{
			"Record with key length <= 4",
			AnalyticsRecord{APIKey: "a59d"},
			"----", // Been obfuscated
		},
		{
			"Record with new format key",
			AnalyticsRecord{APIKey: "eyJvcmciOiI1ZTlkOTU0NGExZGNkNjAwMDFkMGVkMjAiLCJpZCI6InlhYXJhMTIzIiwiaCI6Im11cm11cjY0In0="},
			"****In0=", // Been obfuscated
		},
		{
			"Record with key length of 5",
			AnalyticsRecord{APIKey: "12345"},
			"****2345", // Been obfuscated
		},
	}

	for _, tc := range cases {
		t.Run(tc.testName, func(t *testing.T) {
			tc.record.ObfuscateKey()
			if tc.record.APIKey != tc.expectedKey {
				t.Errorf("Record with an empty Key: Expected %s, actual %s",
					tc.expectedKey, tc.record.APIKey)
			}
		})
	}
}
