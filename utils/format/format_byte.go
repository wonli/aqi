package format

import (
	"fmt"
	"math"
)

func Bites(size float64) string {
	unit := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	s := math.Floor(math.Log(size) / math.Log(1024))
	i := int(s)

	if i < len(unit) {
		return fmt.Sprintf("%.2f %s", size/math.Pow(1024, s), unit[i])
	}

	return fmt.Sprintf("%f %s", size, unit[0])
}
