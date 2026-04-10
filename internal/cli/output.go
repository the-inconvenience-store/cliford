package cli

// OutputFormat represents a supported output format for generated apps.
type OutputFormat string

const (
	OutputPretty OutputFormat = "pretty"
	OutputJSON   OutputFormat = "json"
	OutputYAML   OutputFormat = "yaml"
	OutputTable  OutputFormat = "table"
)

// ValidOutputFormats returns all valid output format strings.
func ValidOutputFormats() []string {
	return []string{
		string(OutputPretty),
		string(OutputJSON),
		string(OutputYAML),
		string(OutputTable),
	}
}
