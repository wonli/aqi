package cli

import (
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var packageNameFlag string

var newCmd = &cobra.Command{
	Use:   "new [project-name]",
	Short: "Create new project skeleton",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		projectName := args[0]

		packageName := projectName
		if packageNameFlag != "" {
			packageName = packageNameFlag
		}

		createDir(projectName)
		createProject(projectName, packageName)
	},
}

func init() {
	newCmd.Flags().StringVarP(&packageNameFlag, "package", "p", "", "Specify package name (optional, defaults to project name)")
	rootCmd.AddCommand(newCmd)
}

func createDir(projectName string) {
	// 检查目录是否存在
	dirs := []struct {
		dirName string
	}{
		{"cmd"},
		{"internal/app"},
		{"internal/dbc"},
		{"internal/entity"},
		{"internal/middlewares"},
		{"internal/router"},
	}

	for _, d := range dirs {
		dirPath := filepath.Join(projectName, d.dirName)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			fmt.Printf("Error creating directory: %v\n", err)
			os.Exit(1)
		}
	}
}

func createProject(name, packageName string) {
	// 复制模板文件
	templates := []struct {
		templatePath string
		outputPath   string
	}{
		{"templates/default/main.go.tmpl", "main.go"},
		{"templates/default/go.mod.tmpl", "go.mod"},
		{"templates/default/makefile.tmpl", "Makefile"},
		{"templates/default/cmd/api.go.tmpl", "cmd/api.go"},
		{"templates/default/cmd/dal.go.tmpl", "cmd/dal.go"},
		{"templates/default/cmd/boot.go.tmpl", "cmd/boot.go"},
		{"templates/default/dbc/dbc.go.tmpl", "internal/dbc/dbc.go"},
		{"templates/default/middlewares/app.go.tmpl", "internal/middlewares/app.go"},
		{"templates/default/middlewares/recovery.go.tmpl", "internal/middlewares/recovery.go"},
		{"templates/default/middlewares/cors.go.tmpl", "internal/middlewares/cors.go"},
		{"templates/default/router/action.go.tmpl", "internal/router/action.go"},
		{"templates/default/router/api.go.tmpl", "internal/router/api.go"},
	}

	for _, t := range templates {
		// 从嵌入的文件系统读取模板
		tmplContent, err := templateFS.ReadFile(t.templatePath)
		if err != nil {
			fmt.Printf("Error reading template: %v\n", err)
			os.Exit(1)
		}

		// 解析模板
		tmpl, err := template.New(t.outputPath).Parse(string(tmplContent))
		if err != nil {
			fmt.Printf("Error parsing template: %v\n", err)
			os.Exit(1)
		}

		outputFile, err := os.Create(filepath.Join(name, t.outputPath))
		if err != nil {
			fmt.Printf("Error creating file: %v\n", err)
			os.Exit(1)
		}

		data := struct {
			PackageName string
		}{
			PackageName: packageName,
		}

		if err := tmpl.Execute(outputFile, data); err != nil {
			fmt.Printf("Error executing template: %v\n", err)
			os.Exit(1)
		}

		_ = outputFile.Close()
	}

	// 执行 go mod tidy 整理依赖
	if err := runGoModTidy(name); err != nil {
		fmt.Printf("Warning: Failed to run go mod tidy: %v\n", err)
	}

	fmt.Printf("Successfully created project: %s\n", name)
}

// runGoModTidy 在指定目录执行 go mod tidy
func runGoModTidy(projectDir string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
