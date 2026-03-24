package main

import (
	"fmt"
	"os"

	"github.com/kayden-kim/oc/internal/app"
	"github.com/kayden-kim/oc/internal/runner"
)

var version = "v0.1.5" // Overridden by ldflags at build time

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(version)
		os.Exit(0)
	}
	if err := app.Run(os.Args[1:], version); err != nil {
		if exitErr, ok := runner.IsExitCode(err); ok {
			os.Exit(exitErr.Code)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
