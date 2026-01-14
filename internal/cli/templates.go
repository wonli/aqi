package cli

import (
	"embed"
)

//go:embed templates/default/*.tmpl templates/default/**/*.tmpl
var templateFS embed.FS

//go:embed templates/service/*.tmpl
var serviceTemplateFS embed.FS
