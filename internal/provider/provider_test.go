package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/youyo/decio/internal/decision"
)

type testProvider struct{}

func (testProvider) Decide(context.Context, decision.Request) (decision.Result, error) {
	return decision.Result{Type: decision.Boolean, Value: true}, nil
}

func TestRegistry(t *testing.T) {
	Register("test", func(model string) (Provider, error) {
		if model != "model-a" {
			t.Fatalf("factory model = %q, want model-a", model)
		}
		return testProvider{}, nil
	})

	got, err := New("test", "model-a")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if got == nil {
		t.Fatal("New() returned nil provider")
	}

	_, err = New("missing", "model-a")
	if !errors.Is(err, ErrUnknown) {
		t.Fatalf("New() error = %v, want ErrUnknown", err)
	}
}
