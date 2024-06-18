package utils

import (
	"strconv"
	"strings"
)

func String2Slice(s string) []string {
	slc := strings.Split(s, ",")
	for i := range slc {
		slc[i] = strings.TrimSpace(slc[i])
	}

	return slc
}

func UintsToString(numbers ...uint) string {
	str := make([]string, len(numbers))
	for i, num := range numbers {
		str[i] = strconv.FormatUint(uint64(num), 10)
	}

	return strings.Join(str, ",")
}
