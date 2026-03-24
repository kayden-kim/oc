package main

import (
	"fmt"
	"os"

	"github.com/kayden-kim/oc/internal/app"
	"github.com/kayden-kim/oc/internal/runner"
)

var version = "dev" // Overridden by ldflags at build time

var runApp = app.Run
var exitFunc = os.Exit

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(version)
		exitFunc(0)
	}
	if err := runApp(os.Args[1:], version); err != nil {
		if exitErr, ok := runner.IsExitCode(err); ok {
			exitFunc(exitErr.Code)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitFunc(1)
	}
}
