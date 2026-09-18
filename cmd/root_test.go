package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/youyo/decio/internal/decision"
	"github.com/youyo/decio/internal/provider"
)

type mockProvider struct {
	decide func(decision.Request) (decision.Result, error)
}

func (p mockProvider) Decide(_ context.Context, req decision.Request) (decision.Result, error) {
	return p.decide(req)
}

func init() {
	provider.Register("mock", func(model string) (provider.Provider, error) {
		return mockProvider{decide: func(req decision.Request) (decision.Result, error) {
			result := decision.Result{Type: req.Type, Provider: "mock", Model: model}
			switch req.Type {
			case decision.ChoiceType:
				result.Value = "normal"
				if strings.Contains(req.Input.Text, "multi") || req.Input.Named["event"] == "event" {
					result.Value = "security"
				}
			case decision.BooleanType:
				result.Value = !strings.Contains(req.Input.Text, "false") && !strings.Contains(req.Input.Named["source"], "false")
			case decision.ScoreType:
				result.Value = req.Range.Min
			}
			return result, nil
		}}, nil
	})
	provider.Register("mock-error", func(string) (provider.Provider, error) {
		return mockProvider{decide: func(decision.Request) (decision.Result, error) {
			return decision.Result{}, fmt.Errorf("%w: test failure", provider.ErrProvider)
		}}, nil
	})
	provider.Register("mock-invalid", func(string) (provider.Provider, error) {
		return mockProvider{decide: func(req decision.Request) (decision.Result, error) {
			return decision.Result{Type: req.Type, Value: "not-allowed", Provider: "mock-invalid"}, nil
		}}, nil
	})
	provider.Register("mock-context", func(string) (provider.Provider, error) {
		return contextMockProvider{}, nil
	})
}

type contextMockProvider struct{}

func (contextMockProvider) Decide(ctx context.Context, req decision.Request) (decision.Result, error) {
	select {
	case <-ctx.Done():
		return decision.Result{}, ctx.Err()
	case <-time.After(20 * time.Millisecond):
		return decision.Result{Type: req.Type, Value: true, Provider: "mock-context"}, nil
	}
}

func executeTestCommand(t *testing.T, args []string, stdin string) (string, string, error) {
	t.Helper()
	root := NewRootCmd()
	var stdout, stderr bytes.Buffer
	root.SetIn(strings.NewReader(stdin))
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	_, err := root.ExecuteC()
	return stdout.String(), stderr.String(), err
}

func exitCode(t *testing.T, err error) int {
	t.Helper()
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error = %v, want ExitError", err)
	}
	return exitErr.Code
}

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "decio.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRootChoiceFromStdinWritesIDOnly(t *testing.T) {
	stdout, stderr, err := executeTestCommand(t, []string{
		"--provider", "mock", "--choice", "normal", "--choice", "security", "--prompt", "choose",
	}, "input")
	if err != nil {
		t.Fatalf("execute = %v", err)
	}
	if stdout != "normal\n" {
		t.Fatalf("stdout = %q, want %q", stdout, "normal\n")
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

func TestRootCommandSourceBooleanAndResultExitCode(t *testing.T) {
	path := writeTestConfig(t, `version: 1
provider: {type: mock, model: test}
input:
  sources:
    source:
      command: echo false
decision:
  type: boolean
  prompt: should continue?
`)
	stdout, _, err := executeTestCommand(t, []string{"-c", path, "--result-exit-code"}, "ignored")
	if code := exitCode(t, err); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "false\n" {
		t.Fatalf("stdout = %q, want %q", stdout, "false\n")
	}
}

func TestRootMultiSourceDispatchAndJSONOutcome(t *testing.T) {
	resultPath := filepath.Join(t.TempDir(), "result")
	configPath := writeTestConfig(t, fmt.Sprintf(`version: 1
provider: {type: mock, model: test}
input:
  sources:
    event:
      stdin: true
    repository:
      literal: multi
decision:
  type: choice
  prompt: choose
  choices:
    normal: {description: normal}
    security: {description: security}
actions:
  security:
    command: "printf '%%s' \"$DECIO_VALUE\" > %s"
`, resultPath))
	stdout, _, err := executeTestCommand(t, []string{"-c", configPath, "--json"}, "event")
	if err != nil {
		t.Fatalf("execute = %v", err)
	}
	if !strings.Contains(stdout, `"type":"choice"`) || !strings.Contains(stdout, `"value":"security"`) || !strings.Contains(stdout, `"executed":true`) {
		t.Fatalf("stdout = %q, want JSON choice with executed action", stdout)
	}
	data, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "security" {
		t.Fatalf("action output = %q, want security", data)
	}
}

func TestRootActionOutputDoesNotMixWithJSONStdout(t *testing.T) {
	configPath := writeTestConfig(t, `version: 1
provider: {type: mock, model: test}
decision:
  type: boolean
  prompt: approve?
actions:
  "true":
    command: "printf 'action output\\n'"
`)
	stdout, stderr, err := executeTestCommand(t, []string{"-c", configPath, "--json"}, "input")
	if err != nil {
		t.Fatalf("execute = %v", err)
	}
	if !strings.HasSuffix(stdout, "\n") || strings.Count(stdout, "\n") != 1 || !json.Valid([]byte(strings.TrimSpace(stdout))) {
		t.Fatalf("stdout = %q, want exactly one JSON line", stdout)
	}
	if strings.Contains(stdout, "action output") {
		t.Fatalf("stdout = %q, must not contain action output", stdout)
	}
	if stderr != "action output\n" {
		t.Fatalf("stderr = %q, want action output", stderr)
	}
}

func TestRootForwardsClaudeJSONAsOriginalActionStdin(t *testing.T) {
	resultPath := filepath.Join(t.TempDir(), "original.json")
	configPath := writeTestConfig(t, fmt.Sprintf(`version: 1
provider: {type: mock, model: test}
decision:
  type: boolean
  prompt: approve?
actions:
  "true":
    command: "cat > %s"
    stdin: original
`, resultPath))
	original := `{"hook_event_name":"PreToolUse","tool_name":"Bash"}`
	stdout, _, err := executeTestCommand(t, []string{"-c", configPath}, original)
	if err != nil {
		t.Fatalf("execute = %v", err)
	}
	if stdout != "true\n" {
		t.Fatalf("stdout = %q, want true", stdout)
	}
	data, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("forwarded stdin = %q, want %q", data, original)
	}
}

