package katana

import _ "embed"

//go:generate go run github.com/katana-project/ogen/cmd/ogen@main --config ./server/ogen.yml --package api --target ./server/api ./server/openapi.yml

//go:embed config.example.toml
var ExampleConfig []byte
