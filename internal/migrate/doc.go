// Package migrate implements default-branch migration orchestration.
//
// It coordinates Git operations, GitHub API updates, and safety checks through
// the Service type, and exposes CommandBuilder plus supporting option structs
// so the workflow can run from the CLI or be embedded in other tools.
package migrate
