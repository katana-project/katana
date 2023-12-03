package katana

import _ "embed"

//go:generate go run github.com/katana-project/ogen/cmd/ogen@main --config ./server/api/ogen.yml --package v1 --target ./server/api/v1 ./server/api/schema/v1.yml

//go:embed config.example.toml
var ExampleConfig []byte
