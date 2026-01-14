package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var verCmd = &cobra.Command{
	Use:   "version",
	Short: "Version of this CLI",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Version: 2.0")
	},
}

func init() {
	rootCmd.AddCommand(verCmd)
}
