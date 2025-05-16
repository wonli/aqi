package logger

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestZap(t *testing.T) {

	logFilters := map[string]string{
		"article.list":  "data:16",
		"res.uploadImg": "params:64",
	}

	filters := make([]FilterRule, 0, len(logFilters))
	for actionPattern, fieldRule := range logFilters {
		// 解析字段路径和长度（格式：field.path:maxLen）
		ruleParts := strings.SplitN(fieldRule, ":", 2)
		if len(ruleParts) != 2 {
			continue
		}

		fieldPath := strings.TrimSpace(ruleParts[0])
		maxLen, _ := strconv.Atoi(strings.TrimSpace(ruleParts[1]))
		if maxLen <= 0 {
			maxLen = 0
		}

		patternStr := fmt.Sprintf(`"action":"%s"`, regexp.QuoteMeta(actionPattern))
		pattern := regexp.MustCompile(patternStr)

		fieldPattern := fmt.Sprintf(`("%s":\s*)(.*)`, regexp.QuoteMeta(fieldPath))
		fieldRegex := regexp.MustCompile(fieldPattern)

		filters = append(filters, FilterRule{
			Action:     actionPattern,
			Field:      fieldPath,
			MaxLen:     maxLen,
			Pattern:    pattern,
			FieldRegex: fieldRegex,
		})
	}

	// 测试消息
	msg1 := `{"action":"res.uploadImg","params":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAYoAAAGICAYAAABftLn"}`
	msg2 := `{"code":0,"action":"article.list","data":{"page":{"current":1,"pageSize":10,"total":5}}}`

	fs := fileStyleEncoder{}

	// 处理消息
	processedMsg1 := fs.processMessage(msg1, filters)
	log.Println("处理后的消息1:", processedMsg1)

	processedMsg2 := fs.processMessage(msg2, filters)
	log.Println("处理后的消息2:", processedMsg2)
}
