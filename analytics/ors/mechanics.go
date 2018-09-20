package ors

import (
	"bufio"
	"encoding/base64"
	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/logrus-prefixed-formatter"
	. "github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tykcommon-logger"
	"net/http"
	"net/url"
	"strings"
)

var mechanicsPrefix = "mechanics"
var log = logger.GetLogger()
var ValueCollection = map[string]interface{}{}

func init() {
	log.Formatter = new(prefixed.TextFormatter)
}

func processViaCoordinates(i map[string]interface{}) (map[string]interface{}, map[string]interface{}) {

}

func getCoordinatesFromReferer(referer string) (map[string]interface{}, map[string]interface{}, map[string]interface{}) {
	coordinates := map[string]interface{}{}
	viaCoordinates := map[string]interface{}{}
	endCoordinates := map[string]interface{}{}
	refererMap := requestRefererToParameterMap(referer)
	if _, ok := refererMap["n1"]; ok {
		if _, ok2 := refererMap["n2"]; ok2 {
			coordinates["start_lat"] = refererMap["n1"]
			coordinates["start_lng"] = refererMap["n2"]
		}
		if _, ok3 := refererMap["a"]; ok3 {
			viaCoordinates, endCoordinates = processViaCoordinates(refererMap["a"])
		}
	}
	return coordinates, viaCoordinates, endCoordinates
}

func GetRequestQueryValues(stringRequest string) url.Values {
	// TODO Some requests have a insufficient rawQuery that doesnt represent everything
	reader := bufio.NewReader(strings.NewReader(stringRequest))
	request, _ := http.ReadRequest(reader)
	query := request.URL.Query()
	queryMap := map[string]interface{}{}
	referer := request.Referer()
	for key, value := range query {
		if key != "coordinates" {
			queryMap[key] = value
		}
		// check := key
		// if _, ok := ValueCollection[check]; !ok {
		// 	 ValueCollection[key] = 0
		// }
	}
	if strings.ContainsAny(referer, "n1 & n2") {
		startCoordinates, viaCoordinates, endCoordinates := getCoordinatesFromReferer(referer)
		queryMap["start_coordinates"] = startCoordinates
		queryMap["via_coordinates"] = viaCoordinates
		queryMap["end_coordinates"] = endCoordinates
	}
	return query
}

func ProcessDecodedRawRequest(decodedRawRequest []byte) OrsRouteStats {
	decodedRawRequestString := string(decodedRawRequest)
	requestQueryValues := GetRequestQueryValues(decodedRawRequestString)
	processedQueryValues := processQueryValues(requestQueryValues)
	orsStats := CalculateOrsStats(processedQueryValues)
	// TODO Check if the return should be in json or as a request with the manipulated Request
	// I could add a new key value pair to the Analytics Record e.g. OrsStats, that holds all the desired values!#
	// The rest of the request wouldnt be touched
	// Maybe the ors_stats can be clean written in the output wo converting it to an array
	return orsStats
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
