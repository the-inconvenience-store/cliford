// Package registry defines the public types for the Cliford Operation Registry.
// The Operation Registry is the central metadata store that maps each API operation
// to its parameters, auth requirements, pagination config, CLI aliases, and TUI display mode.
package registry

import "time"

// OperationMeta is the enriched metadata for a single API operation,
// combining data parsed from the OpenAPI spec with Cliford configuration.
// This is the central type consumed by CLI, TUI, and headless adapters.
type OperationMeta struct {
	// Spec-derived fields
	OperationID string
	Method      string
	Path        string
	Summary     string
	Description string
	Tags        []string
	Parameters  []ParamMeta
	RequestBody *RequestBodyMeta
	Responses   map[int]ResponseMeta
	Security    []SecurityRequirement
	Deprecated  bool

	// CLI-specific (from config/extensions)
	CLICommandName string // Derived name after stutter removal
	CLIAliases     []string
	CLIGroup       string
	CLIHidden      bool
	CLIConfirm     bool
	CLIConfirmMsg  string
	CLIDefaultJQ   string // Default jq expression applied to output (empty = no default)

	// TUI-specific (from config/extensions)
	TUIDisplay     DisplayMode
	TUIRefreshable bool

	// Runtime config
	Pagination *PaginationConfig
	Retries    *RetryConfig
	Timeout    *time.Duration

	// SDK bridge
	SDKMethodName string
}

// ParamMeta describes a single operation parameter.
type ParamMeta struct {
	Name        string
	In          ParamLocation // path, query, header, cookie
	Description string
	Required    bool
	Schema      SchemaMeta
	Default     any
	Enum        []any
	FlagName    string // Derived kebab-case CLI flag name
}

// ParamLocation indicates where a parameter is sent.
type ParamLocation string

const (
	ParamLocationPath   ParamLocation = "path"
	ParamLocationQuery  ParamLocation = "query"
	ParamLocationHeader ParamLocation = "header"
	ParamLocationCookie ParamLocation = "cookie"
)

// SchemaMeta holds type information for a parameter or body field.
type SchemaMeta struct {
	Type        string // string, integer, number, boolean, array, object
	Format      string // int32, int64, float, double, date-time, email, etc.
	Items       *SchemaMeta
	Properties  map[string]SchemaMeta
	Required    []string
	Nullable    bool
	Enum        []any  // Allowed values for this field
	Description string // Human-readable description
	Display     bool   // If true, show this property as a default table column (x-cliford-display)
}

// RequestBodyMeta describes the request body for an operation.
type RequestBodyMeta struct {
	ContentType string
	Schema      SchemaMeta
	Required    bool
	Description string
}

// ResponseMeta describes a single response for an operation.
type ResponseMeta struct {
	StatusCode  int
	Description string
	ContentType string
	Schema      *SchemaMeta
	Headers     map[string]SchemaMeta
}

// SecurityRequirement represents a security scheme requirement for an operation.
type SecurityRequirement struct {
	SchemeName string
	Scopes     []string
}

// SecurityScheme describes an authentication method from the OpenAPI spec.
type SecurityScheme struct {
	Name             string
	Type             SecuritySchemeType // apiKey, http, oauth2, openIdConnect
	In               ParamLocation     // For apiKey: header, query, cookie
	Scheme           string            // For http: basic, bearer
	BearerFormat     string
	Flows            *OAuthFlows
	OpenIDConnectURL string
}

// SecuritySchemeType is the type of a security scheme.
type SecuritySchemeType string

const (
	SecurityTypeAPIKey         SecuritySchemeType = "apiKey"
	SecurityTypeHTTP           SecuritySchemeType = "http"
	SecurityTypeOAuth2         SecuritySchemeType = "oauth2"
	SecurityTypeOpenIDConnect  SecuritySchemeType = "openIdConnect"
)

// OAuthFlows holds the OAuth 2.0 flow configurations.
type OAuthFlows struct {
	AuthorizationCode *OAuthFlow
	ClientCredentials *OAuthFlow
	DeviceCode        *OAuthFlow
}

// OAuthFlow describes a single OAuth 2.0 flow.
type OAuthFlow struct {
	AuthorizationURL string
	TokenURL         string
	RefreshURL       string
	Scopes           map[string]string
}

// DisplayMode controls how an operation is rendered in the TUI.
type DisplayMode string

const (
	DisplayTable  DisplayMode = "table"
	DisplayDetail DisplayMode = "detail"
	DisplayForm   DisplayMode = "form"
	DisplayCustom DisplayMode = "custom"
)

// PaginationConfig describes how a paginated operation should be handled.
type PaginationConfig struct {
	Type           PaginationType
	InputCursor    PaginationInput
	InputLimit     PaginationInput
	OutputResults  string // JSONPath to results array
	OutputNextKey  string // JSONPath to next cursor/page/URL
	OutputTotal    string // JSONPath to total count (optional)
	DefaultLimit   int
}

// PaginationType is the pagination strategy.
type PaginationType string

const (
	PaginationOffset PaginationType = "offset"
	PaginationPage   PaginationType = "page"
	PaginationCursor PaginationType = "cursor"
	PaginationLink   PaginationType = "link"
	PaginationURL    PaginationType = "url"
)

// PaginationInput describes a pagination parameter.
type PaginationInput struct {
	Name     string
	In       ParamLocation
	Default  int
}

// RetryConfig describes retry behavior for an operation.
type RetryConfig struct {
	Enabled              bool
	Strategy             string // "exponential-backoff"
	InitialInterval      time.Duration
	MaxInterval          time.Duration
	MaxElapsedTime       time.Duration
	Exponent             float64
	Jitter               bool
	StatusCodes          []int
	RetryConnectionErrors bool
}

// ServerConfig describes an API server from the OpenAPI spec.
type ServerConfig struct {
	URL         string
	Description string
	Variables   map[string]ServerVariable
}

// ServerVariable describes a server URL template variable.
type ServerVariable struct {
	Default     string
	Description string
	Enum        []string
}

// HookContext is the data passed to before/after request hooks.
// Serialized as JSON for shell hooks; passed as gRPC message for go-plugin hooks.
type HookContext struct {
	OperationID     string            `json:"operation_id"`
	Method          string            `json:"method"`
	URL             string            `json:"url"`
	Headers         map[string]string `json:"headers"`
	Body            []byte            `json:"body,omitempty"`
	Timestamp       string            `json:"timestamp"`
	StatusCode      int               `json:"status_code,omitempty"`
	ResponseHeaders map[string]string `json:"response_headers,omitempty"`
	ResponseBody    []byte            `json:"response_body,omitempty"`
	ElapsedMs       int64             `json:"elapsed_ms,omitempty"`
	Error           string            `json:"error,omitempty"`
}

// Registry holds all parsed and enriched operation metadata.
type Registry struct {
	Title           string
	Description     string
	Version         string
	Servers         []ServerConfig
	SecuritySchemes map[string]SecurityScheme
	Operations      []OperationMeta
	TagGroups       map[string][]OperationMeta
}
