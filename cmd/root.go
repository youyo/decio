package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/youyo/decio/internal/action"
	configpkg "github.com/youyo/decio/internal/config"
	"github.com/youyo/decio/internal/decision"
	inputpkg "github.com/youyo/decio/internal/input"
	"github.com/youyo/decio/internal/output"
	"github.com/youyo/decio/internal/provider"
	_ "github.com/youyo/decio/internal/provider/jev"
)

// ExitError carries the process exit code for a command failure.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e == nil {
		return "exit error"
	}
	if e.Err == nil {
		return fmt.Sprintf("exit code %d", e.Code)
	}
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type options struct {
	configPath    string
	prompt        string
	choices       []string
	booleanPrompt string
	scorePrompt   string
	min           int
	max           int
	levels        []string
	provider      string
	model         string
	asJSON        bool
	noAction      bool
	resultExit    bool
	timeout       time.Duration
	verbose       bool
}

// NewRootCmd creates a fresh Decio command tree.
func NewRootCmd() *cobra.Command {
	opts := &options{provider: "jev", timeout: 30 * time.Second}
	root := &cobra.Command{
		Use:           "decio",
		Short:         "make a typed decision from context",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd, opts)
		},
	}
	flags := root.Flags()
	flags.StringVarP(&opts.configPath, "config", "c", "", "configuration file")
	flags.StringVar(&opts.prompt, "prompt", "", "decision prompt")
	flags.StringArrayVar(&opts.choices, "choice", nil, "allowed choice (repeatable)")
	flags.StringVar(&opts.booleanPrompt, "boolean", "", "boolean decision prompt")
	flags.StringVar(&opts.scorePrompt, "score", "", "score decision prompt")
	flags.IntVar(&opts.min, "min", 0, "minimum score")
	flags.IntVar(&opts.max, "max", 0, "maximum score")
	flags.StringArrayVar(&opts.levels, "level", nil, "score level (repeatable)")
	flags.StringVar(&opts.provider, "provider", opts.provider, "provider name")
	flags.StringVar(&opts.model, "model", "", "provider model")
	flags.BoolVar(&opts.asJSON, "json", false, "write JSON output")
	flags.BoolVar(&opts.noAction, "no-action", false, "skip configured action")
	flags.BoolVar(&opts.resultExit, "result-exit-code", false, "return 1 for a false boolean result without an action")
	flags.DurationVar(&opts.timeout, "timeout", opts.timeout, "decision timeout")
	flags.BoolVar(&opts.verbose, "verbose", false, "write diagnostic details to stderr")

	root.AddCommand(newCompletionCmd(), newConfigCmd(), newVersionCmd())
	return root
}

// Execute runs Decio and converts command errors into process exit codes.
func Execute() {
	root := NewRootCmd()
	if err := root.Execute(); err != nil {
		code := 2
		var exitErr *ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.Code
			if code < 0 {
				code = 6
			}
		}
		_, _ = fmt.Fprintf(root.ErrOrStderr(), "decio: %s\n", err)
		os.Exit(code)
	}
}

