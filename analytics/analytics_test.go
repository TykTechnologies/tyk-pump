package analytics

import (
	"encoding/base64"
	"testing"
)

func TestObfuscateKeys(t *testing.T) {

	const AUTH_HEADER_NAME = "Authorization"

	cases := []struct {
		testName          string
		decodedRawRequest string
		record            AnalyticsRecord
		authHeaderName    string
		expectedKey       string
		expectedRequest   string
	}{
		{
			"Record_with_an_empty_key",
			//"GET ip HTTP/1.1\nHost: localhost:8080\nUser-Agent: PostmanRuntime/7.26.1\nAccept: */*\nAccept-Encoding: gzip, deflate, br\nAuthorization:\nPostman-Token: 934e08ba-bd6b-460f-bee3-c13756b4f445\nX-Api-Version: v2\n"
			`GET ip HTTP/1.1\r\n
			Host: localhost:8080\r\n
			User-Agent: PostmanRuntime/7.26.1\r\n
			Accept: */*\r\n
			Accept-Encoding: gzip, deflate, br\r\n
			Authorization: \r\n
			Postman-Token: 934e08ba-bd6b-460f-bee3-c13756b4f445\nX-Api-Version: v2\r\n
			\r\n\r\n`,
			AnalyticsRecord{
				APIKey: "",
				//RawRequest: "R0VUIGlwIEhUVFAvMS4xCkhvc3Q6IGxvY2FsaG9zdDo4MDgwClVzZXItQWdlbnQ6IFBvc3RtYW5SdW50aW1lLzcuMjYuMQpBY2NlcHQ6ICovKgpBY2NlcHQtRW5jb2Rpbmc6IGd6aXAsIGRlZmxhdGUsIGJyCkF1dGhvcml6YXRpb246ClBvc3RtYW4tVG9rZW46IDkzNGUwOGJhLWJkNmItNDYwZi1iZWUzLWMxMzc1NmI0ZjQ0NQpYLUFwaS1WZXJzaW9uOiB2MgoK"
				// Decoded raw request:
			},
			AUTH_HEADER_NAME,
			"",
			"R0VUIGlwIEhUVFAvMS4xXHJcbgoJCQlIb3N0OiBsb2NhbGhvc3Q6ODA4MFxyXG4KCQkJVXNlci1BZ2VudDogUG9zdG1hblJ1bnRpbWUvNy4yNi4xXHJcbgoJCQlBY2NlcHQ6ICovKlxyXG4KCQkJQWNjZXB0LUVuY29kaW5nOiBnemlwLCBkZWZsYXRlLCBiclxyXG4KCQkJQXV0aG9yaXphdGlvbjogXHJcbgoJCQlQb3N0bWFuLVRva2VuOiA5MzRlMDhiYS1iZDZiLTQ2MGYtYmVlMy1jMTM3NTZiNGY0NDVcblgtQXBpLVZlcnNpb246IHYyXHJcbgoJCQlcclxuXHJcbg==",
			//`GET ip HTTP/1.1\r\n
			//Host: localhost:8080\r\n
			//User-Agent: PostmanRuntime/7.26.1\r\n
			//Accept: */*\r\n
			//Accept-Encoding: gzip, deflate, br\r\n
			//Authorization: \r\n
			//Postman-Token: 934e08ba-bd6b-460f-bee3-c13756b4f445\nX-Api-Version: v2\r\n
			//\r\n\r\n`,
		},
		{
			"Record_with_regular_key",
			`GET ip HTTP/1.1\r\n
			Host: localhost:8080\r\n
			User-Agent: PostmanRuntime/7.26.1\r\n
			Accept: */*\r\n
			Accept-Encoding: gzip, deflate, br\r\n
			Authorization: 59d27324b8125f000137663e2c650c3576b348bfbe1490fef5db0c49\r\n
			Postman-Token: 934e08ba-bd6b-460f-bee3-c13756b4f445\nX-Api-Version: v2\r\n
			\r\n\r\n`,
			AnalyticsRecord{APIKey: "59d27324b8125f000137663e2c650c3576b348bfbe1490fef5db0c49"},
			AUTH_HEADER_NAME,
			"****0c49", // Been obfuscated
			"R0VUIGlwIEhUVFAvMS4xXHJcbgoJCQlIb3N0OiBsb2NhbGhvc3Q6ODA4MFxyXG4KCQkJVXNlci1BZ2VudDogUG9zdG1hblJ1bnRpbWUvNy4yNi4xXHJcbgoJCQlBY2NlcHQ6ICovKlxyXG4KCQkJQWNjZXB0LUVuY29kaW5nOiBnemlwLCBkZWZsYXRlLCBiclxyXG4KCQkJQXV0aG9yaXphdGlvbjogKioqKjBjNDlcclxuCgkJCVBvc3RtYW4tVG9rZW46IDkzNGUwOGJhLWJkNmItNDYwZi1iZWUzLWMxMzc1NmI0ZjQ0NVxuWC1BcGktVmVyc2lvbjogdjJcclxuCgkJCVxyXG5cclxu",
			//`GET ip HTTP/1.1\r\n
			//Host: localhost:8080\r\n
			//User-Agent: PostmanRuntime/7.26.1\r\n
			//Accept: */*\r\n
			//Accept-Encoding: gzip, deflate, br\r\n
			//Authorization: ****0c49\r\n
			//Postman-Token: 934e08ba-bd6b-460f-bee3-c13756b4f445\nX-Api-Version: v2\r\n
			//\r\n\r\n`,
		},
		{
			"Record_with_key_length_less_than4",
			`GET ip HTTP/1.1\r\n
			Host: localhost:8080\r\n
			User-Agent: PostmanRuntime/7.26.1\r\n
			Accept: */*\r\n
			Accept-Encoding: gzip, deflate, br\r\n
			Authorization: a59d\r\n
			Postman-Token: 934e08ba-bd6b-460f-bee3-c13756b4f445\nX-Api-Version: v2\r\n
			\r\n\r\n`,
			AnalyticsRecord{APIKey: "a59d"},
			AUTH_HEADER_NAME,
			"----", // Been obfuscated
			"R0VUIGlwIEhUVFAvMS4xXHJcbgoJCQlIb3N0OiBsb2NhbGhvc3Q6ODA4MFxyXG4KCQkJVXNlci1BZ2VudDogUG9zdG1hblJ1bnRpbWUvNy4yNi4xXHJcbgoJCQlBY2NlcHQ6ICovKlxyXG4KCQkJQWNjZXB0LUVuY29kaW5nOiBnemlwLCBkZWZsYXRlLCBiclxyXG4KCQkJQXV0aG9yaXphdGlvbjogLS0tLVxyXG4KCQkJUG9zdG1hbi1Ub2tlbjogOTM0ZTA4YmEtYmQ2Yi00NjBmLWJlZTMtYzEzNzU2YjRmNDQ1XG5YLUFwaS1WZXJzaW9uOiB2MlxyXG4KCQkJXHJcblxyXG4=",
			//`GET ip HTTP/1.1\r\n
			//Host: localhost:8080\r\n
			//User-Agent: PostmanRuntime/7.26.1\r\n
			//Accept: */*\r\n
			//Accept-Encoding: gzip, deflate, br\r\n
			//Authorization: ----\r\n
			//Postman-Token: 934e08ba-bd6b-460f-bee3-c13756b4f445\nX-Api-Version: v2\r\n
			//\r\n\r\n`,
		},
		{
			"Record_with_new_format_key",
			`GET ip HTTP/1.1\r\n
			Host: localhost:8080\r\n
			User-Agent: PostmanRuntime/7.26.1\r\n
			Accept: */*\r\n
			Accept-Encoding: gzip, deflate, br\r\n
			Authorization: eyJvcmciOiI1ZTlkOTU0NGExZGNkNjAwMDFkMGVkMjAiLCJpZCI6InlhYXJhMTIzIiwiaCI6Im11cm11cjY0In0=\r\n
			Postman-Token: 934e08ba-bd6b-460f-bee3-c13756b4f445\nX-Api-Version: v2
			\r\n\r\n`,
			AnalyticsRecord{APIKey: "eyJvcmciOiI1ZTlkOTU0NGExZGNkNjAwMDFkMGVkMjAiLCJpZCI6InlhYXJhMTIzIiwiaCI6Im11cm11cjY0In0="},
			AUTH_HEADER_NAME,
			"****In0=", // Been obfuscated
			"R0VUIGlwIEhUVFAvMS4xXHJcbgoJCQlIb3N0OiBsb2NhbGhvc3Q6ODA4MFxyXG4KCQkJVXNlci1BZ2VudDogUG9zdG1hblJ1bnRpbWUvNy4yNi4xXHJcbgoJCQlBY2NlcHQ6ICovKlxyXG4KCQkJQWNjZXB0LUVuY29kaW5nOiBnemlwLCBkZWZsYXRlLCBiclxyXG4KCQkJQXV0aG9yaXphdGlvbjogKioqKkluMD1cclxuCgkJCVBvc3RtYW4tVG9rZW46IDkzNGUwOGJhLWJkNmItNDYwZi1iZWUzLWMxMzc1NmI0ZjQ0NVxuWC1BcGktVmVyc2lvbjogdjIKCQkJXHJcblxyXG4=",
			//`GET ip HTTP/1.1\r\n
			//Host: localhost:8080\r\n
			//User-Agent: PostmanRuntime/7.26.1\r\n
			//Accept: */*\r\n
			//Accept-Encoding: gzip, deflate, br\r\n
			//Authorization: ****In0=\r\n
			//Postman-Token: 934e08ba-bd6b-460f-bee3-c13756b4f445\nX-Api-Version: v2
			//\r\n\r\n`,
		},
		{
			"Record_with_key_length_of_5",
			`GET ip HTTP/1.1\r\n
			Host: localhost:8080\r\n
			User-Agent: PostmanRuntime/7.26.1\r\n
			Accept: */*\r\n
			Accept-Encoding: gzip, deflate, br\r\n
			Authorization: 12345\r\n
			Postman-Token: 934e08ba-bd6b-460f-bee3-c13756b4f445\nX-Api-Version: v2\r\n
			\r\n\r\n`,
			AnalyticsRecord{APIKey: "12345"},
			AUTH_HEADER_NAME,
			"****2345", // Been obfuscated
			"R0VUIGlwIEhUVFAvMS4xXHJcbgoJCQlIb3N0OiBsb2NhbGhvc3Q6ODA4MFxyXG4KCQkJVXNlci1BZ2VudDogUG9zdG1hblJ1bnRpbWUvNy4yNi4xXHJcbgoJCQlBY2NlcHQ6ICovKlxyXG4KCQkJQWNjZXB0LUVuY29kaW5nOiBnemlwLCBkZWZsYXRlLCBiclxyXG4KCQkJQXV0aG9yaXphdGlvbjogKioqKjIzNDVcclxuCgkJCVBvc3RtYW4tVG9rZW46IDkzNGUwOGJhLWJkNmItNDYwZi1iZWUzLWMxMzc1NmI0ZjQ0NVxuWC1BcGktVmVyc2lvbjogdjJcclxuCgkJCVxyXG5cclxu",
			//`GET ip HTTP/1.1\r\n
			//Host: localhost:8080\r\n
			//User-Agent: PostmanRuntime/7.26.1\r\n
			//Accept: */*\r\n
			//Accept-Encoding: gzip, deflate, br\r\n
			//Authorization: ****2345\r\n
			//Postman-Token: 934e08ba-bd6b-460f-bee3-c13756b4f445\nX-Api-Version: v2\r\n
			//\r\n\r\n`,
		},
		{
			"Authorization_custom_header", // Authorisation with 's'
			`GET ip HTTP/1.1\r\n
			Host: localhost:8080\r\n
			User-Agent: PostmanRuntime/7.26.1\r\n
			Accept: */*\r\n
			Accept-Encoding: gzip, deflate, br\r\n
			X-Authorisation: eyJvcmciOiI1ZTlkOTU0NGExZGNkNjAwMDFkMGVkMjAiLCJpZCI6InlhYXJhMTIzIiwiaCI6Im11cm11cjY0In0=\r\n
			Postman-Token: 934e08ba-bd6b-460f-bee3-c13756b4f445\nX-Api-Version: v2\r\n
			\r\n\r\n`,
			AnalyticsRecord{APIKey: "eyJvcmciOiI1ZTlkOTU0NGExZGNkNjAwMDFkMGVkMjAiLCJpZCI6InlhYXJhMTIzIiwiaCI6Im11cm11cjY0In0="},
			"X-Authorisation",
			"****In0=", // Been obfuscated,
			"R0VUIGlwIEhUVFAvMS4xXHJcbgoJCQlIb3N0OiBsb2NhbGhvc3Q6ODA4MFxyXG4KCQkJVXNlci1BZ2VudDogUG9zdG1hblJ1bnRpbWUvNy4yNi4xXHJcbgoJCQlBY2NlcHQ6ICovKlxyXG4KCQkJQWNjZXB0LUVuY29kaW5nOiBnemlwLCBkZWZsYXRlLCBiclxyXG4KCQkJWC1BdXRob3Jpc2F0aW9uOiAqKioqSW4wPVxyXG4KCQkJUG9zdG1hbi1Ub2tlbjogOTM0ZTA4YmEtYmQ2Yi00NjBmLWJlZTMtYzEzNzU2YjRmNDQ1XG5YLUFwaS1WZXJzaW9uOiB2MlxyXG4KCQkJXHJcblxyXG4=",
			//`GET ip HTTP/1.1\r\n
			//Host: localhost:8080\r\n
			//User-Agent: PostmanRuntime/7.26.1\r\n
			//Accept: */*\r\n
			//Accept-Encoding: gzip, deflate, br\r\n
			//X-Authorisation: ****In0=\r\n
			//Postman-Token: 934e08ba-bd6b-460f-bee3-c13756b4f445\nX-Api-Version: v2\r\n
			//\r\n\r\n`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.testName, func(t *testing.T) {
			//Calculating the raw request
			tc.record.RawRequest = base64.StdEncoding.EncodeToString([]byte(tc.decodedRawRequest))
			tc.record.ObfuscateKey(tc.authHeaderName)
			if tc.record.APIKey != tc.expectedKey {
				t.Errorf("Error in obfuscated key: expected %s, actual %s",
					tc.expectedKey, tc.record.APIKey)
			}
			if tc.record.RawRequest != tc.expectedRequest {
				t.Errorf("Error in obfuscated raw request: \nexpected request %s,\n actual raw request %s",
					tc.expectedRequest, tc.record.RawRequest)
			}
		})
	}
}
