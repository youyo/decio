package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/youyo/decio/internal/decision"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".decio.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadValidConfigurations(t *testing.T) {
	tests := []struct {
		name  string
		yaml  string
		check func(*testing.T, *Config)
	}{
		{
			name: "choice",
			yaml: `version: 1
provider:
  type: jev
  model: default
input:
  sources:
    event:
      stdin: true
    literal:
      literal: repository
decision:
  type: choice
  prompt: Select a review level.
  choices:
    normal:
      description: Normal review.
    security:
      description: Security review.
actions:
  security:
    command: ./security-review.sh
    stdin: original
`,
			check: func(t *testing.T, cfg *Config) {
				if err := cfg.Validate(); err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				if got, want := cfg.Input.Order, []string{"event", "literal"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
					t.Fatalf("source order = %v, want %v", got, want)
				}
				actions, ok := cfg.Actions.(Actions)
				if !ok || actions["security"].Stdin != "original" {
					t.Fatalf("choice actions = %#v", cfg.Actions)
				}
			},
		},
		{
			name: "boolean",
			yaml: `version: 1
provider: {type: jev, model: default}
decision:
  type: boolean
  prompt: Run tests?
actions:
  "true": {command: mise run test}
`,
			check: func(t *testing.T, cfg *Config) {
				if err := cfg.Validate(); err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				if _, ok := cfg.Actions.(Actions); !ok {
					t.Fatalf("actions type = %T, want Actions", cfg.Actions)
				}
			},
		},
		{
			name: "score",
			yaml: `version: 1
provider: {type: jev, model: default}
decision:
  type: score
  prompt: Rate risk.
  range: {min: 0, max: 100}
actions:
  - min: 80
    command: ./security-review.sh
  - min: 0
    max: 79
    command: ./normal-review.sh
`,
			check: func(t *testing.T, cfg *Config) {
				if err := cfg.Validate(); err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				actions, ok := cfg.Actions.(ScoreActions)
				if !ok || len(actions) != 2 || actions[0].HasMax {
					t.Fatalf("score actions = %#v", cfg.Actions)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(writeConfig(t, tt.yaml))
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			tt.check(t, cfg)
		})
	}
}

func TestLoadRejectsUnknownKey(t *testing.T) {
	_, err := Load(writeConfig(t, "version: 1\ndecision:\n  type: boolean\nunknown: true\n"))
	if err == nil {
		t.Fatal("Load() succeeded for unknown key")
	}
}

func TestValidateRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "no source type",
			yaml: "version: 1\ninput:\n  sources:\n    event: {}\ndecision:\n  type: boolean\n",
		},
		{
			name: "multiple source types",
			yaml: "version: 1\ninput:\n  sources:\n    event: {stdin: true, literal: value}\ndecision:\n  type: boolean\n",
		},
		{
			name: "too few choices",
			yaml: "version: 1\ndecision:\n  type: choice\n  choices:\n    only: {description: one}\n",
		},
		{
			name: "invalid score range",
			yaml: "version: 1\ndecision:\n  type: score\n  range: {min: 10, max: 10}\n",
		},
		{
			name: "overlapping score actions",
			yaml: "version: 1\ndecision:\n  type: score\n  range: {min: 0, max: 100}\nactions:\n  - {min: 0, max: 60, command: first}\n  - {min: 50, max: 100, command: second}\n",
		},
		{
			name: "invalid action stdin",
			yaml: "version: 1\ndecision:\n  type: boolean\nactions:\n  'true': {command: test, stdin: pipe}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(writeConfig(t, tt.yaml))
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() succeeded for invalid configuration")
			}
		})
	}
}

func TestFind(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	if path, ok := Find(); ok || path != "" {
		t.Fatalf("Find() = %q, %v before config", path, ok)
	}
	if err := os.WriteFile(filepath.Join(dir, ".decio.yml"), []byte("version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if path, ok := Find(); !ok || filepath.Base(path) != ".decio.yml" {
		t.Fatalf("Find() = %q, %v", path, ok)
	}
}

func TestValidateErrorIsConfigError(t *testing.T) {
	cfg, err := Load(writeConfig(t, "version: 1\ndecision:\n  type: choice\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); !errors.Is(err, ErrConfig) {
		t.Fatalf("Validate() error = %v, want ErrConfig", err)
	}
}

func TestValidateRejectsDecisionSpecificConfiguration(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{
			name: "score requires at least two levels",
			cfg: Config{Decision: Decision{
				Type:   decision.ScoreType,
				Range:  &decision.ScoreRange{Min: 0, Max: 10},
				Levels: []string{"only"},
			}},
		},
		{
			name: "score rejects map actions",
			cfg: Config{
				Decision: Decision{Type: decision.ScoreType, Range: &decision.ScoreRange{Min: 0, Max: 10}},
				Actions:  Actions{"true": {Command: "test"}},
			},
		},
		{
			name: "choice rejects score actions",
			cfg: Config{
				Decision: Decision{Type: decision.ChoiceType, Choices: map[string]Choice{
					"one": {}, "two": {},
				}},
				Actions: ScoreActions{{Min: 0, Command: "test"}},
			},
		},
		{
			name: "boolean rejects score actions",
			cfg: Config{
				Decision: Decision{Type: decision.BooleanType},
				Actions:  ScoreActions{{Min: 0, Command: "test"}},
			},
		},
		{
			name: "boolean action key must be true or false",
			cfg: Config{
				Decision: Decision{Type: decision.BooleanType},
				Actions:  Actions{"yes": {Command: "test"}},
			},
		},
		{
			name: "choice action key must be configured",
			cfg: Config{
				Decision: Decision{Type: decision.ChoiceType, Choices: map[string]Choice{
					"one": {}, "two": {},
				}},
				Actions: Actions{"other": {Command: "test"}},
			},
		},
		{
			name: "source timeout cannot be negative",
			cfg: Config{
				Input: Input{Sources: map[string]Source{
					"literal": {Literal: "value", Timeout: -time.Second},
				}},
				Decision: Decision{Type: decision.BooleanType},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); err == nil || !errors.Is(err, ErrConfig) {
				t.Fatalf("Validate() error = %v, want ErrConfig", err)
			}
		})
	}
}
