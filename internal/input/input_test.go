package input

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCollectSources(t *testing.T) {
	file := filepath.Join(t.TempDir(), "context.txt")
	if err := os.WriteFile(file, []byte("from file"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Collect(context.Background(), []Source{
		{Name: "literal", Literal: "from literal"},
		{Name: "file", File: file},
		{Name: "command", Command: "printf 'from command'"},
	}, nil)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	for name, want := range map[string]string{"literal": "from literal", "file": "from file", "command": "from command"} {
		if got.Named[name] != want {
			t.Errorf("Named[%q] = %q, want %q", name, got.Named[name], want)
		}
	}
	if got.Order[0] != "literal" || got.Order[2] != "command" {
		t.Fatalf("Order = %v", got.Order)
	}
}

func TestCollectCommandErrors(t *testing.T) {
	got, err := Collect(context.Background(), []Source{{Name: "command", Command: "printf 'details' >&2; exit 9"}}, nil)
	if err == nil || !errors.Is(err, ErrCollect) || !strings.Contains(err.Error(), "details") {
		t.Fatalf("Collect() error = %v, want ErrCollect containing stderr", err)
	}
	if got.Named != nil {
		t.Fatalf("Collect() result = %#v on error", got)
	}
}

func TestCollectCommandTimeout(t *testing.T) {
	if _, err := Collect(context.Background(), []Source{{Name: "slow", Command: "sleep 1", Timeout: 20 * time.Millisecond}}, nil); err == nil || !errors.Is(err, ErrCollect) {
		t.Fatalf("Collect() error = %v, want timeout ErrCollect", err)
	}
}

func TestCollectStdinPreservesBytes(t *testing.T) {
	want := []byte{0, 1, 2, '\n', 255}
	got, err := Collect(context.Background(), []Source{{Name: "event", Stdin: true}}, want)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if string(got.Named["event"]) != string(want) {
		t.Fatalf("stdin = %v, want %v", []byte(got.Named["event"]), want)
	}
}

func TestCollectWithoutSourcesUsesText(t *testing.T) {
	got, err := Collect(context.Background(), nil, []byte("plain input"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "plain input" || len(got.Named) != 0 {
		t.Fatalf("input = %#v", got)
	}
}

func TestCommandIsRunThroughShell(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh is unavailable")
	}
	got, err := Collect(context.Background(), []Source{{Name: "command", Command: "printf 'shell\\n'"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Named["command"] != "shell\n" {
		t.Fatalf("command output = %q", got.Named["command"])
	}
}
