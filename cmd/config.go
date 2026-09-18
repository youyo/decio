package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	configpkg "github.com/youyo/decio/internal/config"
)

func newConfigCmd() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "configuration commands",
	}
	configCmd.AddCommand(newConfigValidateCmd())
	return configCmd
}

func newConfigValidateCmd() *cobra.Command {
	var path string
	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "validate a configuration file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if path == "" {
				return fail(2, fmt.Errorf("-c/--config is required"))
			}
			cfg, err := configpkg.Load(path)
			if err == nil {
				err = cfg.Validate()
			}
			if err != nil {
				return fail(2, err)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return err
		},
	}
	validateCmd.Flags().StringVarP(&path, "config", "c", "", "configuration file")
	return validateCmd
}
