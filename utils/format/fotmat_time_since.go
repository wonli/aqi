package format

import (
	"fmt"
	"time"
)

func TimeSince(t time.Time) string {
	now := time.Now()
	duration := now.Sub(t)

	seconds := int(duration.Seconds())
	minutes := int(duration.Minutes())
	hours := int(duration.Hours())
	days := hours / 24

	if seconds < 1 {
		return fmt.Sprintf("%.2f秒", duration.Seconds())
	} else if seconds < 60 {
		return fmt.Sprintf("%d秒", seconds)
	} else if minutes < 60 {
		return fmt.Sprintf("%d分钟%d秒", minutes, seconds%60)
	} else if hours < 24 {
		return fmt.Sprintf("%d小时%d分钟", hours, minutes%60)
	} else {
		return fmt.Sprintf("%d天%d小时%d分钟", days, hours%24, minutes%60)
	}
}
