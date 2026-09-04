package ws

import "testing"

func TestContextLanguage(t *testing.T) {
	client := &Client{}
	ctx := &Context{
		Client:     client,
		language:   "en-US",
		defaultLng: "zh",
	}

	if got := ctx.Language(); got != "en-US" {
		t.Fatalf("Language() = %q, want en-US", got)
	}

	ctx.SetLanguage("zh-TW")
	if got := ctx.Language(); got != "zh-TW" {
		t.Fatalf("Language() after SetLanguage = %q, want zh-TW", got)
	}
	if got := client.Language(); got != "zh-TW" {
		t.Fatalf("client Language() = %q, want zh-TW", got)
	}
}

func TestContextLanguageFallback(t *testing.T) {
	ctx := &Context{defaultLng: "en"}
	if got := ctx.Language(); got != "en" {
		t.Fatalf("Language() = %q, want en", got)
	}
}
