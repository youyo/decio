package action

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/youyo/decio/internal/config"
	"github.com/youyo/decio/internal/decision"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name string
		cfg  any
		res  decision.Result
		want string
		ok   bool
	}{
		{
			name: "choice",
			cfg:  config.Actions{"security": {Command: "security"}},
			res:  decision.Result{Type: decision.ChoiceType, Value: "security"},
			want: "security",
			ok:   true,
		},
		{
			name: "boolean",
			cfg:  config.Actions{"true": {Command: "tests"}},
			res:  decision.Result{Type: decision.Boolean, Value: true},
			want: "tests",
			ok:   true,
		},
		{
			name: "score",
			cfg:  config.ScoreActions{{Min: 50, Max: 79, Command: "review"}},
			res:  decision.Result{Type: decision.Score, Value: 79},
			want: "review",
			ok:   true,
		},
		{
			name: "score outside",
			cfg:  config.ScoreActions{{Min: 50, Max: 79, Command: "review"}},
			res:  decision.Result{Type: decision.Score, Value: 80},
		},
		{
			name: "no match",
			cfg:  config.Actions{"false": {Command: "no"}},
			res:  decision.Result{Type: decision.Boolean, Value: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Resolve(tt.cfg, tt.res)
			if ok != tt.ok {
				t.Fatalf("Resolve() ok = %v, want %v", ok, tt.ok)
			}
			if tt.ok && got.Command != tt.want {
				t.Fatalf("Resolve() command = %q, want %q", got.Command, tt.want)
			}
		})
	}
}

func TestRunInjectsEnvironmentAndExpandsTemplates(t *testing.T) {
	res := decision.Result{Type: decision.ChoiceType, Value: "security", Provider: "jev", Model: "default"}
	spec := Spec{
		Command: "test \"$DECIO_TYPE\" = choice && test \"$DECIO_VALUE\" = security && test \"$DECIO_PROVIDER\" = jev && test \"$DECIO_MODEL\" = default && test \"$CUSTOM\" = 'security/choice'",
		Env:     map[string]string{"CUSTOM": "{{ result.value }}/{{ result.type }}"},
	}
	if code, err := Run(context.Background(), spec, res, nil); err != nil || code != 0 {
		t.Fatalf("Run() = %d, %v", code, err)
	}
}

func TestRunForwardsOriginalStdin(t *testing.T) {
	spec := Spec{Command: "test \"$(cat)\" = 'original bytes'", Stdin: "original"}
	if code, err := Run(context.Background(), spec, decision.Result{Type: decision.Boolean, Value: true}, []byte("original bytes")); err != nil || code != 0 {
		t.Fatalf("Run() = %d, %v", code, err)
	}
}

func TestRunPropagatesExitCode(t *testing.T) {
	code, err := Run(context.Background(), Spec{Command: "exit 7"}, decision.Result{}, nil)
	if err != nil || code != 7 {
		t.Fatalf("Run() = %d, %v, want 7 and nil error", code, err)
	}
}

func TestRunRejectsInvalidSpec(t *testing.T) {
	_, err := Run(context.Background(), Spec{Stdin: "pipe"}, decision.Result{}, nil)
	if !errors.Is(err, ErrDispatch) {
		t.Fatalf("Run() error = %v, want ErrDispatch", err)
	}
}

func TestRunRejectsSignalTermination(t *testing.T) {
	code, err := Run(context.Background(), Spec{Command: "kill -TERM $$"}, decision.Result{}, nil)
	if code < 0 {
		t.Fatalf("Run() code = %d, must not be negative", code)
	}
	if !errors.Is(err, ErrDispatch) {
		t.Fatalf("Run() error = %v, want ErrDispatch", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "signal") {
		t.Fatalf("Run() error = %v, want signal details", err)
	}
}
