## [v0.0.4] - 2025-10-05

### Highlights

- Boolean CLI flags now accept yes/no style values, making overrides consistent across commands.
- Repository rename operations recognize owners and scaffold directories automatically when needed.

### Features ✨

- Add owner-aware repository rename planning and ensure parent directories are created when owner segments are requested.
- Allow repo-remote-update to enforce owner constraints when synchronizing remote configuration.

### Improvements ⚙️

- Introduce shared toggle parsing with CLI argument normalization so boolean flags accept yes/no/true/false inputs everywhere.
- Handle audit report generation when writing into nested output directories.

### Docs 📚

- _No updates._

### CI & Maintenance

- _No updates._

**Upgrade notes:** No breaking changes.

## [v0.0.3] - 2025-10-03

### Highlights

- Add configuration initialization workflow to CLI
- Extend CLI configuration search paths

**Upgrade notes:** No breaking changes.

## [v0.0.2] - 2025-10-03

### Highlights

- Feature: the --root flag is available for all commands
- Add apply-all confirmation state and prompt result struct
- Adjust CLI initialization logging for console
- Normalize repository discovery paths
- Add minimal audit inspection depth and update CLI usage
- Skip redundant gh repo view start log
- Add user config search path and tests
- Add embedded CLI configuration defaults

### Features ✨

- Feature: the --root flag is available for all commands
- Feature: require-clean safeguard is applied to all branch level operations
- Feature: migrate command accepts from and to flags
- Feature: flags are made global and working across all of the commands

### Improvements ⚙️

- BugFix: default log format is for the console

### Docs 📚

- _No updates._

### CI & Maintenance

- _No updates._

**Upgrade notes:** No breaking changes.

## [v0.0.1] - 2025-09-28

### What's New 🎉

1. Bash scripts replaced with Go implementation
2. The config.yaml file stores the defaults 
3. The config.yaml file defines a runnable workflow, chaining multiple commands
