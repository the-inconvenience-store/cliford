// Package hooks manages the lifecycle and transform hook system for the
// Cliford generation pipeline.
package hooks

import (
	"fmt"
	"strings"
)

// HookPoint identifies where in the pipeline a hook runs.
type HookPoint string

const (
	BeforeGenerate HookPoint = "before:generate"
	AfterValidate  HookPoint = "after:validate"
	BeforeSDK      HookPoint = "before:sdk"
	AfterSDK       HookPoint = "after:sdk"
	BeforeCLI      HookPoint = "before:cli"
	AfterCLI       HookPoint = "after:cli"
	BeforeTUI      HookPoint = "before:tui"
	AfterTUI       HookPoint = "after:tui"
	BeforeDocs     HookPoint = "before:docs"
	AfterDocs      HookPoint = "after:docs"
	AfterGenerate  HookPoint = "after:generate"

	TransformOperation HookPoint = "transform:operation"
	TransformCommand   HookPoint = "transform:command"
	TransformModel     HookPoint = "transform:model"
	TransformStyle     HookPoint = "transform:style"
)

// Hook represents a registered hook command.
type Hook struct {
	Point   HookPoint
	Command string // Shell command to execute
}

// Registry holds all registered hooks organized by hook point.
type Registry struct {
	hooks map[HookPoint][]Hook
}

// NewRegistry creates an empty hook registry.
func NewRegistry() *Registry {
	return &Registry{
		hooks: make(map[HookPoint][]Hook),
	}
}

// Register adds a hook at the given point.
func (r *Registry) Register(point HookPoint, command string) {
	r.hooks[point] = append(r.hooks[point], Hook{
		Point:   point,
		Command: command,
	})
}

// Get returns all hooks registered at the given point.
func (r *Registry) Get(point HookPoint) []Hook {
	return r.hooks[point]
}

// HasHooks returns true if any hooks are registered at the given point.
func (r *Registry) HasHooks(point HookPoint) bool {
	return len(r.hooks[point]) > 0
}

// Count returns total number of registered hooks.
func (r *Registry) Count() int {
	total := 0
	for _, hooks := range r.hooks {
		total += len(hooks)
	}
	return total
}

// LoadFromConfig parses hook definitions from the cliford.yaml hooks section.
// The config format is: hooks: { "after:generate": [{ run: "gofmt -w ." }] }
func LoadFromConfig(hooksConfig map[string][]struct{ Run string }) *Registry {
	reg := NewRegistry()

	for pointStr, defs := range hooksConfig {
		point := HookPoint(pointStr)
		if !isValidPoint(point) {
			fmt.Printf("Warning: unknown hook point %q, skipping\n", pointStr)
			continue
		}
		for _, def := range defs {
			if def.Run != "" {
				reg.Register(point, def.Run)
			}
		}
	}

	return reg
}

func isValidPoint(p HookPoint) bool {
	valid := []HookPoint{
		BeforeGenerate, AfterValidate, BeforeSDK, AfterSDK,
		BeforeCLI, AfterCLI, BeforeTUI, AfterTUI,
		BeforeDocs, AfterDocs, AfterGenerate,
		TransformOperation, TransformCommand, TransformModel, TransformStyle,
	}
	for _, v := range valid {
		if p == v {
			return true
		}
	}
	// Also accept without colon prefix for flexibility
	return strings.Contains(string(p), ":")
}
