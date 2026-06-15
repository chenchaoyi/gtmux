//go:build !darwin

// Portable stub so `go build ./...` / vet / staticcheck / tests stay green on
// Linux CI without pulling in the cgo systray dependency. The real menu-bar app
// (main_darwin.go) is macOS-only.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "gtmux-menubar is macOS-only")
	os.Exit(1)
}
