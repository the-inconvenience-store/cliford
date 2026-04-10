// Package theme defines the public types for Cliford's TUI theme configuration.
package theme

// Config holds the complete theme configuration for a generated TUI app.
type Config struct {
	Colors     ColorConfig     `mapstructure:"colors"`
	Borders    BorderStyle     `mapstructure:"borders"`
	Spinner    string          `mapstructure:"spinner"`
	Table      TableConfig     `mapstructure:"table"`
	Compact    bool            `mapstructure:"compact"`
}

// ColorConfig defines the semantic color palette for the TUI.
type ColorConfig struct {
	Primary    string `mapstructure:"primary"`
	Secondary  string `mapstructure:"secondary"`
	Accent     string `mapstructure:"accent"`
	Background string `mapstructure:"background"`
	Text       string `mapstructure:"text"`
	Dimmed     string `mapstructure:"dimmed"`
	Error      string `mapstructure:"error"`
	Success    string `mapstructure:"success"`
	Warning    string `mapstructure:"warning"`
}

// TableConfig controls the appearance of table output in the TUI.
type TableConfig struct {
	HeaderBold bool `mapstructure:"headerBold"`
	StripeRows bool `mapstructure:"stripeRows"`
}

// BorderStyle is the visual style for borders in the TUI.
type BorderStyle string

const (
	BorderNormal  BorderStyle = "normal"
	BorderRounded BorderStyle = "rounded"
	BorderThick   BorderStyle = "thick"
	BorderDouble  BorderStyle = "double"
)

// DefaultConfig returns a theme config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Colors: ColorConfig{
			Primary:    "#7D56F4",
			Secondary:  "#FF6B6B",
			Accent:     "#4ECDC4",
			Background: "#1A1A2E",
			Text:       "#EAEAEA",
			Dimmed:     "#666666",
			Error:      "#FF4444",
			Success:    "#44FF44",
			Warning:    "#FFAA00",
		},
		Borders: BorderRounded,
		Spinner: "dot",
		Table: TableConfig{
			HeaderBold: true,
			StripeRows: true,
		},
		Compact: false,
	}
}
