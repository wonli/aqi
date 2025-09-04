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
	testCases := []struct {
		name        string
		logFilters  map[string]string
		messages    []string
		description string
	}{
		{
			name: "基础截断测试",
			logFilters: map[string]string{
				"article.list":  "data:116",
				"res.uploadImg": "params:64",
			},
			messages: []string{
				`{"action":"res.uploadImg","params":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAYoAAAGICAYAAABftLn"}`,
				`{"code":0,"action":"article.list","id":"92","data":[{"id":83,"status":1,"parentId":0,"position":"ai-diagnosis","icon":"https://cheguanjia-test.oss-cn-chengdu.aliyuncs.com/files/2011/cce5e12eb9e403b.png?rid=4384\u0026mime_type=image/png","searchIcon":"","tagType":2,"orderSort":5,"name":"AI诊断","questions":{"data":[],"rule":"random","random":5},"cards":{"steps":[]},"notes":"","createdTime":"2025-07-11T11:49:24+08:00","updatedTime":"2025-07-11T13:50:04+08:00"}]}`,
			},
			description: "测试基本的字符串和数组截断功能",
		},
		{
			name: "中文字符截断测试",
			logFilters: map[string]string{
				"user.info": "message:20",
			},
			messages: []string{
				`{"action":"user.info","message":"这是一个包含中文字符的很长的消息内容，用来测试中文字符的正确截断功能，确保不会从中文字符中间截断"}`,
				`{"action":"user.info","message":"Hello World! 这里混合了英文和中文字符，测试混合字符的截断效果"}`,
			},
			description: "测试中文字符安全截断，确保不会从中文字符中间截断",
		},
		{
			name: "边界条件测试",
			logFilters: map[string]string{
				"test.boundary": "content:0",  // 长度为0
				"test.small":    "content:5",  // 很小的长度
				"test.exact":    "content:10", // 刚好等于内容长度
			},
			messages: []string{
				`{"action":"test.boundary","content":"任何内容都会被截断"}`,
				`{"action":"test.small","content":"短内容测试"}`,
				`{"action":"test.exact","content":"1234567890"}`,  // 刚好10个字符
				`{"action":"test.exact","content":"12345678901"}`, // 11个字符，应该被截断
			},
			description: "测试边界条件：长度为0、很小长度、刚好等于内容长度等情况",
		},
		{
			name: "特殊字符测试",
			logFilters: map[string]string{
				"special.chars": "data:30",
			},
			messages: []string{
				`{"action":"special.chars","data":"包含特殊字符: \"引号\" \n换行 \t制表符 \\反斜杠 和其他特殊符号 @#$%^&*()"}`,
				`{"action":"special.chars","data":"JSON转义字符测试: \u0026 \u003c \u003e 以及 emoji 😀🎉🚀"}`,
				`{"action":"special.chars","data":"URL编码字符: %20 %3A %2F %3F 和HTML实体: &amp; &lt; &gt;"}`,
			},
			description: "测试包含特殊字符、转义字符、emoji等的内容截断",
		},
		{
			name: "复杂JSON结构测试",
			logFilters: map[string]string{
				"complex.nested": "payload:50",
				"complex.array":  "items:40",
			},
			messages: []string{
				`{"action":"complex.nested","payload":{"user":{"id":123,"name":"张三","profile":{"age":25,"city":"北京","hobbies":["读书","游泳","编程"]}},"timestamp":"2024-01-01T00:00:00Z"}}`,
				`{"action":"complex.array","items":[{"id":1,"name":"项目A","tags":["重要","紧急"]},{"id":2,"name":"项目B","tags":["普通"]},{"id":3,"name":"项目C","tags":["低优先级"]}]}`,
			},
			description: "测试复杂嵌套JSON结构的截断处理",
		},
		{
			name: "无匹配规则测试",
			logFilters: map[string]string{
				"match.rule": "content:20",
			},
			messages: []string{
				`{"action":"no.match","content":"这条消息不应该被截断，因为action不匹配任何规则"}`,
				`{"action":"match.rule","other_field":"这个字段不会被截断，因为字段名不匹配"}`,
				`{"different_action":"match.rule","content":"action字段名不同，不会匹配规则"}`,
			},
			description: "测试不匹配过滤规则的消息应该保持原样",
		},
		{
			name: "空值和null测试",
			logFilters: map[string]string{
				"empty.test": "value:10",
			},
			messages: []string{
				`{"action":"empty.test","value":""}`,
				`{"action":"empty.test","value":null}`,
				`{"action":"empty.test","value":"   "}`, // 只有空格
				`{"action":"empty.test"}`,               // 缺少字段
			},
			description: "测试空字符串、null值、纯空格和缺失字段的处理",
		},
		{
			name: "多字段规则测试",
			logFilters: map[string]string{
				"multi.field1": "title:15",
				"multi.field2": "description:25",
			},
			messages: []string{
				`{"action":"multi.field1","title":"这是一个很长的标题内容需要被截断","description":"这个描述不会被截断因为规则不匹配"}`,
				`{"action":"multi.field2","title":"这个标题不会被截断","description":"这是一个更长的描述内容，包含更多的详细信息和说明文字，应该被截断"}`,
			},
			description: "测试不同action对应不同字段的截断规则",
		},
		{
			name: "性能压力测试",
			logFilters: map[string]string{
				"perf.test": "large_data:100",
			},
			messages: []string{
				`{"action":"perf.test","large_data":"` + strings.Repeat("这是重复的内容用于测试大数据量的截断性能。", 100) + `"}`,
			},
			description: "测试大数据量的截断性能",
		},
		{
			name: "格式异常测试",
			logFilters: map[string]string{
				"format.test": "data:20",
			},
			messages: []string{
				`{"action":"format.test","data":"正常数据"}`,        // 正常JSON
				`{"action":"format.test","data":"包含\"引号\"的数据"}`, // 包含转义引号
				`{"action":"format.test","data":"包含,逗号的数据"}`,    // 包含逗号
				`{"action":"format.test","data":"包含}花括号的数据"}`,   // 包含花括号
				`{"action":"format.test","data":"包含]方括号的数据"}`,   // 包含方括号
			},
			description: "测试包含JSON特殊字符的数据处理",
		},
		{
			name: "负数和零长度测试",
			logFilters: map[string]string{
				"negative.test": "data:-5", // 负数长度
				"zero.test":     "data:0",  // 零长度
			},
			messages: []string{
				`{"action":"negative.test","data":"负数长度测试数据"}`,
				`{"action":"zero.test","data":"零长度测试数据"}`,
			},
			description: "测试负数和零长度的边界情况处理",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			log.Printf("\n=== %s ===\n%s\n", tc.name, tc.description)

			// 构建过滤规则
			filters := make([]FilterRule, 0, len(tc.logFilters))
			for actionPattern, fieldRule := range tc.logFilters {
				// 解析字段路径和长度（格式：field.path:maxLen）
				ruleParts := strings.SplitN(fieldRule, ":", 2)
				if len(ruleParts) != 2 {
					continue
				}

				fieldPath := strings.TrimSpace(ruleParts[0])
				maxLen, _ := strconv.Atoi(strings.TrimSpace(ruleParts[1]))
				if maxLen < 0 {
					maxLen = 0
				}

				patternStr := fmt.Sprintf(`"action":"%s"`, regexp.QuoteMeta(actionPattern))
				pattern := regexp.MustCompile(patternStr)

				filters = append(filters, FilterRule{
					Action:  actionPattern,
					Field:   fieldPath,
					MaxLen:  maxLen,
					Pattern: pattern,
				})
			}

			fs := fileStyleEncoder{}

			// 处理每条消息
			for i, msg := range tc.messages {
				originalLen := len(msg)
				processedMsg := fs.processMessage(msg, filters)
				processedLen := len(processedMsg)

				log.Printf("消息 %d (原长度: %d, 处理后长度: %d):\n", i+1, originalLen, processedLen)
				log.Printf("处理后: %s\n", processedMsg)

				// 验证截断是否正确
				if processedLen < originalLen {
					log.Printf("✓ 消息被成功截断\n")
				} else if processedLen == originalLen {
					log.Printf("- 消息长度未变化（可能未匹配规则或无需截断）\n")
				}
				log.Println()
			}
		})
	}
}

