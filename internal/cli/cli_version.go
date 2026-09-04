package cli

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

var verCmd = &cobra.Command{
	Use:   "version",
	Short: "Version of this CLI",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Version:", version())
	},
}

func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}

	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return strings.TrimPrefix(info.Main.Version, "v")
	}

	var revision string
	var modified bool
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}

	if len(revision) > 7 {
		revision = revision[:7]
	}
	if revision == "" {
		return "devel"
	}
	if modified {
		return revision + "-dirty"
	}
	return revision
}

func init() {
	rootCmd.AddCommand(verCmd)
}
