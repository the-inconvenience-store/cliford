package config

import (
	"github.com/the-inconvenience-store/cliford/pkg/registry"
)

// MergeIntoRegistry applies ClifordConfig operation overrides into the
// Operation Registry. Resolution order (highest to lowest):
//  1. cliford.yaml operation-level config
//  2. x-cliford-* OpenAPI extensions (already applied during parsing)
//  3. cliford.yaml global defaults
//  4. Built-in defaults (already set during parsing)
func MergeIntoRegistry(cfg *ClifordConfig, reg *registry.Registry) {
	for i := range reg.Operations {
		op := &reg.Operations[i]

		// Apply per-operation overrides from cliford.yaml (highest priority)
		override, ok := cfg.Operations[op.OperationID]
		if !ok {
			continue
		}

		// CLI overrides
		if len(override.CLI.Aliases) > 0 {
			op.CLIAliases = override.CLI.Aliases
		}
		if override.CLI.Group != "" {
			op.CLIGroup = override.CLI.Group
		}
		if override.CLI.Hidden {
			op.CLIHidden = true
		}
		if override.CLI.Confirm {
			op.CLIConfirm = true
		}
		if override.CLI.ConfirmMsg != "" {
			op.CLIConfirmMsg = override.CLI.ConfirmMsg
		}
		if override.CLI.AgentFormat != "" {
			op.CLIAgentFormat = override.CLI.AgentFormat
		}
		if override.CLI.DefaultOutputFormat != "" {
			op.CLIDefaultOutputFormat = override.CLI.DefaultOutputFormat
		}
		if override.CLI.RequestID {
			op.CLIRequestID = true
		}

		// TUI overrides
		if override.TUI.DisplayAs != "" {
			op.TUIDisplay = registry.DisplayMode(override.TUI.DisplayAs)
		}
		if override.TUI.Refreshable {
			op.TUIRefreshable = true
		}
	}

	// Rebuild tag groups after merges (group overrides may have changed tags)
	reg.TagGroups = make(map[string][]registry.OperationMeta)
	for _, op := range reg.Operations {
		tags := op.Tags
		if op.CLIGroup != "" {
			tags = []string{op.CLIGroup}
		}
		if len(tags) == 0 {
			tags = []string{"default"}
		}
		for _, tag := range tags {
			reg.TagGroups[tag] = append(reg.TagGroups[tag], op)
		}
	}
}

// ApplyToPipeline converts ClifordConfig fields into pipeline.Config fields.
// Returns the values that the pipeline needs.
type PipelineOverrides struct {
	SpecPath          string
	AppName           string
	PackageName       string
	EnvVarPrefix      string
	AppVersion        string
	RemoveStutter     bool
	GenerateTUI       bool
	GenerateRelease   bool
	CustomCodeRegions bool
}

// ToPipelineOverrides extracts pipeline-relevant settings from the config.
func ToPipelineOverrides(cfg *ClifordConfig) PipelineOverrides {
	return PipelineOverrides{
		SpecPath:          cfg.Spec,
		AppName:           cfg.App.Name,
		PackageName:       cfg.App.Package,
		EnvVarPrefix:      cfg.App.EnvVarPrefix,
		AppVersion:        cfg.App.Version,
		RemoveStutter:     cfg.Generation.CLI.RemoveStutter,
		GenerateTUI:       cfg.Generation.TUI.Enabled,
		GenerateRelease:   cfg.Features.Distribution.GoReleaser,
		CustomCodeRegions: cfg.Features.CustomCodeRegions,
	}
}
