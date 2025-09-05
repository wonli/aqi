package uidgen

import (
	"fmt"
	"strconv"
	"sync/atomic"
	"time"
)

// 全局原子计数器
var counter uint64

// GenId 并发安全的生成 16 位整型ID
func GenId() int64 {
	idStr := GenSid()
	id, _ := strconv.ParseInt(idStr, 10, 64)
	return id
}

// GenSid 并发安全的生成 16 位字符串ID
func GenSid() string {
	now := time.Now()
	yearPart := 100 + (now.Year() - 2025)
	dayPart := now.YearDay() + 521
	secsOfDay := now.Hour()*3600 + now.Minute()*60 + now.Second()

	subSecondPart := now.Nanosecond() / 1e5
	base := uint64(secsOfDay)*100000 + uint64(subSecondPart)
	seq := atomic.AddUint64(&counter, 1)
	if seq < base {
		atomic.CompareAndSwapUint64(&counter, seq, base)
		seq = base
	}

	return fmt.Sprintf("%03d%03d%010d", yearPart, dayPart, seq)
}
