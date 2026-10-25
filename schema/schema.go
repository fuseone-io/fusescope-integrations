// Package schema holds the JSON Schema of manifest.yaml.
package schema

import _ "embed"

//go:embed manifest.schema.json
var Manifest []byte
