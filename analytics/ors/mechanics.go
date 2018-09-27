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
	"reflect"
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
	var viaCoords []map[string]interface{}
	for index, coordinate := range splittedCoordinates {
		cleanedCoordinatePair := strings.TrimRight(coordinate, "|")
		lat, lng := processBareCoordinatePair(cleanedCoordinatePair)
		viaCoordPair := map[string]interface{}{}
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

var ExchangeMap = map[string]interface{}{}

/*func complexMapToSimpleExchangeMap(jsonMap map[string]interface{}) map[string]interface{}{
	returnMap := map[string]interface{}{}
	for key, value := range jsonMap {
		if reflect.TypeOf(value) == reflect.TypeOf(map[string]interface{}{}) {
			returnMap[key] = complexMapToSimpleExchangeMap(value.(map[string]interface{}))
		} else {
			returnMap[key] = value
		}
	}
	return returnMap
}*/

// TODO Change, so every next level will be returned with its prior level as prefix
func complexMapToSimpleExchangeMap(jsonMap map[string]interface{}) {
	for key, value := range jsonMap {
		if reflect.TypeOf(value) == reflect.TypeOf(map[string]interface{}{}) {
			complexMapToSimpleExchangeMap(value.(map[string]interface{}))
			ExchangeMap[key] = true
		} else {
			ExchangeMap[key] = value
		}
	}
}

func parseJsonFromString(potentialJson string) (map[string]interface{}, error) {
	var unmarshalledJson map[string]interface{}
	jsonReader := json.NewDecoder(strings.NewReader(potentialJson))
	err := jsonReader.Decode(&unmarshalledJson)
	if err != nil {
		return map[string]interface{}{}, err
	} else {
		complexMapToSimpleExchangeMap(unmarshalledJson)
		finalReturnMap := ExchangeMap
		ExchangeMap = map[string]interface{}{}
		return finalReturnMap, err
	}
}

func GetRequestQueryValues(stringRequest string) map[string]interface{} {
	reader := bufio.NewReader(strings.NewReader(stringRequest))
	request, _ := http.ReadRequest(reader)
	query := request.URL.Query()
	queryMap := map[string]interface{}{}
	// TODO check how the complex map works in graylog when a json is unmarshaled into a map with multiple levels!
	// Multiple Levels dont work. Try with putting every key in the first level. If that doesn't work,
	// create manual types like the coordinate type! That seems to work. But before the one level solution
	for key, value := range query {
		if key == "coordinates" {
			routeCoordinates := processCoordinates(value[0])
			queryMap[key] = routeCoordinates
		} else {
			potentialJson, err := parseJsonFromString(value[0])
			if err == nil {
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
