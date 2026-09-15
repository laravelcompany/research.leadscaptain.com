package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, input json.RawMessage) (any, error)
}
type Registry struct{ tools map[string]Tool }

func NewRegistry() *Registry        { return &Registry{tools: map[string]Tool{}} }
func (r *Registry) Register(t Tool) { r.tools[t.Name()] = t }
func (r *Registry) Execute(ctx context.Context, name string, input json.RawMessage) (any, error) {
	t, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("unknown tool %s", name)
	}
	return t.Execute(ctx, input)
}
func (r *Registry) Names() []string {
	var n []string
	for k := range r.tools {
		n = append(n, k)
	}
	return n
}

// Describe returns "name: description" for every registered tool, for prompts.
func (r *Registry) Describe() []string {
	var out []string
	for _, t := range r.tools {
		out = append(out, t.Name()+": "+t.Description())
	}
	return out
}