func run(cmd *cobra.Command, opts *options) error {
	cfg, configPath, err := resolveConfig(cmd, opts)
	if err != nil {
		return fail(2, err)
	}
	if opts.verbose {
		writeDiagnostic(cmd, "config=%s", configPath)
	}

	parent := cmd.Context()
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, opts.timeout)
	defer cancel()
	stdin, hasStdin, err := readStdin(cmd.InOrStdin())
	if err != nil {
		return fail(3, err)
	}

	sources := configuredSources(cfg)
	collected, err := inputpkg.Collect(ctx, sources, stdin)
	if err != nil {
		return fail(3, err)
	}
	if opts.verbose {
		writeSourceDiagnostics(cmd, collected, hasStdin)
	}

	providerName := cfg.Provider.Type
	if providerName == "" {
		providerName = "jev"
	}
	providerModel := cfg.Provider.Model
	if opts.verbose {
		writeDiagnostic(cmd, "provider=%s model=%s", providerName, providerModel)
	}
	selected, err := provider.New(providerName, providerModel)
	if err != nil {
		if errors.Is(err, provider.ErrConfig) || errors.Is(err, provider.ErrUnknown) {
			return fail(2, err)
		}
		return fail(4, err)
	}

	request := decision.Request{
		Type:    cfg.Decision.Type,
		Prompt:  cfg.Decision.Prompt,
		Input:   collected,
		Choices: decisionChoices(cfg.Decision.Choices),
		Range:   cfg.Decision.Range,
		Levels:  cfg.Decision.Levels,
	}
	started := time.Now()
	result, err := selected.Decide(ctx, request)
	if err != nil {
		if errors.Is(err, decision.ErrInvalidResult) {
			return fail(5, err)
		}
		return fail(4, err)
	}
	if err := decision.Validate(request, result); err != nil {
		return fail(5, err)
	}
	if result.Provider == "" {
		result.Provider = providerName
	}
	if result.Model == "" {
		result.Model = providerModel
	}
	if opts.verbose {
		writeDiagnostic(cmd, "decision=%s value=%s elapsed=%s", result.Type, result.String(), time.Since(started).Round(time.Millisecond))
	}

	var outcome *output.ActionOutcome
	var actionCode int
	actionExecuted := false
	if !opts.noAction {
		if spec, ok := action.Resolve(cfg.Actions, result); ok {
			actionStarted := time.Now()
			code, runErr := action.Run(ctx, *spec, result, stdin)
			if runErr != nil {
				return fail(6, runErr)
			}
			actionCode = code
			actionExecuted = true
			outcome = &output.ActionOutcome{Executed: true, ExitCode: code}
			if opts.verbose {
				writeDiagnostic(cmd, "action=executed exit_code=%d elapsed=%s", code, time.Since(actionStarted).Round(time.Millisecond))
			}
		} else if opts.verbose {
			writeDiagnostic(cmd, "action=none")
		}
	} else if opts.verbose {
		writeDiagnostic(cmd, "action=skipped")
	}

	if err := output.Write(cmd.OutOrStdout(), result, outcome, opts.asJSON); err != nil {
		return fail(2, fmt.Errorf("write output: %w", err))
	}
	if actionExecuted && actionCode != 0 {
		return fail(actionCode, fmt.Errorf("action exited with code %d", actionCode))
	}
	if opts.resultExit && result.Type == decision.BooleanType {
		value, ok := result.Value.(bool)
		if ok && !value && !actionExecuted {
			return fail(1, errors.New("decision result is false"))
		}
	}
	return nil
}

func resolveConfig(cmd *cobra.Command, opts *options) (*configpkg.Config, string, error) {
	path := opts.configPath
	if !cmd.Flags().Changed("config") {
		if found, ok := configpkg.Find(); ok {
			path = found
		}
	}

	var cfg *configpkg.Config
	if path != "" {
		loaded, err := configpkg.Load(path)
		if err != nil {
			return nil, path, err
		}
		cfg = loaded
	} else {
		cfg = &configpkg.Config{}
	}
	if err := applyCLIOverrides(cmd, opts, cfg); err != nil {
		return nil, path, err
	}
	if cfg.Provider.Type == "" {
		cfg.Provider.Type = "jev"
	}
	if err := cfg.Validate(); err != nil {
		return nil, path, err
	}
	return cfg, path, nil
}

