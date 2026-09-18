package cmd

import (
	"embed"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type docTopic struct {
	Name        string
	Description string
}

//go:embed docs/*.md
var docsContent embed.FS

var docTopics = []docTopic{
	{Name: "overview", Description: "Decio's model, boundaries, and when to use it"},
	{Name: "cli", Description: "Root flags, subcommands, and configuration discovery"},
	{Name: "config", Description: "The YAML schema, constraints, and complete examples"},
	{Name: "actions", Description: "Action resolution, environment, stdin, and exit codes"},
	{Name: "output", Description: "Plain and JSON output shapes and exit-code semantics"},
	{Name: "claude-code", Description: "Claude Code Hook integration recipes"},
	{Name: "pipelines", Description: "Git, CI/CD, shell, and chained-decision recipes"},
	{Name: "troubleshooting", Description: "Common failures, diagnostics, and validation"},
}

func newDocsCmd() *cobra.Command {
	var list bool

	docsCmd := &cobra.Command{
		Use:   "docs [topic]",
		Short: "read embedded agent documentation",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("accepts at most one topic")
			}
			if list && len(args) == 1 {
				return fmt.Errorf("--list cannot be combined with a topic")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if list {
				return writeDocList(cmd)
			}
			if len(args) == 0 {
				return writeAllDocs(cmd)
			}
			content, ok := docContent(args[0])
			if !ok {
				return fail(2, fmt.Errorf("unknown topic %q (available: %s)", args[0], docTopicNames()))
			}
			return writeDoc(cmd, content)
		},
	}
	docsCmd.Flags().BoolVar(&list, "list", false, "list available topics")
	return docsCmd
}

func writeDocList(cmd *cobra.Command) error {
	for _, topic := range docTopics {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", topic.Name, topic.Description); err != nil {
			return fail(2, fmt.Errorf("write docs list: %w", err))
		}
	}
	return nil
}

func writeAllDocs(cmd *cobra.Command) error {
	for _, topic := range docTopics {
		content, ok := docContent(topic.Name)
		if !ok {
			return fail(2, fmt.Errorf("embedded topic %q is missing", topic.Name))
		}
		if err := writeDoc(cmd, content); err != nil {
			return err
		}
	}
	return nil
}

func writeDoc(cmd *cobra.Command, content []byte) error {
	if _, err := cmd.OutOrStdout().Write(content); err != nil {
		return fail(2, fmt.Errorf("write docs: %w", err))
	}
	return nil
}

func docContent(name string) ([]byte, bool) {
	content, err := docsContent.ReadFile("docs/" + name + ".md")
	return content, err == nil
}

func docTopicNames() string {
	names := make([]string, 0, len(docTopics))
	for _, topic := range docTopics {
		names = append(names, topic.Name)
	}
	return strings.Join(names, ", ")
}
