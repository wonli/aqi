package i18n

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestManagerTranslate(t *testing.T) {
	dir := t.TempDir()
	catalogDir := filepath.Join(dir, "i18n")
	if err := os.MkdirAll(catalogDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(catalogDir, "en.yaml"), []byte("user.login.400: login failed\n"), 0644); err != nil {
		t.Fatal(err)
	}

	manager := New(dir, "zh-CN")
	if got := manager.DefaultLanguage(); got != "zh-cn" {
		t.Fatalf("DefaultLanguage() = %q, want zh-cn", got)
	}

	const (
		action = "user.login"
		code   = 400
		msg    = "登录失败"
	)
	if got := manager.Translate("zh-CN", action, code, msg); got != msg {
		t.Fatalf("Translate(default) = %q, want %q", got, msg)
	}
	if got := manager.Translate("en-US", action, code, msg); got != "login failed" {
		t.Fatalf("Translate(en-US) = %q, want login failed", got)
	}
	if got := manager.Translate("../../etc", action, code, msg); got != msg {
		t.Fatalf("Translate(invalid locale) = %q, want fallback %q", got, msg)
	}
}

func TestManagerConcurrentRead(t *testing.T) {
	dir := t.TempDir()
	catalogDir := filepath.Join(dir, "i18n")
	if err := os.MkdirAll(catalogDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(catalogDir, "en.yaml"), []byte("test.action.400: concurrent message\n"), 0644); err != nil {
		t.Fatal(err)
	}

	manager := New(dir, "zh")
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := manager.Translate("en", "test.action", 400, "并发消息"); got != "concurrent message" {
				t.Errorf("Translate(en) = %q, want concurrent message", got)
			}
		}()
	}
	wg.Wait()
}
