// Package execshell provides structured helpers for invoking external tools.
//
// It wraps os/exec with logging and timeouts via ShellExecutor, exposes
// OSCommandRunner for default process execution, and defines abstractions used
// throughout gix to run git, gh, and other CLIs in a testable manner.
package execshell
