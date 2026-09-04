package i18n

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type Manager struct {
	dataPath        string
	defaultLanguage string
	catalogs        sync.Map
}

type catalog struct {
	messages map[string]string
}

func New(dataPath, defaultLanguage string) *Manager {
	defaultLanguage = normalizeLanguage(defaultLanguage)
	if defaultLanguage == "" {
		defaultLanguage = "zh"
	}

	return &Manager{
		dataPath:        dataPath,
		defaultLanguage: defaultLanguage,
	}
}

func (m *Manager) DefaultLanguage() string {
	if m == nil || m.defaultLanguage == "" {
		return "zh"
	}
	return m.defaultLanguage
}

func (m *Manager) Translate(language, action string, code int, msg string) string {
	if m == nil || msg == "" {
		return msg
	}

	language = normalizeLanguage(language)
	if language == "" {
		language = m.DefaultLanguage()
	}
	if language == m.DefaultLanguage() {
		return msg
	}

	key := messageKey(action, code)
	for _, candidate := range languageFallbacks(language) {
		if candidate == m.DefaultLanguage() {
			return msg
		}
		if translated, ok := m.catalog(candidate).load(key); ok && translated != "" {
			return translated
		}
	}
	return msg
}

func (m *Manager) catalog(language string) *catalog {
	if value, ok := m.catalogs.Load(language); ok {
		return value.(*catalog)
	}

	loaded := loadCatalog(filepath.Join(m.dataPath, "i18n", language+".yaml"))
	actual, _ := m.catalogs.LoadOrStore(language, loaded)
	return actual.(*catalog)
}

func loadCatalog(filePath string) *catalog {
	loaded := &catalog{messages: make(map[string]string)}
	data, err := os.ReadFile(filePath)
	if err != nil || len(data) == 0 {
		return loaded
	}
	_ = yaml.Unmarshal(data, &loaded.messages)
	return loaded
}

func (c *catalog) load(key string) (string, bool) {
	value, ok := c.messages[key]
	return value, ok
}

func messageKey(action string, code int) string {
	return fmt.Sprintf("%s.%d", action, code)
}

func normalizeLanguage(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	language = strings.ReplaceAll(language, "_", "-")
	parts := strings.Split(language, "-")
	if len(parts) == 0 || len(parts[0]) < 2 || len(parts[0]) > 8 || !lettersOnly(parts[0]) {
		return ""
	}
	for _, part := range parts[1:] {
		if len(part) == 0 || len(part) > 8 || !lettersOrDigits(part) {
			return ""
		}
	}
	return strings.Join(parts, "-")
}

func languageFallbacks(language string) []string {
	fallbacks := []string{language}
	if i := strings.IndexByte(language, '-'); i > 0 {
		fallbacks = append(fallbacks, language[:i])
	}
	return fallbacks
}

func lettersOnly(value string) bool {
	for _, r := range value {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

func lettersOrDigits(value string) bool {
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}
