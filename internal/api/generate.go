// Package api is the client's contract (api/openapi.json) as Go: the models,
// the router and StrictServerInterface (api.gen.go, by oapi-codegen), and
// Unimplemented (unimplemented.gen.go, by tools/stubgen), which answers
// every operation not built yet. Both are generated: make generate, never by hand.
package api

//go:generate go tool oapi-codegen -config ../../api/oapi-codegen.yaml -o api.gen.go ../../api/openapi.json
//go:generate go run ../../tools/stubgen -in api.gen.go -out unimplemented.gen.go
