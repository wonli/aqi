package utils

import (
	"strconv"
	"strings"
)

func StringToIntSlice(str string) []uint {
	parts := strings.Split(str, ",")
	seen := make(map[uint]bool)
	nums := make([]uint, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		num, err := strconv.Atoi(part)
		if err != nil {
			continue
		}

		uNum := uint(num)
		if !seen[uNum] {
			nums = append(nums, uNum)
			seen[uNum] = true
		}
	}

	return nums
}

func IntSliceToString(nums ...uint) string {
	var strBuilder strings.Builder
	seen := make(map[uint]bool)
	for i, num := range nums {
		if seen[num] {
			continue
		}

		if i > 0 {
			strBuilder.WriteString(",")
		}
		strBuilder.WriteString(strconv.FormatUint(uint64(num), 10))
		seen[num] = true
	}
	return strBuilder.String()
}
