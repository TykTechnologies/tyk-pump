package ors

import (
	"net/url"
	"strings"
)

var commaHex string = "%2C"
var pipeHex string = "%7C"
var jsonOpenHex string = "%7B"
var jsonCloseHex string = "%7D"

func processRefererCoordinates(coordinatesString string) RefererCoordinates {
	return RefererCoordinates{}
}

func processOptions(encodedOptionsJsonString string, profile string) {
	// Decode the json first with http and translate it to a map
	//https://blog.golang.org/json-and-go
}

func processRefererKeyValue(refererKeyValueObject map[string]interface{}) map[string]interface{} {
	// Do all the magic here plus add the length parameter
	var refererMap map[string]interface{}

	return refererMap
}

func splitVariablesToKeyValue(splittedCleanedReferer []string) map[string]interface{} {
	refererKeyValueMap := map[string]interface{}{}
	for _, parameter := range splittedCleanedReferer {
		splittedParameter := strings.SplitAfterN(parameter, "=", 2)
		key := splittedParameter[0]
		key = strings.TrimRight(key, "=")
		value := splittedParameter[1]
		refererKeyValueMap[key] = value
	}
	return refererKeyValueMap
}

func removeEndpointFromReferer(refererString string) string {
	if strings.Contains(refererString, "directions?") {
		refererStringWoEndpoint := strings.SplitAfterN(refererString, "?", 2)
		return refererStringWoEndpoint[1]
	} else {
		return refererString
	}
}

func splitAndCleanRefererToVariables(refererString string) []string {
	refererStringWoEndpoint := removeEndpointFromReferer(refererString)
	splittedRequestReferer := strings.SplitAfter(refererStringWoEndpoint, "&")
	for index, parameter := range splittedRequestReferer {
		parameter = strings.TrimRight(parameter, "&")
		splittedRequestReferer[index] = parameter
	}
	return splittedRequestReferer
}

// Processes e.g. unprocessed json values
func processQueryValues(values url.Values) url.Values {
	return values
}

//GetRequestRefererAsMap Split the request url here and return all the elements as a map
// Use refererCoordinates
func requestQueryToParameterMap(requestReferer string) map[string]interface{} {
	refererVariables := splitAndCleanRefererToVariables(requestReferer)
	refererKeyValueObject := splitVariablesToKeyValue(refererVariables)
	processedRefererMap := processRefererKeyValue(refererKeyValueObject)
	return processedRefererMap
}
