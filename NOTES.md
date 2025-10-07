# Notes

## Rules of engagement

Review the NOTES.md. Make a plan for autonomously fixing every item under Features, BugFixes, Improvements, Maintenance. Ensure no regressions. Ensure adding tests. Lean into integration tests. Fix every issue. Document the changes.

Fix issues one by one. Write a nice comprehensive commit message AFTER EACH issue is fixed and tested and covered with tests. Do not work on all issues at all. Work at one issue at a time sequntially. 

Remove an issue from the NOTES.md after the issue is fixed (new tests are passing). Commit the changes and push to the remote.

Leave Features, BugFixes, Improvements, Maintenance sections empty when all fixes are implemented but don't delete the sections themselves.

## Features

## Improvements

- [ ] [GX-04] Add --version flag and use the technique similar to the ctx.
Add a helper function (e.g., GetApplicationVersion) that returns a version string by first checking debug.ReadBuildInfo(). If that yields a
    non-empty, non-(devel) version, return it. Otherwise, look for the repository root, then try git describe --tags --exact-match, and finally
    git describe --tags --long --dirty; trim whitespace and fall back to a constant like "unknown" if every call fails.
  - In the Cobra root command constructor, bind a --version flag to a boolean. Use a PersistentPreRun hook to detect when the flag is set,
    print fmt.Printf("app version: %s\n", GetApplicationVersion()), and exit immediately with os.Exit(0) so no subcommands execute.
  - Ensure the flag is documented in usage text, and keep the version template in a constant to avoid repeated string literals.
  - Maintain clean GoDoc for exported APIs, run go fmt ./..., go vet ./..., and go test ./... before finishing.

## BugFixes

- [ ] [GX-05] when running Repo PRS purge command it didnt ask for the confirmation. ensure that the dialog about any distructive operations is codined in one place and is present for all of the operations that perform actual changes, plan first

## Maintenance
