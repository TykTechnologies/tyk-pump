package routes

import (
	"github.com/TykTechnologies/tyk-pump/analytics"
	"reflect"
	"strings"
)

var SimpleMapHolder = map[string]interface{}{}

func complexToSimpleMap(complexMap map[string]interface{}, prefix string) map[string]interface{} {
	for key, value := range complexMap {
		combinedPrefix := strings.TrimLeft(strings.Join([]string{prefix, key}, "_"), "_")
		if reflect.TypeOf(value) == reflect.TypeOf(map[string]interface{}{}) {
			SimpleMapHolder[combinedPrefix] = "True"
			complexToSimpleMap(value.(map[string]interface{}), combinedPrefix)
		} else if key == "coordinates" {
			coordinates := value.(RouteCoordinates)
			SimpleMapHolder["startLng"] = coordinates.StartLng
			SimpleMapHolder["endLat"] = coordinates.EndLat
			SimpleMapHolder["endLat"] = coordinates.EndLng
			if coordinates.ViaCoords != nil {
				SimpleMapHolder["viaCoords"] = coordinates.ViaCoords
			}
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
		orsRouteStats.Distance = distance
	}
	orsRouteStats.Data = complexToSimpleMap(values, "")
	return orsRouteStats
}
