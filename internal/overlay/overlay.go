// Package overlay applies OAI Overlay Specification v1.0/v1.1 files to an
// OpenAPI spec without modifying the original file on disk.
package overlay

import (
	"bytes"
	"fmt"
	"os"

	"github.com/speakeasy-api/openapi/overlay"
	"gopkg.in/yaml.v3"
)

// Apply loads the spec at specPath, applies each overlay file in overlayPaths
// in order, and returns the merged spec as YAML bytes.
//
// Each overlay file must conform to the OAI Overlay Specification v1.0/v1.1.
// Actions whose JSONPath target matches zero nodes are treated as warnings by
// the underlying library; Apply propagates any errors from the overlay engine
// but not zero-match warnings.
func Apply(specPath string, overlayPaths []string) ([]byte, error) {
	f, err := os.Open(specPath)
	if err != nil {
		return nil, fmt.Errorf("open spec %s: %w", specPath, err)
	}
	defer f.Close()

	var specNode yaml.Node
	if err := yaml.NewDecoder(f).Decode(&specNode); err != nil {
		return nil, fmt.Errorf("parse spec %s: %w", specPath, err)
	}

	for _, ovPath := range overlayPaths {
		ov, err := overlay.Parse(ovPath)
		if err != nil {
			return nil, fmt.Errorf("load overlay %s: %w", ovPath, err)
		}
		if err := ov.ApplyTo(&specNode); err != nil {
			return nil, fmt.Errorf("apply overlay %s: %w", ovPath, err)
		}
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&specNode); err != nil {
		return nil, fmt.Errorf("marshal merged spec: %w", err)
	}
	return buf.Bytes(), nil
}
