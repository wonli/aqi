package cli

import "github.com/wonli/aqi/service"

func init() {
	rootCmd.AddCommand(service.Command(service.CommandOptions{}))
}
