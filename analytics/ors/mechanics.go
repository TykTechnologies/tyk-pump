package ors

import (
	"bufio"
	"encoding/base64"
	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/logrus-prefixed-formatter"
	. "github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tykcommon-logger"
	"net/http"
	"strings"
)

var mechanicsPrefix = "mechanics"
var log = logger.GetLogger()

func init() {
	log.Formatter = new(prefixed.TextFormatter)
}

func CalculateOrsStats(requestContentAsList map[string]interface{}) OrsRouteStats {
	return OrsRouteStats{1, 0, 0, 0, 0}
}

func getRequestReferer(stringRequest string) string {
	reader := bufio.NewReader(strings.NewReader(stringRequest))
	request, _ := http.ReadRequest(reader)
	referer := request.Header.Get("Referer")
	return referer
}

func GetRequestRefererAsParameterMap(decodedRequestContent string) map[string]interface{} {
	requestReferer := getRequestReferer(decodedRequestContent)
	requestRefererParameterMap := requestRefererToParameterMap(requestReferer)
	return requestRefererParameterMap
}

func ProcessDecodedRawRequest(decodedRawRequest []byte) OrsRouteStats {
	decodedRawRequestString := string(decodedRawRequest)
	requestRefererAsMap := GetRequestRefererAsParameterMap(decodedRawRequestString)
	processedOrsStats := CalculateOrsStats(requestRefererAsMap)
	// TODO Check if the return should be in json or as a request with the manipulated Request
	// I could add a new key value pair to the Analytics Record e.g. OrsStats, that holds all the desired values!#
	// The rest of the request wouldnt be touched
	// Maybe the ors_stats can be clean written in the output wo converting it to an array
	return processedOrsStats
}

func ProcessRawRequestToOrsRouteStats(rawEncodedRequest string) OrsRouteStats {
	decodedRawReq, _ := base64.StdEncoding.DecodeString(rawEncodedRequest)
	orsRouteStats := ProcessDecodedRawRequest(decodedRawReq)
	return orsRouteStats
}

func CalculateOrsRouteStats(analyticsRecord AnalyticsRecord) AnalyticsRecord {

	analyticsRecord = analyticsRecord
	method := analyticsRecord.Method
	if method == "GET" {
		rawRequest := analyticsRecord.RawRequest
		orsRouteStats := ProcessRawRequestToOrsRouteStats(rawRequest)
		analyticsRecord.OrsRouteStats = orsRouteStats
	} else if method == "POST" {
		log.WithFields(logrus.Fields{
			"prefix": mechanicsPrefix,
		}).Debug("Method not implemented: ", method)
	}

	return analyticsRecord
}
