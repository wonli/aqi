package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

func TestProjectDirName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "myapp", want: "myapp"},
		{input: "github.com/a/b", want: "b"},
		{input: "github.com/a/b/", want: "b"},
		{input: `github.com\a\b`, want: "b"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := projectDirName(tt.input); got != tt.want {
				t.Fatalf("projectDirName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCreateProjectUsesAppNameInMakefile(t *testing.T) {
	tmplContent, err := os.ReadFile(filepath.Join("templates", "default", "makefile.tmpl"))
	if err != nil {
		t.Fatalf("read makefile template: %v", err)
	}

	tmpl, err := template.New("Makefile").Parse(string(tmplContent))
	if err != nil {
		t.Fatalf("parse makefile template: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, newProjectTemplateData("b", "github.com/a/b")); err != nil {
		t.Fatalf("execute makefile template: %v", err)
	}

	if !strings.Contains(buf.String(), "APP_NAME = b") {
		t.Fatalf("Makefile APP_NAME mismatch:\n%s", buf.String())
	}
}

func TestCreateProjectIncludesGitignoreTemplate(t *testing.T) {
	tmplContent, err := os.ReadFile(filepath.Join("templates", "default", "gitignore.tmpl"))
	if err != nil {
		t.Fatalf("read gitignore template: %v", err)
	}

	content := string(tmplContent)
	for _, want := range []string{".idea/", ".vscode/", ".fleet/", ".zed/"} {
		if !strings.Contains(content, want) {
			t.Fatalf("gitignore template missing %q:\n%s", want, content)
		}
	}

	found := false
	for _, tmpl := range projectTemplates() {
		if tmpl.outputPath == ".gitignore" && tmpl.templatePath == "templates/default/gitignore.tmpl" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("project templates do not include .gitignore output")
	}
}