// 辅助函数：生成大量测试数据
func generateLargeTestData(size int) string {
	return strings.Repeat("测试数据", size)
}

// 基准测试：测试截断性能
func BenchmarkProcessMessage(b *testing.B) {
	logFilters := map[string]string{
		"perf.test": "data:100",
	}

	filters := make([]FilterRule, 0, len(logFilters))
	for actionPattern, fieldRule := range logFilters {
		ruleParts := strings.SplitN(fieldRule, ":", 2)
		if len(ruleParts) != 2 {
			continue
		}

		fieldPath := strings.TrimSpace(ruleParts[0])
		maxLen, _ := strconv.Atoi(strings.TrimSpace(ruleParts[1]))
		if maxLen < 0 {
			maxLen = 0
		}

		patternStr := fmt.Sprintf(`"action":"%s"`, regexp.QuoteMeta(actionPattern))
		pattern := regexp.MustCompile(patternStr)

		filters = append(filters, FilterRule{
			Action:  actionPattern,
			Field:   fieldPath,
			MaxLen:  maxLen,
			Pattern: pattern,
		})
	}

	fs := fileStyleEncoder{}
	testMsg := `{"action":"perf.test","data":"` + generateLargeTestData(1000) + `"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fs.processMessage(testMsg, filters)
	}
}
