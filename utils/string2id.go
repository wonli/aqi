package utils

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/exp/constraints"
)

// Integer 限制 T 只能是整数类型。
type Integer interface {
	constraints.Integer
}

// StringToSlice 将字符串转换为去重后的整数切片。
func StringToSlice[T Integer](str string) []T {
	parts := strings.Split(str, ",")
	seen := make(map[T]bool)
	nums := make([]T, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		num, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			continue
		}

		typedNum := T(num)
		if !seen[typedNum] {
			nums = append(nums, typedNum)
			seen[typedNum] = true
		}
	}

	return nums
}

func IntSliceToString[T Integer](nums ...T) string {
	var strBuilder strings.Builder
	seen := make(map[T]bool)
	for i, num := range nums {
		if seen[num] {
			continue
		}

		if i > 0 {
			strBuilder.WriteString(",")
		}

		strBuilder.WriteString(fmt.Sprintf("%d", num))
		seen[num] = true
	}
	return strBuilder.String()
}
