package regx

import (
	"fmt"
	"time"
)

func ValidateTimes(times []string) error {
	if times == nil || len(times) != 2 {
		return fmt.Errorf("incorrect input time format")
	}

	startTime, err1 := time.Parse("15:04", times[0])
	endTime, err2 := time.Parse("15:04", times[1])

	if err1 != nil {
		return fmt.Errorf("invalid start time format: %v", err1)
	}

	if err2 != nil {
		return fmt.Errorf("invalid end time format: %v", err2)
	}

	// Check if start time is after end time
	if startTime.After(endTime) {
		return fmt.Errorf("start time cannot be later than end time")
	}

	return nil
}
