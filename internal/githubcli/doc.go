// Package githubcli wraps the GitHub CLI for git_scripts workflows.
//
// It layers typed request and response structures for gh subcommands, exposes
// interfaces consumed by other packages, and integrates with execshell so
// interactions with GitHub can be mocked during testing.
package githubcli