func TestConfigValidateVersionAndCompletion(t *testing.T) {
	validPath := writeTestConfig(t, "version: 1\ndecision:\n  type: boolean\n  prompt: ok\n")
	stdout, _, err := executeTestCommand(t, []string{"config", "validate", "-c", validPath}, "")
	if err != nil || stdout != "ok\n" {
		t.Fatalf("validate = stdout %q, err %v", stdout, err)
	}
	invalidPath := writeTestConfig(t, "version: 1\ndecision:\n  type: choice\n")
	_, _, err = executeTestCommand(t, []string{"config", "validate", "-c", invalidPath}, "")
	if code := exitCode(t, err); code != 2 {
		t.Fatalf("invalid validate exit code = %d, want 2", code)
	}
	stdout, _, err = executeTestCommand(t, []string{"version"}, "")
	if err != nil || stdout != "dev\n" {
		t.Fatalf("version = stdout %q, err %v", stdout, err)
	}
	stdout, _, err = executeTestCommand(t, []string{"completion", "zsh"}, "")
	if err != nil || strings.TrimSpace(stdout) == "" {
		t.Fatalf("completion = stdout %q, err %v", stdout, err)
	}
}

func TestRootMapsProviderAndValidationErrors(t *testing.T) {
	_, _, err := executeTestCommand(t, []string{"--provider", "mock-error", "--boolean", "approve"}, "input")
	if code := exitCode(t, err); code != 4 {
		t.Fatalf("provider error exit code = %d, want 4", code)
	}
	_, _, err = executeTestCommand(t, []string{
		"--provider", "mock-invalid", "--choice", "normal", "--choice", "security", "--prompt", "choose",
	}, "input")
	if code := exitCode(t, err); code != 5 {
		t.Fatalf("invalid result exit code = %d, want 5", code)
	}
}

func TestRootRejectsActionsAfterCLITypeOverride(t *testing.T) {
	path := writeTestConfig(t, `version: 1
provider: {type: mock, model: test}
decision:
  type: score
  range: {min: 0, max: 10}
actions:
  - min: 0
    command: test
`)
	_, _, err := executeTestCommand(t, []string{
		"-c", path, "--choice", "normal", "--choice", "security",
	}, "input")
	if code := exitCode(t, err); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestRootUsesCommandContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	root := NewRootCmd()
	root.SetContext(ctx)
	root.SetIn(strings.NewReader("input"))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"--provider", "mock-context", "--boolean", "approve"})
	_, err := root.ExecuteC()
	if code := exitCode(t, err); code != 4 {
		t.Fatalf("exit code = %d, want 4", code)
	}
}

func TestRootMapsSignalActionFailureToDispatchExitCode(t *testing.T) {
	path := writeTestConfig(t, `version: 1
provider: {type: mock, model: test}
decision:
  type: boolean
  prompt: approve?
actions:
  "true":
    command: "kill -TERM $$"
`)
	_, _, err := executeTestCommand(t, []string{"-c", path}, "input")
	if code := exitCode(t, err); code != 6 {
		t.Fatalf("exit code = %d, want 6", code)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "signal") {
		t.Fatalf("error = %v, want signal details", err)
	}
}
