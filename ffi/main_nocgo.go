//go:build !cgo

package main

// main keeps `go build ./...` working when cgo is disabled.
// The c-shared exports live in exports.go and are built with CGO_ENABLED=1.
func main() {}
