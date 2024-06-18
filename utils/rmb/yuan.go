package rmb

import (
	"fmt"
	"strconv"
	"strings"
)

func YuanToFen(yuan string, decimalPlaces float64) (int64, error) {
	parts := strings.Split(yuan, ".")
	integerPart, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, err
	}

	fen := integerPart * 100

	// If there is a decimal part
	if len(parts) > 1 {
		// Only get the first two decimal places, the rest will be ignored
		decimalPart := parts[1]
		if len(decimalPart) > 2 {
			decimalPart = decimalPart[:2]
		}

		decimalValue, err := strconv.ParseInt(decimalPart, 10, 64)
		if err != nil {
			return 0, err
		}

		// Decide whether to add 10 or 100 based on the number of decimal places
		if len(decimalPart) == 1 {
			fen += decimalValue * 10
		} else {
			fen += decimalValue
		}
	}

	// Calculate the percentage of the price
	fen = int64(float64(fen) * decimalPlaces)

	return fen, nil
}

func FloatYuanToFen(yuan float64, decimalPlaces int) (int64, error) {
	formatted := fmt.Sprintf("%.*f", decimalPlaces, yuan)

	// Remove the decimal point
	formatted = strings.Replace(formatted, ".", "", -1)

	// Convert the formatted string to int64
	fen, err := strconv.ParseInt(formatted, 10, 64)
	if err != nil {
		return 0, err
	}

	return fen, nil
}
