// Package packages manages GitHub Packages maintenance from the CLI.
//
// It provides CommandBuilder for wiring Cobra commands, PurgeService for
// deleting untagged container versions, configuration helpers, and token
// resolution utilities that integrate with GHCR and external credentials.
package packages
