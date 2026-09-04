package docview

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed api_viewer.html
var viewerFS embed.FS

type authConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Realm    string `yaml:"realm"`
}

type documentInfo struct {
	Name  string `yaml:"name" json:"name"`
	Label string `yaml:"label" json:"label"`
	File  string `yaml:"file" json:"file"`
}

type config struct {
	AppDocuments []documentInfo `yaml:"appDocuments"`
	Auth         authConfig     `yaml:"auth"`
}

// Handler returns an HTTP handler for embedded AQI API documentation.
// The project FS only needs to embed generated document files and doc-config.yaml.
func Handler(projectFS embed.FS) http.Handler {
	cfg := loadConfig(projectFS)
	viewer := renderViewer(cfg.AppDocuments)
	allowed := make(map[string]struct{}, len(cfg.AppDocuments))
	for _, doc := range cfg.AppDocuments {
		if doc.File != "" {
			allowed[path.Base(doc.File)] = struct{}{}
		}
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fileName := path.Base(path.Clean(r.URL.Path))
		if r.URL.Path == "" || strings.HasSuffix(r.URL.Path, "/") || fileName == "." {
			serveViewer(w, viewer)
			return
		}

		if _, ok := allowed[fileName]; !ok {
			http.NotFound(w, r)
			return
		}

		data, err := projectFS.ReadFile(fileName)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		if contentType := mime.TypeByExtension(path.Ext(fileName)); contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		_, _ = w.Write(data)
	})

	return basicAuthMiddleware(handler, cfg.Auth)
}

func loadConfig(projectFS embed.FS) config {
	data, err := projectFS.ReadFile("doc-config.yaml")
	if err != nil {
		return config{}
	}

	var cfg config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return config{}
	}
	return cfg
}

func renderViewer(documents []documentInfo) []byte {
	viewer, err := viewerFS.ReadFile("api_viewer.html")
	if err != nil {
		return []byte("API viewer unavailable")
	}

	docsJSON, err := json.Marshal(documents)
	if err != nil {
		docsJSON = []byte("[]")
	}

	return []byte(strings.ReplaceAll(string(viewer), "{{.DocsConfig}}", string(docsJSON)))
}

func serveViewer(w http.ResponseWriter, viewer []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(viewer)
}

func basicAuthMiddleware(handler http.Handler, auth authConfig) http.Handler {
	if !auth.Enabled {
		return handler
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if ok && secureEqual(username, auth.Username) && secureEqual(password, auth.Password) {
			handler.ServeHTTP(w, r)
			return
		}

		realm := auth.Realm
		if realm == "" {
			realm = "API 文档"
		}
		w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm=%q`, realm))
		http.Error(w, "Unauthorized.", http.StatusUnauthorized)
	})
}

func secureEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
