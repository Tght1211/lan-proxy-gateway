package template

import (
	_ "embed"
)

//go:embed template.yaml
var TemplateContent string
