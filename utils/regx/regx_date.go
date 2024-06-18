package regx

import (
	"fmt"
	"time"
)

func ValidateDates(dates []string) error {
	if dates == nil || len(dates) != 2 {
		return fmt.Errorf("please provide a start date and an end date")
	}

	startDate, err1 := time.Parse("2006-01-02", dates[0])
	endDate, err2 := time.Parse("2006-01-02", dates[1])

	if err1 != nil {
		return fmt.Errorf("invalid start date format: %v", err1)
	}

	if err2 != nil {
		return fmt.Errorf("onvalid end date format: %v", err2)
	}

	if startDate.After(endDate) {
		return fmt.Errorf("start date cannot be later than end date")
	}

	return nil
}