func applyCLIOverrides(cmd *cobra.Command, opts *options, cfg *configpkg.Config) error {
	choiceSet := cmd.Flags().Changed("choice")
	booleanSet := cmd.Flags().Changed("boolean")
	scoreSet := cmd.Flags().Changed("score")
	typeCount := 0
	if choiceSet {
		typeCount++
	}
	if booleanSet {
		typeCount++
	}
	if scoreSet {
		typeCount++
	}
	if typeCount > 1 {
		return errors.New("--choice, --boolean, and --score are mutually exclusive")
	}

	switch {
	case choiceSet:
		cfg.Decision.Type = decision.ChoiceType
		cfg.Decision.Choices = make(map[string]configpkg.Choice, len(opts.choices))
		for _, choice := range opts.choices {
			cfg.Decision.Choices[choice] = configpkg.Choice{}
		}
	case booleanSet:
		cfg.Decision.Type = decision.BooleanType
		cfg.Decision.Prompt = opts.booleanPrompt
	case scoreSet:
		cfg.Decision.Type = decision.ScoreType
		cfg.Decision.Prompt = opts.scorePrompt
	}

	if cmd.Flags().Changed("prompt") {
		cfg.Decision.Prompt = opts.prompt
	}
	if cmd.Flags().Changed("level") {
		cfg.Decision.Levels = append([]string(nil), opts.levels...)
	}
	if cfg.Decision.Type == decision.ScoreType && (cmd.Flags().Changed("min") || cmd.Flags().Changed("max")) {
		if cfg.Decision.Range == nil {
			cfg.Decision.Range = &decision.ScoreRange{}
		}
		if cmd.Flags().Changed("min") {
			cfg.Decision.Range.Min = opts.min
		}
		if cmd.Flags().Changed("max") {
			cfg.Decision.Range.Max = opts.max
		}
	}
	if cmd.Flags().Changed("provider") {
		cfg.Provider.Type = opts.provider
	}
	if cmd.Flags().Changed("model") {
		cfg.Provider.Model = opts.model
	}
	if cfg.Decision.Type == decision.ChoiceType && choiceSet && len(opts.choices) == 0 {
		return errors.New("at least two --choice values are required")
	}
	return nil
}

func configuredSources(cfg *configpkg.Config) []inputpkg.Source {
	if len(cfg.Input.Sources) == 0 {
		return nil
	}
	ordered := make([]string, 0, len(cfg.Input.Sources))
	seen := make(map[string]bool, len(cfg.Input.Sources))
	for _, name := range cfg.Input.Order {
		if _, ok := cfg.Input.Sources[name]; ok && !seen[name] {
			ordered = append(ordered, name)
			seen[name] = true
		}
	}
	remaining := make([]string, 0, len(cfg.Input.Sources)-len(ordered))
	for name := range cfg.Input.Sources {
		if !seen[name] {
			remaining = append(remaining, name)
		}
	}
	sort.Strings(remaining)
	ordered = append(ordered, remaining...)

	sources := make([]inputpkg.Source, 0, len(ordered))
	for _, name := range ordered {
		source := cfg.Input.Sources[name]
		sources = append(sources, inputpkg.Source{
			Name: name, Stdin: source.Stdin, Command: source.Command, File: source.File,
			Literal: source.Literal, Timeout: source.Timeout,
		})
	}
	return sources
}

func decisionChoices(choices map[string]configpkg.Choice) []decision.Choice {
	ids := make([]string, 0, len(choices))
	for id := range choices {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]decision.Choice, 0, len(ids))
	for _, id := range ids {
		result = append(result, decision.Choice{ID: id, Description: choices[id].Description})
	}
	return result
}

func readStdin(reader io.Reader) ([]byte, bool, error) {
	if file, ok := reader.(*os.File); ok {
		info, err := file.Stat()
		if err != nil {
			return nil, false, fmt.Errorf("stat stdin: %w", err)
		}
		if info.Mode()&os.ModeCharDevice != 0 {
			return nil, false, nil
		}
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, true, fmt.Errorf("read stdin: %w", err)
	}
	return data, true, nil
}

func writeSourceDiagnostics(cmd *cobra.Command, collected decision.Input, hasStdin bool) {
	if len(collected.Named) == 0 {
		if hasStdin {
			writeDiagnostic(cmd, "source=stdin size=%d", len(collected.Text))
		}
		return
	}
	for _, name := range collected.Order {
		writeDiagnostic(cmd, "source=%s size=%d", name, len(collected.Named[name]))
	}
}

func writeDiagnostic(cmd *cobra.Command, format string, args ...any) {
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), format+"\n", args...)
}

func fail(code int, err error) error {
	return &ExitError{Code: code, Err: err}
}
