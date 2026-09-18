package action

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/youyo/decio/internal/config"
	"github.com/youyo/decio/internal/decision"
)

var ErrDispatch = errors.New("action dispatch error")

type Spec struct {
	Command string
	Stdin   string
	Env     map[string]string
}

type Action = config.Action
type Actions = config.Actions
type ScoreAction = config.ScoreAction
type ScoreActions = config.ScoreActions

// Resolve maps a validated configuration action to a result.
func Resolve(actions any, result decision.Result) (*Spec, bool) {
	switch configured := actions.(type) {
	case config.Actions:
		return resolveMap(configured, result)
	case config.ScoreActions:
		return resolveScores(configured, result)
	default:
		return nil, false
	}
}

func resolveMap(configured map[string]config.Action, result decision.Result) (*Spec, bool) {
	var key string
	switch result.Type {
	case decision.ChoiceType:
		key, _ = result.Value.(string)
	case decision.BooleanType:
		value, ok := result.Value.(bool)
		if !ok {
			return nil, false
		}
		key = strconv.FormatBool(value)
	default:
		return nil, false
	}
	configuredAction, ok := configured[key]
	if !ok {
		return nil, false
	}
	return &Spec{Command: configuredAction.Command, Stdin: configuredAction.Stdin, Env: cloneEnv(configuredAction.Env)}, true
}

func resolveScores(configured config.ScoreActions, result decision.Result) (*Spec, bool) {
	value, ok := result.Value.(int)
	if result.Type != decision.ScoreType || !ok {
		return nil, false
	}
	for _, configuredAction := range configured {
		if value < configuredAction.Min || (scoreActionHasMax(configuredAction) && value > configuredAction.Max) {
			continue
		}
		return &Spec{Command: configuredAction.Command, Stdin: configuredAction.Stdin, Env: cloneEnv(configuredAction.Env)}, true
	}
	return nil, false
}

func scoreActionHasMax(action config.ScoreAction) bool {
	return action.HasMax || action.Max != 0
}

func cloneEnv(env map[string]string) map[string]string {
	if len(env) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(env))
	for key, value := range env {
		cloned[key] = value
	}
	return cloned
}

// Run executes an already-resolved action and returns its process exit code.
func Run(ctx context.Context, spec Spec, result decision.Result, originalStdin []byte) (int, error) {
	if strings.TrimSpace(spec.Command) == "" {
		return -1, fmt.Errorf("%w: command is empty", ErrDispatch)
	}
	if spec.Stdin != "" && spec.Stdin != "none" && spec.Stdin != "original" {
		return -1, fmt.Errorf("%w: invalid stdin mode %q", ErrDispatch, spec.Stdin)
	}

	command := exec.CommandContext(ctx, "sh", "-c", spec.Command)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if spec.Stdin == "original" {
		command.Stdin = bytes.NewReader(originalStdin)
	} else {
		command.Stdin = strings.NewReader("")
	}

	envMap := make(map[string]string)
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			envMap[key] = value
		}
	}
	metadata := map[string]string{
		"DECIO_TYPE":       string(result.Type),
		"DECIO_VALUE":      result.String(),
		"DECIO_PROVIDER":   result.Provider,
		"DECIO_MODEL":      result.Model,
		"DECIO_CONFIDENCE": "",
	}
	if result.Confidence != nil {
		metadata["DECIO_CONFIDENCE"] = strconv.FormatFloat(*result.Confidence, 'f', -1, 64)
	}
	for key, value := range metadata {
		envMap[key] = value
	}
	replacer := strings.NewReplacer(
		"{{ result.value }}", result.String(),
		"{{ result.type }}", string(result.Type),
		"{{ result.confidence }}", metadata["DECIO_CONFIDENCE"],
		"{{ result.provider }}", result.Provider,
		"{{ result.model }}", result.Model,
	)
	for key, value := range spec.Env {
		envMap[key] = replacer.Replace(value)
	}
	env := make([]string, 0, len(envMap))
	for key, value := range envMap {
		env = append(env, key+"="+value)
	}
	command.Env = env

	err := command.Run()
	if err == nil {
		return 0, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		if exitCode := exitError.ExitCode(); exitCode < 0 {
			state := "unknown signal"
			if exitError.ProcessState != nil {
				state = exitError.ProcessState.String()
			}
			return 0, fmt.Errorf("%w: command terminated by signal (%s)", ErrDispatch, state)
		}
		return exitError.ExitCode(), nil
	}
	return -1, fmt.Errorf("%w: run command: %v", ErrDispatch, err)
}
