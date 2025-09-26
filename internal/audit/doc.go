// Package audit implements repository discovery, reporting, and reconciliation
// workflows used by the git_scripts CLI.
//
// It exposes CommandBuilder for wiring the audit Cobra command, Service for driving the
// workflow programmatically, and supporting abstractions for Git, GitHub, file
// system, and prompting collaborators.
package audit
