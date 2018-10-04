package routes

import (
	"github.com/golang/geo/s2"
	"math"
	"strconv"
)

type RouteCoordinates struct {
	StartLatLng string
	EndLatLng   string
	StartLng    string
	StartLat    string
	EndLng      string
	EndLat      string
	ViaCoords   []map[string]interface{}
}

func RoundFloat64(val float64, roundOn float64, places int) (newVal float64) {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * val
	_, div := math.Modf(digit)
	if div >= roundOn {
		round = math.Ceil(digit)
	} else {
		round = math.Floor(digit)
	}
	newVal = round / pow
	return
}

func GetDistanceCategory(distance float64) string {
	switch {
	case distance >= 0 && distance < 10:
		return "0-10km"
	case distance >= 10 && distance < 20:
		return "10-20km"
	case distance >= 20 && distance < 50:
		return "20-50km"
	case distance >= 50 && distance < 100:
		return "50-100km"
	case distance >= 100 && distance < 500:
		return "100-500km"
	case distance >= 500 && distance < 1000:
		return "500-1000km"
	case distance >= 1000 && distance < 5000:
		return "1000-5000km"
	case distance >= 5000:
		return "> 5000km"
	}
	return ""
}

func GetEuclideanDistance(coordinates RouteCoordinates, decimals int) float64 {
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
		distanceInKM = RoundFloat64(distanceInKM, .5, decimals)
		return distanceInKM
	} else {
		return 0
	}
}
