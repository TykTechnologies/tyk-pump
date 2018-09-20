package ors

import (
	"container/list"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/golang/geo/s2"
	"net/url"
)

type RefererCoordinates struct {
	//viaCoords := list.New()
	//viaCoords.PushBack(0.00)
	//coordinates := refererCoordinates{0, 0, 0, 0, viaCoords}
	//Interface to store any type in here!
	startLong float64
	startLat  float64
	endLong   float64
	endLat    float64
	viaCoords *list.List
}

func GetEuclideanDistance(fromLat float64, fromLng float64, toLat float64, toLng float64) float64 {
	var earthLength float64 = 6378.388
	fromLatLngObject := s2.LatLngFromDegrees(fromLat, fromLng)
	toLatLngObject := s2.LatLngFromDegrees(toLat, toLng)
	fromPoint := s2.PointFromLatLng(fromLatLngObject)
	toPoint := s2.PointFromLatLng(toLatLngObject)

	distanceInAngle := fromPoint.Distance(toPoint)
	distanceInRad := distanceInAngle.Radians()
	distanceInKM := earthLength * distanceInRad
	return distanceInKM
}

func GetLengthForRefererCoordinates(refererCoordinates RefererCoordinates) float64 {
	// Include via logic from refererCoordinates here!s
	return float64(0)
}

func CalculateOrsStats(requestContentAsList url.Values) analytics.OrsRouteStats {
	return analytics.OrsRouteStats{1, 0, 0, 0, 0}
}
