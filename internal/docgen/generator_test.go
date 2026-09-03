package docgen

import (
	"strings"
	"testing"
)

func TestGroupByActionPrefix(t *testing.T) {
	actions := []ActionDoc{
		{Name: "ping"},
		{Name: "user.login"},
		{Name: "user.logout"},
		{Name: "admin.user.list"},
	}

	groups := groupByActionPrefix(actions)
	if len(groups[""]) != 1 || groups[""][0].Name != "ping" {
		t.Fatalf("ungrouped actions = %#v", groups[""])
	}
	if len(groups["user"]) != 2 {
		t.Fatalf("user group size = %d, want 2", len(groups["user"]))
	}
	if len(groups["admin"]) != 1 || groups["admin"][0].Name != "admin.user.list" {
		t.Fatalf("admin actions = %#v", groups["admin"])
	}
}

func TestBuildJSONActionBuildsNestedRequiredParams(t *testing.T) {
	action := ActionDoc{
		Name:        "user.list",
		Description: "list users",
		Params: []ParamField{
			{Name: "page.current", Type: "int", Required: true},
			{Name: "page.size", Type: "int", Required: true},
			{Name: "keyword", Type: "string", Required: false},
		},
		Returns: ReturnType{SuccessType: "[]User", HasData: true},
	}

	got := buildJSONAction(action)
	request, ok := got.Example.Request.(map[string]interface{})
	if !ok {
		t.Fatalf("example request type = %T", got.Example.Request)
	}
	if request["action"] != "user.list" {
		t.Fatalf("example action = %#v", request["action"])
	}

	params, ok := request["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("params type = %T", request["params"])
	}
	page, ok := params["page"].(map[string]interface{})
	if !ok {
		t.Fatalf("page type = %T", params["page"])
	}
	if page["current"] != 0 || page["size"] != 0 {
		t.Fatalf("nested page example = %#v", page)
	}
	if _, exists := params["keyword"]; exists {
		t.Fatal("optional keyword unexpectedly included in example")
	}
}

func TestFormattingHelpers(t *testing.T) {
	if got := getFileTitle("user_action.go"); got != "User action API" {
		t.Fatalf("getFileTitle() = %q", got)
	}
	if got := formatReturns(ReturnType{SuccessType: "User", HasData: true}); got != "数据对象 (User)" {
		t.Fatalf("formatReturns() = %q", got)
	}
	if got := formatReturns(ReturnType{}); got != "状态码" {
		t.Fatalf("formatReturns(no data) = %q", got)
	}

	action := ActionDoc{
		Name: "user.get",
		Params: []ParamField{
			{Name: "id", Type: "uint", Required: true},
			{Name: "verbose", Type: "bool", Required: true},
			{Name: "optional", Type: "string", Required: false},
		},
	}
	example := formatExample(action)
	if !strings.Contains(example, `"id": 0`) || !strings.Contains(example, `"verbose": true`) {
		t.Fatalf("formatExample() = %q", example)
	}
	if strings.Contains(example, "optional") {
		t.Fatalf("optional parameter leaked into example: %q", example)
	}
}
