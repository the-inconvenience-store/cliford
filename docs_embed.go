// Package cliford embeds Cliford's own documentation so the CLI can display it
// via the `cliford docs` command.
package cliford

import "embed"

// DocsFS contains the Markdown documentation files shipped with Cliford.
//
//go:embed docs/*.md
var DocsFS embed.FS
