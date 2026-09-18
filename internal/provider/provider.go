package provider

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/youyo/decio/internal/decision"
)

// Provider makes a typed decision from normalized input.
type Provider interface {
	Decide(ctx context.Context, req decision.Request) (decision.Result, error)
}

// Factory constructs a provider for a model.
type Factory func(model string) (Provider, error)

var (
	ErrUnknown  = errors.New("unknown provider")
	ErrConfig   = errors.New("provider configuration error")
	ErrProvider = errors.New("provider error")
)

var registry = struct {
	sync.RWMutex
	factories map[string]Factory
}{factories: make(map[string]Factory)}

// Register adds or replaces a provider factory by name.
func Register(name string, factory Factory) {
	registry.Lock()
	defer registry.Unlock()
	registry.factories[name] = factory
}

// New constructs a registered provider.
func New(name, model string) (Provider, error) {
	registry.RLock()
	factory, ok := registry.factories[name]
	registry.RUnlock()
	if !ok || factory == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknown, name)
	}
	return factory(model)
}
