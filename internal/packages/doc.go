// Package packages manages GitHub Packages maintenance from the CLI.
//
// It provides CommandBuilder for wiring Cobra commands, PurgeService for
// deleting untagged container versions, configuration helpers (including
// service_base_url and page_size overrides), and token resolution utilities
// that integrate with GHCR and external credentials.
package packages
