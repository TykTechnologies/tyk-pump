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

var ExchangeMap map[string]interface{}

func init() {
	log.Formatter = new(prefixed.TextFormatter)
	ExchangeMap = map[string]interface{}{}
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
			var coordinates []string
			coordinates = append(coordinates, lat, lng)
			routeCoordinates.StartLatLng = strings.Join(coordinates, ",")
			routeCoordinates.StartLat = lat
			routeCoordinates.StartLng = lng
		} else if index == len(splittedCoordinates)-1 {
			var coordinates []string
			coordinates = append(coordinates, lat, lng)
			routeCoordinates.EndLatLng = strings.Join(coordinates, ",")
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

func parseJsonFromString(potentialJson string) (map[string]interface{}, error) {
	var unmarshalledJson map[string]interface{}
	jsonReader := json.NewDecoder(strings.NewReader(potentialJson))
	err := jsonReader.Decode(&unmarshalledJson)
	if err != nil {
		return map[string]interface{}{}, err
	} else {
		return unmarshalledJson, err
	}
}

func GetRequestQueryValues(stringRequest string) map[string]interface{} {
	reader := bufio.NewReader(strings.NewReader(stringRequest))
	request, _ := http.ReadRequest(reader)
	if request != nil {
		query := request.URL.Query()
		queryMap := map[string]interface{}{}
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
	return nil
}

func generateStatsFromRawGetReq(rawRequest string) OrsRouteStats {
	decodedRawReq, _ := base64.StdEncoding.DecodeString(rawRequest)
	decodedRawRequestString := string(decodedRawReq)
	requestQueryValues := GetRequestQueryValues(decodedRawRequestString)
	processedQueryValues := ProcessQueryValues(requestQueryValues)
	return processedQueryValues
}

func generateStatsFromRawPostReq(rawRequest string) {
}

func ProcessDirectionsRecordOrsRouteStats(analyticsRecod AnalyticsRecord) OrsRouteStats {
	method := analyticsRecod.Method
	rawRequest := analyticsRecod.RawRequest
	orsRouteStats := OrsRouteStats{}
	if method == "GET" {
		orsRouteStats = generateStatsFromRawGetReq(rawRequest)
	} else if method == "POST" {
		generateStatsFromRawPostReq(rawRequest)
		log.WithFields(logrus.Fields{
			"prefix": mechanicsPrefix,
		}).Debug("Method not implemented: ", method)
	}
	return orsRouteStats
}

func CalculateOrsStats(analyticsRecord AnalyticsRecord) AnalyticsRecord {
	path := strings.ToLower(analyticsRecord.Path)
	apiName := strings.ToLower(analyticsRecord.APIName)
	if path == "isochrones" || apiName == "isochrones" {
		return analyticsRecord
	} else if path == "matrix" || apiName == "matrix" {
		return analyticsRecord
	} else if path == "directions" || apiName == "directions" {
		analyticsRecord.OrsRouteStats = ProcessDirectionsRecordOrsRouteStats(analyticsRecord)
		return analyticsRecord
	} else if path == "geocodereverseforpublic" || apiName == "geocodereverseforpublic" {
		return analyticsRecord
	}
	return analyticsRecord
}
