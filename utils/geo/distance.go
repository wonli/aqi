package geo

import (
	"math"
	"strconv"
)

// Distance represents a spacial distance. Fundamentally, the underlying float64 represents the raw
// number of meters
type Distance float64

// MilesPerKilometer Constants for conversions
const MilesPerKilometer = 0.6213712

// Standard length constants
const (
	Millimeter = Distance(0.001)
	Centimeter = Distance(0.01)
	Meter      = Distance(1)
	Kilometer  = Distance(1000)
	Mile       = Distance(1 / MilesPerKilometer * 1000)
)

// Millimeters gets the number of total millimeters represented by the distance
func (d Distance) Millimeters() float64 {
	return d.Meters() * 1000
}

// Centimeters gets the number of total centimeters represented by the distance
func (d Distance) Centimeters() float64 {
	return d.Meters() * 100
}

// Meters gets the number of total meters represented by the distance
func (d Distance) Meters() float64 {
	return float64(d)
}

// Kilometers gets the number of total kilometers represented by the distance
func (d Distance) Kilometers() float64 {
	return float64(d) / 1000
}

// Miles gets the number of total miles represented by the distance
func (d Distance) Miles() float64 {
	return d.Kilometers() * MilesPerKilometer
}

// String implements Stringer and returns a formatted string representation of the distance
func (d Distance) String() string {
	if d < 0.01 {
		return strconv.FormatFloat(d.Millimeters(), 'f', 2, 64) + "mm"
	}

	if d < 1 {
		return strconv.FormatFloat(d.Centimeters(), 'f', 2, 64) + "cm"
	}

	if d < 100 {
		return strconv.FormatFloat(d.Meters(), 'f', 2, 64) + "m"
	}

	return strconv.FormatFloat(d.Kilometers(), 'f', 2, 64) + "km"
}

func (d Distance) Equals(other, tolerance Distance) bool {
	difference := math.Abs(float64(d - other))
	return difference <= float64(tolerance)
}
