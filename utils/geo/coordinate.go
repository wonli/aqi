package geo

import (
	"fmt"
	"math"
	"strconv"
)

// Coordinate represents a specific location on Earth
type Coordinate struct {
	Lat, Lng float64
}

// Constants needed for distance calculations
const (
	EarthRadius       = 6371 * Kilometer
	DoubleEarthRadius = 2 * EarthRadius
	PiOver180         = math.Pi / 180
)

// DistanceBetween calculates the distance between two coordinates
func DistanceBetween(a, b Coordinate) Distance {
	value := 0.5 - math.Cos((b.Lat-a.Lat)*PiOver180)/2 + math.Cos(a.Lat*PiOver180)*math.Cos(b.Lat*PiOver180)*(1-math.Cos((b.Lng-a.Lng)*PiOver180))/2
	return DoubleEarthRadius * Distance(math.Asin(math.Sqrt(value)))
}

// DistanceTo calculates the distance from this coordinate to another coordinate
func (c Coordinate) DistanceTo(other Coordinate) Distance {
	return DistanceBetween(c, other)
}

// String implements Stringer, returns a string representation of the coordinate
func (c Coordinate) String() string {
	return fmt.Sprintf(
		"(%s, %s)",
		strconv.FormatFloat(c.Lat, 'f', -1, 64),
		strconv.FormatFloat(c.Lng, 'f', -1, 64),
	)
}
