// Package cli constructs the git_scripts command-line interface.
//
// It wires the Cobra root command with configuration loading, structured
// logging, and command registration for the audit, branch cleanup, migration,
// and packages subcommands. The package exposes the Application type used by
// main along with NewApplication and Execute helpers so the CLI can be reused
// programmatically.
package cli
