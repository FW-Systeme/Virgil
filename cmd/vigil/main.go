package main

import (
	"os"

	"github.com/chris576/vigil/internal/cli"
)

var version = "dev"

func main() {
	cli.SetVersion(version)

	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
