package routes

import (
	"github.com/golang/geo/s2"
	"strconv"
)

type RouteCoordinates struct {
	//viaCoords := list.New()
	//viaCoords.PushBack(0.00)
	//coordinates := refererCoordinates{0, 0, 0, 0, viaCoords}
	//Interface to store any type in here!
	StartLng  string
	StartLat  string
	EndLng    string
	EndLat    string
	ViaCoords []map[string]interface{}
}

func GetEuclideanDistance(coordinates RouteCoordinates) float64 {
	fromLat, ok1 := strconv.ParseFloat(coordinates.StartLat, 64)
	fromLng, ok2 := strconv.ParseFloat(coordinates.StartLng, 64)
	toLat, ok3 := strconv.ParseFloat(coordinates.EndLat, 64)
	toLng, ok4 := strconv.ParseFloat(coordinates.EndLng, 64)
	if ok1 == nil && ok2 == nil && ok3 == nil && ok4 == nil {
		var earthLength float64 = 6378.388
		fromLatLngObject := s2.LatLngFromDegrees(fromLat, fromLng)
		toLatLngObject := s2.LatLngFromDegrees(toLat, toLng)
		fromPoint := s2.PointFromLatLng(fromLatLngObject)
		toPoint := s2.PointFromLatLng(toLatLngObject)

		distanceInAngle := fromPoint.Distance(toPoint)
		distanceInRad := distanceInAngle.Radians()
		distanceInKM := earthLength * distanceInRad
		return distanceInKM
	} else {
		return 0
	}
}
