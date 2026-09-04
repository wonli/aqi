package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "aqi",
	Short:         "AQI Project Generator",
	SilenceErrors: true,
	SilenceUsage:  true,
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}
