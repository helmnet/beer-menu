// Package api embeds the API contract so later features can serve it.
package api

import _ "embed"

// OpenAPI is the API contract, api/openapi.yaml. Every route in httpapi.Routes must be
// documented in it.
//
//go:embed openapi.yaml
var OpenAPI []byte
// tmp 016 check
