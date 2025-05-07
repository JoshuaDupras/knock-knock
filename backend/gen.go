//go:build tools
// +build tools

package gen

//go:generate oapi-codegen -config=config.yaml ../chat-api.yaml
