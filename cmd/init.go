package cmd

import (
	"embed"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

//go:embed templates/*.yaml
var templates embed.FS

func newInitCmd() *cobra.Command {
	var decisionType string
	var outputPath string
	var force bool

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "write a configuration template",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			template, err := initTemplate(decisionType)
			if err != nil {
				return fail(2, err)
			}
			if outputPath == "" {
				if _, err := cmd.OutOrStdout().Write(template); err != nil {
					return fail(2, fmt.Errorf("write template to stdout: %w", err))
				}
				return nil
			}
			if err := writeInitTemplate(outputPath, template, force); err != nil {
				return fail(2, err)
			}
			_, err = fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s\n", outputPath)
			return err
		},
	}
	initCmd.Flags().StringVar(&decisionType, "type", "choice", "decision type: choice, boolean, or score")
	initCmd.Flags().StringVarP(&outputPath, "output", "o", "", "write the template to a file")
	initCmd.Flags().BoolVar(&force, "force", false, "overwrite an existing output file")
	return initCmd
}

func initTemplate(decisionType string) ([]byte, error) {
	if decisionType != "choice" && decisionType != "boolean" && decisionType != "score" {
		return nil, fmt.Errorf("unsupported init type %q (want choice, boolean, or score)", decisionType)
	}
	return templates.ReadFile("templates/" + decisionType + ".yaml")
}

func writeInitTemplate(path string, template []byte, force bool) error {
	flags := os.O_WRONLY | os.O_CREATE
	if force {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	file, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if _, err := file.Write(template); err != nil {
		_ = file.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}
	return nil
}
