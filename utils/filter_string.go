package utils

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wonli/aqi/logger"
)

func FilterStringId(stringIds string, filterId int) string {
	var result []int
	strArray := strings.Split(stringIds, ",")
	for _, strId := range strArray {
		strIdIntVal, err := strconv.Atoi(strId)
		if err != nil {
			logger.SugarLog.Errorf("translate id error %s", err.Error())
			continue
		}

		if strIdIntVal != filterId {
			result = append(result, strIdIntVal)
		}
	}

	if len(result) > 0 {
		return arrayToString(result, ",")
	}

	return ""
}

func arrayToString(a []int, delim string) string {
	return strings.Trim(strings.Replace(fmt.Sprint(a), " ", delim, -1), "[]")
}
