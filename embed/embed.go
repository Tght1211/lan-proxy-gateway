// Package embed exposes the baked-in mihomo skeleton template and rule data.
package embed

import (
	_ "embed"
)

//go:embed template.yaml
var Template string
