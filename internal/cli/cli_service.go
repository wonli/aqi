package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

var (
	serviceCommandFlag    string
	serviceNameFlag       string
	serviceUserFlag       string
	serviceOutputFlag     string
	serviceConfigFlag     string
	serviceStdoutFlag     bool
	serviceAfterFlag      string
	serviceRestartSecFlag string
)

var serviceCmd = &cobra.Command{
	Use:   "service <program-path>",
	Short: "Generate systemd service installation script",
	Long:  "Generate systemd service installation script for the specified program.",
	Run: func(cmd *cobra.Command, args []string) {
		generateServiceScript(args)
	},
}

func init() {
	serviceCmd.Flags().StringVarP(&serviceCommandFlag, "command", "c", "api", "子命令名称")
	serviceCmd.Flags().StringVarP(&serviceNameFlag, "service", "s", "", "服务名称（可选，默认自动生成）")
	serviceCmd.Flags().StringVarP(&serviceUserFlag, "user", "u", "root", "运行用户")
	serviceCmd.Flags().StringVarP(&serviceOutputFlag, "output", "o", "", "输出文件路径（可选，默认当前目录）")
	serviceCmd.Flags().StringVar(&serviceConfigFlag, "config", "config.yaml", "配置文件路径（用于ExecStart）")
	serviceCmd.Flags().BoolVar(&serviceStdoutFlag, "stdout", false, "直接输出到标准输出，不写文件")
	serviceCmd.Flags().StringVar(&serviceAfterFlag, "after", "syslog.target network.target mysql.service", "依赖服务列表")
	serviceCmd.Flags().StringVar(&serviceRestartSecFlag, "restart-sec", "5", "重启间隔（秒）")
	rootCmd.AddCommand(serviceCmd)
}

func generateServiceScript(args []string) {
	// 检查参数
	if len(args) == 0 {
		fmt.Printf("错误: 请提供程序路径作为参数，例如: aqi service <program-path>\n")
		os.Exit(1)
	}

	// 获取当前工作目录
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("错误: 无法获取当前工作目录: %v\n", err)
		os.Exit(1)
	}

	// 确定程序路径（从参数获取）
	progPath := args[0]
	if !filepath.IsAbs(progPath) {
		// 如果是相对路径，转换为绝对路径
		absPath, err := filepath.Abs(progPath)
		if err == nil {
			progPath = absPath
		} else {
			// 如果转换失败，尝试在当前目录查找
			progPath = filepath.Join(currentDir, args[0])
		}
	}

	// 从程序路径提取友好名称
	friendlyName := ""
	progName := filepath.Base(progPath)
	splitNames := strings.Split(progName, "-")
	if len(splitNames) > 0 {
		friendlyName = splitNames[0]
	}
	if friendlyName == "" {
		friendlyName = "app"
	}

	// 检查程序路径是否存在
	if _, err := os.Stat(progPath); os.IsNotExist(err) {
		fmt.Printf("错误: 程序路径不存在: %s\n", progPath)
		os.Exit(1)
	}

	// 确定配置文件路径
	configPath := serviceConfigFlag
	if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(currentDir, configPath)
	}

	// 检查配置文件路径是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("错误: 配置文件路径不存在: %s\n", configPath)
	}

	// 生成 Description 和 ExecStart
	description := fmt.Sprintf("%s %s service", friendlyName, serviceCommandFlag)
	execStart := fmt.Sprintf("%s %s -c=%s", progPath, serviceCommandFlag, configPath)

	// 确定服务名称
	serviceName := serviceNameFlag
	if serviceName == "" {
		serviceName = fmt.Sprintf("%s_%s.service", friendlyName, serviceCommandFlag)
	} else {
		serviceName = strings.TrimSuffix(serviceName, ".service") + ".service"
	}

	// 确定输出路径
	outputPath := serviceOutputFlag
	if outputPath == "" {
		outputPath = filepath.Join(currentDir, serviceName)
	} else {
		// 如果输出路径是目录，则添加文件名
		if info, err := os.Stat(outputPath); err == nil && info.IsDir() {
			outputPath = filepath.Join(outputPath, serviceName)
		}
	}

	// 检查输出目录是否可写
	outputDir := filepath.Dir(outputPath)
	if !serviceStdoutFlag {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			fmt.Printf("错误: 无法创建输出目录: %v\n", err)
			os.Exit(1)
		}
	}

	// 读取模板文件
	tmplContent, err := serviceTemplateFS.ReadFile("templates/service/systemd.service.tmpl")
	if err != nil {
		fmt.Printf("错误: 无法读取模板文件: %v\n", err)
		os.Exit(1)
	}

	// 解析模板
	tmpl, err := template.New("systemd.service").Parse(string(tmplContent))
	if err != nil {
		fmt.Printf("错误: 无法解析模板: %v\n", err)
		os.Exit(1)
	}

	// 处理 RestartSec，确保有单位
	restartSec := serviceRestartSecFlag
	if !strings.HasSuffix(restartSec, "s") && !strings.HasSuffix(restartSec, "m") && !strings.HasSuffix(restartSec, "h") {
		restartSec = restartSec + "s"
	}

	// 准备模板数据
	tmplData := struct {
		Description      string
		WorkingDirectory string
		ExecStart        string
		User             string
		After            string
		RestartSec       string
	}{
		Description:      description,
		WorkingDirectory: currentDir,
		ExecStart:        execStart,
		User:             serviceUserFlag,
		After:            serviceAfterFlag,
		RestartSec:       restartSec,
	}

	// 执行模板并输出
	if serviceStdoutFlag {
		// 输出到标准输出
		if err := tmpl.Execute(os.Stdout, tmplData); err != nil {
			fmt.Printf("错误: 无法执行模板: %v\n", err)
			os.Exit(1)
		}
	} else {
		// 写入文件
		outputFile, err := os.Create(outputPath)
		if err != nil {
			fmt.Printf("错误: 无法创建输出文件: %v\n", err)
			os.Exit(1)
		}
		defer outputFile.Close()

		if err := tmpl.Execute(outputFile, tmplData); err != nil {
			fmt.Printf("错误: 无法执行模板: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✓ 服务脚本生成成功: %s\n\n", outputPath)
		fmt.Println("请执行以下操作安装服务：")
		fmt.Printf("  cp %s /usr/lib/systemd/system/\n", outputPath)
		fmt.Printf("  sudo systemctl daemon-reload\n")
		fmt.Printf("  sudo systemctl start %s\n", serviceName)
		fmt.Printf("  sudo systemctl enable %s\n", serviceName)
		fmt.Println("\n查看服务状态：")
		fmt.Printf("  sudo systemctl status %s\n", serviceName)
		fmt.Println("\n重新启动服务：")
		fmt.Printf("  sudo systemctl restart %s\n", serviceName)
		fmt.Println("\n停止服务：")
		fmt.Printf("  sudo systemctl stop %s\n", serviceName)
		fmt.Println("\n禁用服务：")
		fmt.Printf("  sudo systemctl disable %s\n", serviceName)
	}
}
