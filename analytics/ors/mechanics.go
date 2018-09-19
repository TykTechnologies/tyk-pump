package ors

import (
	"bufio"
	"encoding/base64"
	. "github.com/TykTechnologies/tyk-pump/analytics"
	"net/http"
	"strings"
)

func CalculateOrsStats(requestContentAsList map[string]interface{}) OrsRouteStats {
	return OrsRouteStats{0}
}

func getRequestReferer(stringRequest string) string {
	reader := bufio.NewReader(strings.NewReader(stringRequest))
	request, _ := http.ReadRequest(reader)
	referer := request.Header.Get("Referer")
	return referer
}

//GetRequestRefererAsMap Split the request url here and return all the elements as a map
// Use refererCoordinates
func requestRefererToMap(s string) map[string]interface{} {
	//viaCoords := list.New()
	//viaCoords.PushBack(0.00)
	//coordinates := refererCoordinates{0, 0, 0, 0, viaCoords}
	//Interface to store any type in here!
	m := make(map[string]interface{})
	m["length"] = 0
	return m
}

func GetRequestRefererAsMap(decodedRequestContent string) map[string]interface{} {
	requestReferer := getRequestReferer(decodedRequestContent)
	requestRefererAsMap := requestRefererToMap(requestReferer)
	return requestRefererAsMap
}

func ProcessDecodedRawRequest(decodedRawRequest []byte) OrsRouteStats {
	decodedRawRequestString := string(decodedRawRequest)
	requestRefererAsMap := GetRequestRefererAsMap(decodedRawRequestString)
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
	rawRequest := analyticsRecord.RawRequest
	orsRouteStats := ProcessRawRequestToOrsRouteStats(rawRequest)
	analyticsRecord.OrsRouteStats = orsRouteStats
	return analyticsRecord
}
