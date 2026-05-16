package strategies

import "embed"

// FS contains the built-in strategy YAML files.
//
//go:embed *.yaml
var FS embed.FS
