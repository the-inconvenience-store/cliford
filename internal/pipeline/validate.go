// Package pipeline orchestrates the Cliford code generation stages.
package pipeline

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/the-inconvenience-store/cliford/internal/openapi"
	"github.com/the-inconvenience-store/cliford/pkg/registry"
)

// ValidationResult holds the outcome of pre-generation validation.
type ValidationResult struct {
	Errors   []string
	Warnings []string
}

// HasErrors returns true if any validation errors were found.
func (r *ValidationResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// Validate performs pre-generation validation: checks the spec is parseable,
// verifies config, and detects command name collisions after stutter removal.
func Validate(ctx context.Context, specPath string, removeStutter bool) (*ValidationResult, *registry.Registry, error) {
	result := &ValidationResult{}

	// Check spec file exists
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		result.Errors = append(result.Errors, fmt.Sprintf("OpenAPI spec not found: %s", specPath))
		return result, nil, nil
	}

	// Parse the spec
	parser := openapi.NewParser(specPath)
	reg, err := parser.Parse(ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Spec parse error: %v", err))
		return result, nil, nil
	}

	// Build registry with stutter removal and collision detection
	builder := openapi.NewRegistryBuilder(removeStutter)
	if err := builder.Build(reg); err != nil {
		var collErr *openapi.CollisionError
		if ok := isCollisionError(err, &collErr); ok {
			result.Errors = append(result.Errors,
				fmt.Sprintf("Command name collision: operations '%s' and '%s' both resolve to '%s' in tag '%s'. "+
					"Rename one operationId or disable stutter removal.",
					collErr.OperationID1, collErr.OperationID2, collErr.CommandName, collErr.Tag))
		} else {
			result.Errors = append(result.Errors, fmt.Sprintf("Registry build error: %v", err))
		}
		return result, nil, nil
	}

	// Validate operation IDs are present
	for _, op := range reg.Operations {
		if op.OperationID == "" {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Operation %s %s has no operationId; auto-generated ID may not be stable across spec changes",
					op.Method, op.Path))
		}
	}

	// Check for empty tags
	for _, op := range reg.Operations {
		if len(op.Tags) == 0 {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Operation '%s' has no tags; it will be grouped under 'default'", op.OperationID))
		}
	}

	// Check servers are defined
	if len(reg.Servers) == 0 {
		result.Warnings = append(result.Warnings,
			"No servers defined in spec; generated app will require --server flag")
	}

	// Check for insecure servers
	for _, s := range reg.Servers {
		if strings.HasPrefix(s.URL, "http://") && !strings.Contains(s.URL, "localhost") {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Server '%s' uses HTTP (not HTTPS); credentials will be sent unencrypted", s.URL))
		}
	}

	return result, reg, nil
}

// isCollisionError checks if an error is a CollisionError using type assertion.
func isCollisionError(err error, target **openapi.CollisionError) bool {
	if ce, ok := err.(*openapi.CollisionError); ok {
		*target = ce
		return true
	}
	return false
}
