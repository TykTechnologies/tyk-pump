package routes

import (
	"github.com/TykTechnologies/tyk-pump/analytics"
	"strings"
)

var commaHex string = "%2C"
var pipeHex string = "%7C"
var jsonOpenHex string = "%7B"
var jsonCloseHex string = "%7D"

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

//GetRequestRefererAsMap Split the request url here and return all the elements as a map
// Use refererCoordinates
func requestRefererToParameterMap(requestReferer string) map[string]interface{} {
	refererVariables := splitAndCleanRefererToVariables(requestReferer)
	refererKeyValueObject := splitVariablesToKeyValue(refererVariables)
	return refererKeyValueObject
}

func processCoordinates(coordinates interface{}) map[string]interface{} {
	processedCoordinates := map[string]interface{}{}
	return processedCoordinates
}

// Processes e.g. unprocessed json values
// The question is, do i need to process coords here? analytics are already ready or analytics here?
func processQueryValues(values map[string]interface{}) analytics.OrsRouteStats {
	processedQueryValues := map[string]interface{}{}
	orsRouteStats := analytics.OrsRouteStats{}
	// Coords are already processed here, calc length here?
	if coordinates, present := ValueCollection["coordinates"]; present {
		processedCoordinates := processCoordinates(coordinates)
		processedQueryValues["coordinates"] = processedCoordinates
	}
	if options, present := ValueCollection["options"]; present {
		processedOptions := processOptions(options)
		processedQueryValues["options"] = processedOptions
	}
	orsRouteStats.Data = processedQueryValues
	return orsRouteStats
}
func processOptions(options interface{}) map[string]interface{} {
	processedOptions := map[string]interface{}{}
	return processedOptions
}
