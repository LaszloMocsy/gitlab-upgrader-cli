package config

import _ "embed"

//go:embed template.yaml
var template []byte

// Template returns the annotated default config file that
// `gluper config init` writes.
func Template() []byte { return append([]byte(nil), template...) }
