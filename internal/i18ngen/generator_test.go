package i18ngen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	root := t.TempDir()
	source := `package router

func Actions() {
	app.Add("user.login", func(a *Context) {
		a.SendCode(400, "登录失败")
		a.SendCode(code, "动态 code")
		a.SendCode(401, dynamicMessage)
		a.SendCode(402, fmt.Sprintf("超级管理员 %s", name))
	})
	app.Add("user.profile", profile)
}

func profile(a *Context) {
	a.SendCode(404, "用户不存在")
}
`
	if err := os.WriteFile(filepath.Join(root, "router.go"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}

	count, output, changed, err := Generate(root, "data", "zh")
	if err != nil {
		t.Fatal(err)
	}
	if count != 4 || !changed {
		t.Fatalf("count=%d changed=%v, want 4 true", count, changed)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "user.login.400: 登录失败") || !strings.Contains(text, "user.profile.404: 用户不存在") {
		t.Fatalf("unexpected catalog:\n%s", text)
	}
	if !strings.Contains(text, "user.login.401:\n") || !strings.Contains(text, "user.login.402:\n") {
		t.Fatalf("dynamic messages should keep empty placeholders:\n%s", text)
	}
	if strings.Contains(text, "动态 code") {
		t.Fatalf("dynamic code should be ignored:\n%s", text)
	}

	// Manual values are business-owned and must survive regeneration.
	text = strings.Replace(text, "user.login.402:\n", "user.login.402: 超级管理员\n", 1)
	if err := os.WriteFile(output, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, changed, err = Generate(root, "data", "zh")
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("generation should preserve existing manual values")
	}
}

func TestGenerateRejectsDuplicateKey(t *testing.T) {
	root := t.TempDir()
	source := `package router
func Actions() {
	app.Add("user.login", func(a *Context) {
		a.SendCode(400, "登录失败")
		a.SendCode(400, "登录失败")
	})
}`
	if err := os.WriteFile(filepath.Join(root, "router.go"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := Generate(root, "data", "zh")
	if err == nil || !strings.Contains(err.Error(), "duplicate i18n key: user.login.400") {
		t.Fatalf("Generate() error=%v, want duplicate key", err)
	}
}

func TestGenerateRejectsDuplicateDynamicKey(t *testing.T) {
	root := t.TempDir()
	source := `package router
func Actions() {
	app.Add("hi", func(a *Context) {
		a.SendCode(1001, fmt.Sprintf("超级管理员 %s", name))
		if name == "ideaa" {
			a.SendCode(1001, fmt.Sprintf("超级管理员 %s", name))
		}
	})
}`
	if err := os.WriteFile(filepath.Join(root, "router.go"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := Generate(root, "data", "zh")
	if err == nil || !strings.Contains(err.Error(), "duplicate i18n key: hi.1001") {
		t.Fatalf("Generate() error=%v, want duplicate dynamic key", err)
	}
}

func TestCheck(t *testing.T) {
	root := t.TempDir()
	source := `package router
func Actions() {
	app.Add("user.login", func(a *Context) {
		a.SendCode(400, "登录失败")
		a.SendCode(401, dynamicMessage)
	})
}`
	if err := os.WriteFile(filepath.Join(root, "router.go"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := Generate(root, "data", "zh"); err != nil {
		t.Fatal(err)
	}
	catalogDir := filepath.Join(root, "data", "i18n")
	if err := os.WriteFile(filepath.Join(catalogDir, "en.yaml"), []byte("other.key.1: old\n"), 0644); err != nil {
		t.Fatal(err)
	}
	warnings, err := Check(root, "data", "zh")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "user.login.401: dynamic message needs manual translation") ||
		!strings.Contains(joined, "en: 2 missing translations") ||
		!strings.Contains(joined, "en: 1 orphan keys") {
		t.Fatalf("warnings=%q", joined)
	}
}
