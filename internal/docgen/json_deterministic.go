package docgen

import (
	"encoding/json/v2"
	"sort"
)

// MarshalJSON 保证示例请求中的 map 按键稳定输出，避免重复生成文档时产生无意义 diff。
func (e JSONExample) MarshalJSON() ([]byte, error) {
	type example JSONExample
	return json.Marshal(example(e), json.Deterministic(true))
}

// MarshalJSON 保证参数和错误码数组使用稳定顺序输出。
func (a JSONAction) MarshalJSON() ([]byte, error) {
	type action JSONAction

	stable := action(a)
	stable.Params = append([]ParamField(nil), a.Params...)
	sort.Slice(stable.Params, func(i, j int) bool {
		return stable.Params[i].Name < stable.Params[j].Name
	})
	stable.ErrorCodes = append([]ErrorCode(nil), a.ErrorCodes...)
	sort.Slice(stable.ErrorCodes, func(i, j int) bool {
		if stable.ErrorCodes[i].Code == stable.ErrorCodes[j].Code {
			return stable.ErrorCodes[i].Message < stable.ErrorCodes[j].Message
		}
		return stable.ErrorCodes[i].Code < stable.ErrorCodes[j].Code
	})

	return json.Marshal(stable, json.Deterministic(true))
}
