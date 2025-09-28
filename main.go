package main

import (
	"fmt"
	"os"

	"github.com/temirov/gix/cmd/cli"
)

const (
	exitErrorTemplateConstant = "%v\n"
)

// main executes the git_scripts command-line application.
func main() {
	if executionError := cli.Execute(); executionError != nil {
		fmt.Fprintf(os.Stderr, exitErrorTemplateConstant, executionError)
		os.Exit(1)
	}
}
