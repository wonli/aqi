package validate

import "testing"

func TestNormalCachesPerLocale(t *testing.T) {
	zh := Normal("zh-CN")
	if zh != Normal("zh") {
		t.Fatal("zh-CN and zh should share the same validator manager")
	}

	en := Normal("en-US")
	if en != Normal("en") {
		t.Fatal("en-US and en should share the same validator manager")
	}
	if en == zh {
		t.Fatal("different locales should not share the same validator manager")
	}
}

func TestNormalizeLocaleFallback(t *testing.T) {
	if got := normalizeLocale("zh-TW"); got != "zh" {
		t.Fatalf("normalizeLocale(zh-TW) = %q, want zh", got)
	}
	if got := normalizeLocale("fr-FR"); got != "en" {
		t.Fatalf("normalizeLocale(fr-FR) = %q, want en fallback", got)
	}
}
