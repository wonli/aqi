package format

import (
	"fmt"
	"time"
)

type FriendTime struct {
	t         int64
	format    string
	startTime int64
	suffix    string
}

func NewFriendTime(unixTime int64) *FriendTime {
	return &FriendTime{
		t:      unixTime,
		format: "2006-01-02 15:04:05",
		suffix: "前",
	}
}

func (f *FriendTime) SetFormat(format string) {
	f.format = format
}

func (f *FriendTime) SetStartTime(startTime int64) {
	f.startTime = startTime
}

func (f *FriendTime) SetSuffix(suffix string) {
	f.suffix = suffix
}

func (f *FriendTime) Format() string {
	startTime := f.startTime
	if startTime == 0 {
		startTime = time.Now().Unix()
	}

	delta := startTime - f.t
	if delta < 63072000 {
		conf := []struct {
			Duration int64
			Label    string
		}{
			{31536000, "年"},
			{2592000, "个月"},
			{604800, "星期"},
			{86400, "天"},
			{3600, "小时"},
			{60, "分钟"},
			{1, "秒"},
		}

		for _, diff := range conf {
			if c := delta / diff.Duration; c != 0 {
				return fmt.Sprintf("%d%s%s", c, diff.Label, f.suffix)
			}
		}
	}

	return time.Unix(f.t, 0).Format(f.format)
}
