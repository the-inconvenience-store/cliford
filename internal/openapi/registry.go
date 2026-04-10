package openapi

import (
	"strings"

	"github.com/cliford/cliford/pkg/registry"
)

// RegistryBuilder enriches parsed operations with CLI command names,
// applies stutter removal, and builds the final tag-grouped registry.
type RegistryBuilder struct {
	removeStutter bool
}

// NewRegistryBuilder creates a builder. If removeStutter is true, redundant
// prefixes/suffixes are stripped from command names (e.g., "pets list-pets"
// becomes "pets list").
func NewRegistryBuilder(removeStutter bool) *RegistryBuilder {
	return &RegistryBuilder{removeStutter: removeStutter}
}

// Build processes the registry's operations in-place: assigns CLI command
// names, applies stutter removal, and rebuilds tag groups.
func (b *RegistryBuilder) Build(reg *registry.Registry) error {
	for i := range reg.Operations {
		op := &reg.Operations[i]
		op.CLICommandName = b.deriveCommandName(op)
	}

	if err := b.checkCollisions(reg); err != nil {
		return err
	}

	// Rebuild tag groups with enriched operations
	reg.TagGroups = make(map[string][]registry.OperationMeta)
	for _, op := range reg.Operations {
		tags := op.Tags
		if len(tags) == 0 {
			tags = []string{"default"}
		}
		for _, tag := range tags {
			reg.TagGroups[tag] = append(reg.TagGroups[tag], op)
		}
	}

	return nil
}

// deriveCommandName produces the CLI subcommand name from the operation ID,
// optionally removing stutter with the tag name.
func (b *RegistryBuilder) deriveCommandName(op *registry.OperationMeta) string {
	name := toKebabCase(op.OperationID)

	if !b.removeStutter || len(op.Tags) == 0 {
		return name
	}

	tag := strings.ToLower(op.Tags[0])
	tagSingular := strings.TrimSuffix(tag, "s")

	// Remove tag as prefix: "list-pets" under "pets" -> "list"
	for _, prefix := range []string{tag + "-", tagSingular + "-"} {
		if trimmed := strings.TrimPrefix(name, prefix); trimmed != name && trimmed != "" {
			return trimmed
		}
	}

	// Remove tag as suffix: "pets-list" under "pets" -> "list" (less common)
	for _, suffix := range []string{"-" + tag, "-" + tagSingular} {
		if trimmed := strings.TrimSuffix(name, suffix); trimmed != name && trimmed != "" {
			return trimmed
		}
	}

	return name
}

// checkCollisions detects command name collisions within the same tag group.
func (b *RegistryBuilder) checkCollisions(reg *registry.Registry) error {
	type key struct {
		tag  string
		name string
	}
	seen := make(map[key]string) // key -> operationID

	for _, op := range reg.Operations {
		tags := op.Tags
		if len(tags) == 0 {
			tags = []string{"default"}
		}
		for _, tag := range tags {
			k := key{tag: tag, name: op.CLICommandName}
			if existing, ok := seen[k]; ok {
				return &CollisionError{
					Tag:          tag,
					CommandName:  op.CLICommandName,
					OperationID1: existing,
					OperationID2: op.OperationID,
				}
			}
			seen[k] = op.OperationID
		}
	}

	return nil
}

// CollisionError is returned when two operations produce the same CLI
// command name within the same tag group.
type CollisionError struct {
	Tag          string
	CommandName  string
	OperationID1 string
	OperationID2 string
}

func (e *CollisionError) Error() string {
	return "command name collision: operations " + e.OperationID1 +
		" and " + e.OperationID2 + " both resolve to command '" +
		e.CommandName + "' under tag '" + e.Tag + "'"
}
