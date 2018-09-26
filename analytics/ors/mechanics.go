package ors

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/logrus-prefixed-formatter"
	. "github.com/TykTechnologies/tyk-pump/analytics"
	. "github.com/TykTechnologies/tyk-pump/analytics/ors/routes"
	"github.com/TykTechnologies/tykcommon-logger"
	"net/http"
	"strings"
)

var mechanicsPrefix = "mechanics"
var log = logger.GetLogger()

func init() {
	log.Formatter = new(prefixed.TextFormatter)
}

func processBareCoordinatePair(bareCoordsPair string) (string, string) {
	splittedCoordinates := strings.SplitAfter(bareCoordsPair, ",")
	lat := strings.TrimRight(splittedCoordinates[0], ",")
	lng := strings.TrimRight(splittedCoordinates[1], ",")
	return strings.TrimSpace(lat), strings.TrimSpace(lng)

}

func processCoordinates(bareCoords string) RouteCoordinates {
	// interface to string?
	routeCoordinates := RouteCoordinates{}
	splittedCoordinates := strings.SplitAfter(bareCoords, "|")
	viaCoords := make([]map[string]interface{}, len(splittedCoordinates)-2)
	for index, coordinate := range splittedCoordinates {
		cleanedCoordinatePair := strings.TrimRight(coordinate, "|")
		lat, lng := processBareCoordinatePair(cleanedCoordinatePair)
		viaCoordPair := make(map[string]interface{}, 2)
		if index == 0 {
			routeCoordinates.StartLat = lat
			routeCoordinates.StartLng = lng
		} else if index == len(splittedCoordinates)-1 {
			routeCoordinates.EndLat = lat
			routeCoordinates.EndLng = lng
		} else {
			viaCoordPair["lat"] = lat
			viaCoordPair["lng"] = lng
			viaCoords = append(viaCoords, viaCoordPair)
		}
	}
	routeCoordinates.ViaCoords = viaCoords
	return routeCoordinates
}

func processJsonFromString(potentialJson string) (map[string]interface{}, error) {
	var unmarshalledJson map[string]interface{}
	err := json.Unmarshal([]byte(potentialJson), &unmarshalledJson)
	if err != nil {
		return map[string]interface{}{}, err
	} else {
		return unmarshalledJson, err
	}
}

func GetRequestQueryValues(stringRequest string) map[string]interface{} {
	reader := bufio.NewReader(strings.NewReader(stringRequest))
	request, _ := http.ReadRequest(reader)
	query := request.URL.Query()
	queryMap := map[string]interface{}{}
	// TODO check how the complex map works in graylog when a json is unmarshaled into a map with multiple levels!
	for key, value := range query {
		if key == "coordinates" {
			routeCoordinates := processCoordinates(value[0])
			queryMap[key] = routeCoordinates
		} else {
			potentialJson, validJson := processJsonFromString(value[0])
			if validJson == nil {
				queryMap[key] = potentialJson
			} else {
				queryMap[key] = value
			}
		}

	}
	return queryMap
}

func generateStatsFromDecodedGetReq(decodedRawRequest []byte) OrsRouteStats {
	decodedRawRequestString := string(decodedRawRequest)
	requestQueryValues := GetRequestQueryValues(decodedRawRequestString)
	processedQueryValues := ProcessQueryValues(requestQueryValues)
	//orsStats := CalculateOrsStats(processedQueryValues)
	// TODO Check if the return should be in json or as a request with the manipulated Request
	// I could add a new key value pair to the Analytics Record e.g. OrsStats, that holds all the desired values!#
	// The rest of the request wouldnt be touched
	// Maybe the ors_stats can be clean written in the output wo converting it to an array
	return processedQueryValues
}

func generateStatsFromDecodedPostReq(bytes []byte) {
	// Example function for further endpoints
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

func CalculateDirectionsStats(analyticsRecod AnalyticsRecord) {
	// Example function for further endpoints
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
