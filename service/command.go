package service

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

//go:embed systemd.service.tmpl
var templateFS embed.FS

type CommandOptions struct {
	// Self uses the current executable as the service program path.
	Self bool
	// Config returns the current project config path. It is useful when the
	// parent command already owns a persistent --config flag.
	Config func() string
}

func Command(opts CommandOptions) *cobra.Command {
	var (
		command    string
		serviceName string
		user       string
		output     string
		config     string
		stdout     bool
		after      string
		restartSec string
	)

	use := "service <program-path>"
	long := "Generate systemd service installation script for the specified program."
	if opts.Self {
		use = "service"
		long = "Generate systemd service installation script for this program."
	}

	cmd := &cobra.Command{
		Use:   use,
		Short: "Generate systemd service installation script",
		Long:  long,
		Args: func(cmd *cobra.Command, args []string) error {
			if opts.Self {
				if len(args) != 0 {
					return fmt.Errorf("service does not accept a program path when used from the project binary")
				}
				return nil
			}
			if len(args) != 1 {
				return fmt.Errorf("please provide program path, for example: aqi service <program-path>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			currentDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("无法获取当前工作目录: %w", err)
			}

			progPath := ""
			if opts.Self {
				progPath, err = os.Executable()
				if err != nil {
					return fmt.Errorf("无法获取当前程序路径: %w", err)
				}
			} else {
				progPath = args[0]
			}

			if !filepath.IsAbs(progPath) {
				progPath, err = filepath.Abs(progPath)
				if err != nil {
					progPath = filepath.Join(currentDir, progPath)
				}
			}

			if _, err := os.Stat(progPath); os.IsNotExist(err) {
				return fmt.Errorf("程序路径不存在: %s", progPath)
			}

			friendlyName := strings.Split(filepath.Base(progPath), "-")[0]
			if friendlyName == "" {
				friendlyName = "app"
			}

			configPath := config
			if opts.Self && opts.Config != nil {
				configPath = opts.Config()
			}
			if configPath == "" {
				configPath = "config.yaml"
			}
			if !filepath.IsAbs(configPath) {
				configPath = filepath.Join(currentDir, configPath)
			}
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				fmt.Printf("错误: 配置文件路径不存在: %s\n", configPath)
			}

			name := serviceName
			if name == "" {
				name = fmt.Sprintf("%s_%s.service", friendlyName, command)
			} else {
				name = strings.TrimSuffix(name, ".service") + ".service"
			}

			outputPath := output
			if outputPath == "" {
				outputPath = filepath.Join(currentDir, name)
			} else if info, err := os.Stat(outputPath); err == nil && info.IsDir() {
				outputPath = filepath.Join(outputPath, name)
			}

			if !stdout {
				if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
					return fmt.Errorf("无法创建输出目录: %w", err)
				}
			}

			restart := restartSec
			if !strings.HasSuffix(restart, "s") && !strings.HasSuffix(restart, "m") && !strings.HasSuffix(restart, "h") {
				restart += "s"
			}

			tmplContent, err := templateFS.ReadFile("systemd.service.tmpl")
			if err != nil {
				return fmt.Errorf("无法读取模板文件: %w", err)
			}
			tmpl, err := template.New("systemd.service").Parse(string(tmplContent))
			if err != nil {
				return fmt.Errorf("无法解析模板: %w", err)
			}

			data := struct {
				Description      string
				WorkingDirectory string
				ExecStart        string
				User             string
				After            string
				RestartSec       string
			}{
				Description:      fmt.Sprintf("%s %s service", friendlyName, command),
				WorkingDirectory: currentDir,
				ExecStart:        fmt.Sprintf("%s %s -c=%s", progPath, command, configPath),
				User:             user,
				After:            after,
				RestartSec:       restart,
			}

			if stdout {
				if err := tmpl.Execute(os.Stdout, data); err != nil {
					return fmt.Errorf("无法执行模板: %w", err)
				}
				return nil
			}

			outputFile, err := os.Create(outputPath)
			if err != nil {
				return fmt.Errorf("无法创建输出文件: %w", err)
			}
			defer outputFile.Close()

			if err := tmpl.Execute(outputFile, data); err != nil {
				return fmt.Errorf("无法执行模板: %w", err)
			}

			fmt.Printf("✓ 服务脚本生成成功: %s\n\n", outputPath)
			fmt.Println("请执行以下操作安装服务：")
			fmt.Printf("  cp %s /usr/lib/systemd/system/\n", outputPath)
			fmt.Println("  sudo systemctl daemon-reload")
			fmt.Printf("  sudo systemctl start %s\n", name)
			fmt.Printf("  sudo systemctl enable %s\n", name)
			fmt.Println("\n查看服务状态：")
			fmt.Printf("  sudo systemctl status %s\n", name)
			fmt.Println("\n重新启动服务：")
			fmt.Printf("  sudo systemctl restart %s\n", name)
			fmt.Println("\n停止服务：")
			fmt.Printf("  sudo systemctl stop %s\n", name)
			fmt.Println("\n禁用服务：")
			fmt.Printf("  sudo systemctl disable %s\n", name)
			return nil
		},
	}

	if opts.Self {
		cmd.Flags().StringVar(&command, "command", "api", "子命令名称")
	} else {
		cmd.Flags().StringVarP(&command, "command", "c", "api", "子命令名称")
		cmd.Flags().StringVar(&config, "config", "config.yaml", "配置文件路径（用于ExecStart）")
	}
	cmd.Flags().StringVarP(&serviceName, "service", "s", "", "服务名称（可选，默认自动生成）")
	cmd.Flags().StringVarP(&user, "user", "u", "root", "运行用户")
	cmd.Flags().StringVarP(&output, "output", "o", "", "输出文件路径（可选，默认当前目录）")
	cmd.Flags().BoolVar(&stdout, "stdout", false, "直接输出到标准输出，不写文件")
	cmd.Flags().StringVar(&after, "after", "syslog.target network.target mysql.service", "依赖服务列表")
	cmd.Flags().StringVar(&restartSec, "restart-sec", "5", "重启间隔（秒）")

	return cmd
}
