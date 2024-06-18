package utils

import (
	"time"
)

func FriendlyTime(t time.Time) string {
	return Time(t)
}

func TodayStartAndEndTime() (beginTime, endTime uint) {
	timeStr := time.Now().Format("2006-01-02")
	t, _ := time.ParseInLocation("2006-01-02", timeStr, time.Local)
	beginTime = uint(t.Unix())
	endTime = beginTime + 86400

	return beginTime, endTime
}
