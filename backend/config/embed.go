// Package countrydata embeds country configuration in the distributable binary.
package countrydata

import _ "embed"

// JSON is a YAML 1.2 subset, so this resource is parsed with the standard library.
//
//go:embed countries.yaml
var resource []byte

// Read returns a copy to keep the embedded source immutable to callers.
func Read() []byte { return append([]byte(nil), resource...) }
