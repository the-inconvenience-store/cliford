package openapi

import (
	"encoding/json"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/the-inconvenience-store/cliford/pkg/registry"
)

// WatchCLIExtension holds x-cliford-cli.watch configuration.
type WatchCLIExtension struct {
	Enabled  *bool  `json:"enabled"`
	Interval string `json:"interval"`
	MaxCount int    `json:"maxCount"`
}

// CLIExtension holds x-cliford-cli data from an operation.
type CLIExtension struct {
	Aliases             []string           `json:"aliases"`
	Group               string             `json:"group"`
	Hidden              bool               `json:"hidden"`
	Confirm             bool               `json:"confirm"`
	ConfirmMessage      string             `json:"confirmMessage"`
	DefaultJQ           string             `json:"defaultJQ"`
	AgentFormat         string             `json:"agentFormat"`
	DefaultOutputFormat string             `json:"defaultOutputFormat"`
	RequestID           bool               `json:"requestId"`
	Watch               *WatchCLIExtension `json:"watch"`
}

// TUIExtension holds x-cliford-tui data from an operation.
type TUIExtension struct {
	DisplayAs   string `json:"displayAs"`
	Refreshable bool   `json:"refreshable"`
}

// PaginationExtension holds x-cliford-pagination data.
type PaginationExtension struct {
	Type   string                        `json:"type"`
	Input  map[string]PaginationInputExt `json:"input"`
	Output PaginationOutputExt           `json:"output"`
}

// PaginationInputExt describes a pagination input parameter.
type PaginationInputExt struct {
	Name    string `json:"name"`
	In      string `json:"in"`
	Default int    `json:"default"`
}

// PaginationOutputExt describes pagination output JSONPaths.
type PaginationOutputExt struct {
	Results    string `json:"results"`
	NextCursor string `json:"nextCursor"`
	TotalCount string `json:"totalCount"`
}

// RetryExtension holds x-cliford-retries data.
type RetryExtension struct {
	Enabled     bool  `json:"enabled"`
	MaxAttempts int   `json:"maxAttempts"`
	StatusCodes []int `json:"statusCodes"`
}

// ExtractExtensions reads x-cliford-* extensions from an OpenAPI operation
// and applies them to the OperationMeta.
func ExtractExtensions(op *openapi3.Operation, meta *registry.OperationMeta) {
	if op.Extensions == nil {
		return
	}

	if raw, ok := op.Extensions["x-cliford-cli"]; ok {
		var ext CLIExtension
		if data, err := marshalRaw(raw); err == nil {
			if err := json.Unmarshal(data, &ext); err == nil {
				if len(ext.Aliases) > 0 {
					meta.CLIAliases = ext.Aliases
				}
				if ext.Group != "" {
					meta.CLIGroup = ext.Group
				}
				meta.CLIHidden = ext.Hidden
				meta.CLIConfirm = ext.Confirm
				if ext.ConfirmMessage != "" {
					meta.CLIConfirmMsg = ext.ConfirmMessage
				}
				if ext.DefaultJQ != "" {
					meta.CLIDefaultJQ = ext.DefaultJQ
				}
				if ext.AgentFormat != "" {
					meta.CLIAgentFormat = ext.AgentFormat
				}
				if ext.DefaultOutputFormat != "" {
					meta.CLIDefaultOutputFormat = ext.DefaultOutputFormat
				}
				if ext.RequestID {
					meta.CLIRequestID = true
				}
				if ext.Watch != nil {
					if ext.Watch.Enabled != nil {
						meta.CLIWatchEnabled = ext.Watch.Enabled
					}
					if ext.Watch.Interval != "" {
						meta.CLIWatchInterval = ext.Watch.Interval
					}
					if ext.Watch.MaxCount > 0 {
						meta.CLIWatchMaxCount = ext.Watch.MaxCount
					}
				}
			}
		}
	}

	if raw, ok := op.Extensions["x-cliford-tui"]; ok {
		var ext TUIExtension
		if data, err := marshalRaw(raw); err == nil {
			if err := json.Unmarshal(data, &ext); err == nil {
				if ext.DisplayAs != "" {
					meta.TUIDisplay = registry.DisplayMode(ext.DisplayAs)
				}
				meta.TUIRefreshable = ext.Refreshable
			}
		}
	}

	if raw, ok := op.Extensions["x-cliford-pagination"]; ok {
		var ext PaginationExtension
		if data, err := marshalRaw(raw); err == nil {
			if err := json.Unmarshal(data, &ext); err == nil {
				meta.Pagination = convertPaginationExt(ext)
			}
		}
	}

	if raw, ok := op.Extensions["x-cliford-retries"]; ok {
		var ext RetryExtension
		if data, err := marshalRaw(raw); err == nil {
			if err := json.Unmarshal(data, &ext); err == nil {
				meta.Retries = convertRetryExt(ext)
			}
		}
	}
}

func marshalRaw(v any) ([]byte, error) {
	// kin-openapi stores extensions as json.RawMessage or already-unmarshaled values
	switch val := v.(type) {
	case json.RawMessage:
		return val, nil
	default:
		return json.Marshal(val)
	}
}

func convertPaginationExt(ext PaginationExtension) *registry.PaginationConfig {
	cfg := &registry.PaginationConfig{
		Type: registry.PaginationType(ext.Type),
	}

	if cursor, ok := ext.Input["cursor"]; ok {
		cfg.InputCursor = registry.PaginationInput{
			Name: cursor.Name,
			In:   registry.ParamLocation(cursor.In),
		}
	}
	if offset, ok := ext.Input["offset"]; ok {
		cfg.InputCursor = registry.PaginationInput{
			Name: offset.Name,
			In:   registry.ParamLocation(offset.In),
		}
	}
	if limit, ok := ext.Input["limit"]; ok {
		cfg.InputLimit = registry.PaginationInput{
			Name:    limit.Name,
			In:      registry.ParamLocation(limit.In),
			Default: limit.Default,
		}
		cfg.DefaultLimit = limit.Default
	}

	cfg.OutputResults = ext.Output.Results
	cfg.OutputNextKey = ext.Output.NextCursor
	cfg.OutputTotal = ext.Output.TotalCount

	return cfg
}

func convertRetryExt(ext RetryExtension) *registry.RetryConfig {
	return &registry.RetryConfig{
		Enabled:     ext.Enabled,
		StatusCodes: ext.StatusCodes,
	}
}
