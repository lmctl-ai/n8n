package host

import _ "embed"

// Schema is the fixed LMCode contract used for loading and result validation.
//
//go:embed schema.json
var Schema []byte
