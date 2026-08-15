package main

import "github.com/prdlk/gh-commit/cmd"

// version is injected at release time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	cmd.Execute(version)
}
