package katana

import _ "embed"

// OpenAPI server generation has template overrides at ./server/api/v1/templates

// Changes in the templates:
// - expose the http.Request for strict response visit

//go:generate go run github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@latest --config ./server/api/v1/models.cfg.yaml -o ./server/api/v1/models.gen.go ./server/api/schema/v1.yaml
//go:generate go run github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@latest --config ./server/api/v1/server.cfg.yaml --templates ./server/api/v1/templates -o ./server/api/v1/server.gen.go ./server/api/schema/v1.yaml

//go:embed config.example.toml
var ExampleConfig []byte
