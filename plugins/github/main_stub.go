//go:build !plugin

// main_stub.go provides a no-op main for non-plugin builds so `go build ./...`
// succeeds on this directory. Plugin .so builds use `-tags plugin` (via
// `-buildmode=plugin`, which sets the plugin build mode) and ignore this file;
// they export the package-level Plugin symbol instead of running main.
//
// The binary produced by a plain `go build` of this package is never intended
// to be executed — it exists only to keep whole-repo build tooling green. The
// canonical build is `make plugin NAME=github`.
package main

func main() {}
