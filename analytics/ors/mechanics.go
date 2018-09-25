package ors

import (
	"bufio"
	"encoding/base64"
	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/logrus-prefixed-formatter"
	. "github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tykcommon-logger"
	"math"
	"net/http"
	"strings"
)

var mechanicsPrefix = "mechanics"
var log = logger.GetLogger()
var ValueCollection = map[string]interface{}{}

func init() {
	log.Formatter = new(prefixed.TextFormatter)
}

func processViaCoordinates(viaCoordinates interface{}) []map[string]interface{} {
	// interface to string?
	splittedCoordinates := strings.SplitAfter(viaCoordinates.(string), ",")
	viaCoords := make([]map[string]interface{}, len(splittedCoordinates))
	coordinatePair := map[string]interface{}{}
	coordinateCounter := 0.0
	for _, coordinate := range splittedCoordinates {
		cleanedCoordinate := strings.TrimRight(coordinate, ",")
		if math.Mod(coordinateCounter, 2) == 0 {
			coordinatePair["lat"] = cleanedCoordinate
		} else {
			coordinatePair["lng"] = cleanedCoordinate
			viaCoords = append(viaCoords, coordinatePair)
			coordinatePair = map[string]interface{}{}
		}
	}
	return viaCoords
}

func getEndCoordinatesFromViaCoords(viaCoordinates []map[string]interface{}) (float64, float64) {
	endCoordinates := viaCoordinates[len(viaCoordinates)-1]
	endLat := endCoordinates["lat"].(float64)
	endLng := endCoordinates["lng"]
	return endLat, endLng.(float64)

}

func getCoordinatesFromReferer(referer string) RefererCoordinates {
	refererMap := requestRefererToParameterMap(referer)
	coordinates := RefererCoordinates{}
	if _, ok := refererMap["n1"]; ok {
		if _, ok2 := refererMap["n2"]; ok2 {
			coordinates.startLat = refererMap["n1"].(float64)
			coordinates.endLng = refererMap["n1"].(float64)
		}
		if _, ok3 := refererMap["a"]; ok3 {
			viaCoordinatesString := refererMap["a"]
			viaCoordinates := processViaCoordinates(viaCoordinatesString)
			coordinates.viaCoords = viaCoordinates
			endLat, endLng := getEndCoordinatesFromViaCoords(viaCoordinates)
			coordinates.endLat = endLat
			coordinates.endLng = endLng
		}
	}
	return coordinates
}

func GetRequestQueryValues(stringRequest string) map[string]interface{} {
	// TODO Some requests have an insufficient rawQuery that doesnt represent everything
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
		coordinates := getCoordinatesFromReferer(referer)
		queryMap["coordinates"] = coordinates
	}
	return queryMap
}

func generateStatsFromDecodedGetReq(decodedRawRequest []byte) OrsRouteStats {
	decodedRawRequestString := string(decodedRawRequest)
	requestQueryValues := GetRequestQueryValues(decodedRawRequestString)
	processedQueryValues := processQueryValues(requestQueryValues)
	//orsStats := CalculateOrsStats(processedQueryValues)
	// TODO Check if the return should be in json or as a request with the manipulated Request
	// I could add a new key value pair to the Analytics Record e.g. OrsStats, that holds all the desired values!#
	// The rest of the request wouldnt be touched
	// Maybe the ors_stats can be clean written in the output wo converting it to an array
	return processedQueryValues
}

func generateStatsFromDecodedPostReq(bytes []byte) {

}

func ProcessDirectionsRecordOrsRouteStats(analyticsRecod AnalyticsRecord) OrsRouteStats {
	method := analyticsRecod.Method
	encodedRawRequest := analyticsRecod.RawRequest
	decodedRawReq, _ := base64.StdEncoding.DecodeString(encodedRawRequest)
	orsRouteStats := OrsRouteStats{}
	if method == "GET" {
		orsRouteStats = generateStatsFromDecodedGetReq(decodedRawReq)
		return orsRouteStats

	} else if method == "POST" {
		generateStatsFromDecodedPostReq(decodedRawReq)
		log.WithFields(logrus.Fields{
			"prefix": mechanicsPrefix,
		}).Debug("Method not implemented: ", method)
		return orsRouteStats
	} else {
		return orsRouteStats
	}
}

func CalculateDirectionsStats(analyticsRecod AnalyticsRecord) OrsRouteStats {

}

func CalculateOrsStats(analyticsRecord AnalyticsRecord) AnalyticsRecord {
	analyticsRecord = analyticsRecord
	endpoint := analyticsRecord.APIName
	if endpoint == "Isochrones" {

	} else if endpoint == "Matrix" {
		return analyticsRecord
	} else if endpoint == "Directions" {
		analyticsRecord.OrsRouteStats = ProcessDirectionsRecordOrsRouteStats(analyticsRecord)
		return analyticsRecord
	} else if endpoint == "GeocodeReverseForPublic" {
		return analyticsRecord
	}
	return analyticsRecord
}
