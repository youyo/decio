package input

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/youyo/decio/internal/decision"
)

var ErrCollect = errors.New("input collection error")

type Source struct {
	Name    string
	Stdin   bool
	Command string
	File    string
	Literal string
	Timeout time.Duration
}

// Collect resolves sources in order and returns normalized input.
func Collect(ctx context.Context, sources []Source, stdin []byte) (decision.Input, error) {
	if len(sources) == 0 {
		return decision.Input{Text: string(stdin)}, nil
	}

	result := decision.Input{
		Named: make(map[string]string, len(sources)),
		Order: make([]string, 0, len(sources)),
	}
	hasText := false
	for _, source := range sources {
		value, err := collectSource(ctx, source, stdin)
		if err != nil {
			return decision.Input{}, err
		}
		if source.Name == "" {
			if len(result.Named) != 0 || hasText {
				return decision.Input{}, fmt.Errorf("%w: unnamed source cannot be mixed with named sources", ErrCollect)
			}
			result.Text = value
			hasText = true
			continue
		}
		if hasText {
			return decision.Input{}, fmt.Errorf("%w: unnamed source cannot be mixed with named sources", ErrCollect)
		}
		result.Named[source.Name] = value
		result.Order = append(result.Order, source.Name)
	}
	return result, nil
}

func collectSource(parent context.Context, source Source, stdin []byte) (string, error) {
	switch {
	case source.Stdin:
		return string(stdin), nil
	case source.Command != "":
		return collectCommand(parent, source)
	case source.File != "":
		data, err := os.ReadFile(source.File)
		if err != nil {
			return "", fmt.Errorf("%w: read %s: %v", ErrCollect, source.File, err)
		}
		return string(data), nil
	default:
		return source.Literal, nil
	}
}

func collectCommand(parent context.Context, source Source) (string, error) {
	ctx := parent
	var cancel context.CancelFunc
	if source.Timeout > 0 {
		ctx, cancel = context.WithTimeout(parent, source.Timeout)
		defer cancel()
	}

	command := exec.CommandContext(ctx, "sh", "-c", source.Command)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(parent.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("%w: command %q timed out", ErrCollect, source.Name)
		}
		diagnostic := strings.TrimSpace(stderr.String())
		if diagnostic == "" {
			return "", fmt.Errorf("%w: command %q failed: %v", ErrCollect, source.Name, err)
		}
		return "", fmt.Errorf("%w: command %q failed: %v: %s", ErrCollect, source.Name, err, diagnostic)
	}
	return stdout.String(), nil
}
