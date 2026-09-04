package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wonli/aqi/internal/i18ngen"
)

var (
	i18nRootFlag     string
	i18nDataPathFlag string
	i18nLanguageFlag string
)

var i18nCmd = &cobra.Command{
	Use:   "i18n",
	Short: "Generate and check i18n catalogs",
}

var i18nGenCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate the default language catalog from source",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		count, output, changed, err := i18ngen.Generate(i18nRootFlag, i18nDataPathFlag, i18nLanguageFlag)
		if err != nil {
			return err
		}
		status := "unchanged"
		if changed {
			status = "updated"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✓ i18n catalog %s: %s (%d messages)\n", status, output, count)
		return nil
	},
}

var i18nCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check i18n catalogs against source",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		warnings, err := i18ngen.Check(i18nRootFlag, i18nDataPathFlag, i18nLanguageFlag)
		if err != nil {
			return err
		}
		for _, warning := range warnings {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", warning)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "✓ i18n catalogs are in sync")
		return nil
	},
}

func init() {
	for _, cmd := range []*cobra.Command{i18nGenCmd, i18nCheckCmd} {
		cmd.Flags().StringVar(&i18nRootFlag, "root", ".", "source root")
		cmd.Flags().StringVar(&i18nDataPathFlag, "data", "data", "runtime data path")
		cmd.Flags().StringVar(&i18nLanguageFlag, "lang", "zh", "default language")
	}
	i18nCmd.AddCommand(i18nGenCmd, i18nCheckCmd)
	rootCmd.AddCommand(i18nCmd)
}
