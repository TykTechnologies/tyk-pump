package ors

import (
	"bufio"
	"container/list"
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

func processViaCoordinates(viaCoordinates interface{}) interface{} {
	// interface to string?
	splittedCoordinates := strings.SplitAfter(viaCoordinates, ",")
	viaCoords := list.New()
	coordinatePair := map[string]interface{}{}
	coordinateCounter := 0
	for coordinate, _ := range splittedCoordinates {
		cleanedCoordinate := strings.TrimRight(coordinate, ",")
		if coordinateCounter%2 == 0 {
			coordinatePair["lat"] = cleanedCoordinate
		} else {
			coordinatePair["lng"] = cleanedCoordinate
			viaCoords.PushBack(coordinatePair)
			coordinatePair = map[string]interface{}{}
		}
	}
	return viaCoords
}

func getCoordinatesFromReferer(referer string) RefererCoordinates {
	refererMap := requestRefererToParameterMap(referer)
	coordinates := RefererCoordinates{}
	if _, ok := refererMap["n1"]; ok {
		if _, ok2 := refererMap["n2"]; ok2 {
			coordinates.startLat = refererMap["n1"]
			coordinates.endLong = refererMap["n1"]
		}
		if _, ok3 := refererMap["a"]; ok3 {
			viaCoordinatesString := refererMap["a"]
			viaCoordinates := processViaCoordinates(viaCoordinatesString)
			coordinates.viaCoords = viaCoordinates
			endCoordinates := //last variable of viaCoordinates
		}
	}
	return coordinates
}

func GetRequestQueryValues(stringRequest string) map[string]interface{} {
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
		coordinates := getCoordinatesFromReferer(referer)
		queryMap["coordinates"] = coordinates
	}
	return queryMap
}

func ProcessDecodedRawRequest(decodedRawRequest []byte) map[string]interface{} {
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

func ProcessRawRequestToOrsRouteStats(rawEncodedRequest string) map[string]interface{} {
	decodedRawReq, _ := base64.StdEncoding.DecodeString(rawEncodedRequest)
	processedRequest := ProcessDecodedRawRequest(decodedRawReq)
	return processedRequest
}

func CalculateOrsRouteStats(analyticsRecord AnalyticsRecord) map[string]interface{} {
	analyticsRecord = analyticsRecord
	method := analyticsRecord.Method
	var orsRouteStats map[string]interface{}{}
	if method == "GET" {
		rawRequest := analyticsRecord.RawRequest
		orsRouteStats = ProcessRawRequestToOrsRouteStats(rawRequest)
		// analyticsRecord.OrsRouteStats = orsRouteStats
	} else if method == "POST" {
		log.WithFields(logrus.Fields{
			"prefix": mechanicsPrefix,
		}).Debug("Method not implemented: ", method)
	}

	return orsRouteStats
}
