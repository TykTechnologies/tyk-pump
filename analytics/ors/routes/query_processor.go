package routes

import (
	"fmt"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"reflect"
	"strings"
)

var SimpleMapHolder = map[string]interface{}{}

func complexToSimpleMap(complexMap map[string]interface{}, prefix string) map[string]interface{} {
	for key, value := range complexMap {
		combinedPrefix := strings.TrimLeft(strings.Join([]string{prefix, key}, "_"), "_")
		if reflect.TypeOf(value) == reflect.TypeOf(map[string]interface{}{}) {
			SimpleMapHolder[combinedPrefix] = true
			complexToSimpleMap(value.(map[string]interface{}), combinedPrefix)
		} else if key == "coordinates" {
			coordinates := value.(RouteCoordinates)
			SimpleMapHolder["startLatLng"] = coordinates.StartLatLng
			SimpleMapHolder["endLatLng"] = coordinates.EndLatLng
			//if coordinates.ViaCoords != nil {
			//	SimpleMapHolder["viaCoords"] = coordinates.ViaCoords
			//}
		} else if strings.Contains(fmt.Sprint(value), "|") {
			pipeStrings := strings.Split(fmt.Sprint(value), "|")
			for _, pipeString := range pipeStrings {
				bracketlessPipeString := strings.Trim(pipeString, "[ && ]")
				spacelessPipeString := strings.TrimSpace(bracketlessPipeString)
				combinedPrefixWithPipe := strings.TrimLeft(strings.Join([]string{combinedPrefix, spacelessPipeString}, "_"), "_")
				SimpleMapHolder[combinedPrefixWithPipe] = true
			}
		} else if reflect.TypeOf(value) == reflect.TypeOf([]string{}) {
			SimpleMapHolder[combinedPrefix] = value.([]string)[0]

		} else {
			SimpleMapHolder[combinedPrefix] = value
		}
	}
	return SimpleMapHolder
}

// Processes e.g. unprocessed json values
// The question is, do i need to process coords here? analytics are already ready or analytics here?
func ProcessQueryValues(values map[string]interface{}) analytics.OrsRouteStats {
	orsRouteStats := analytics.OrsRouteStats{}
	// Coords are already processed here, calc length here?
	if coordinates, present := values["coordinates"]; present {
		coordinates := coordinates.(RouteCoordinates)
		distance := GetEuclideanDistance(coordinates, 2)
		distanceCategory := GetDistanceCategory(distance)
		SimpleMapHolder["distance"] = distance
		SimpleMapHolder["distance_category"] = distanceCategory

	}
	orsRouteStats.Data = complexToSimpleMap(values, "")
	SimpleMapHolder = map[string]interface{}{}
	return orsRouteStats
}
