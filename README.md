# Git/GitHub helper scripts

A small collection of practical Bash scripts that automate common Git and GitHub maintenance tasks.

- delete_merged_branches.sh — cleans up branches whose pull requests are closed, removing them both on origin and locally (if they still exist).
- main_to_master.sh — migrates a repository from default branch "main" to "master" safely and thoroughly.
- remove_github_packages.sh — deletes UNTAGGED versions of a GHCR container package while preserving tagged images.

## Prerequisites
These scripts assume a Unix-like environment (macOS or Linux) with the following tools:

- Bash 4+ and coreutils (sed, find, awk)
- git
- GitHub CLI: gh (and authenticated: run gh auth login)
- jq (JSON processor)
- curl

Installation links:
- GitHub CLI (gh): https://cli.github.com/manual/installation
- jq: https://jqlang.github.io/jq/download/
- git: https://git-scm.com/downloads
- curl: https://curl.se/download.html
- Bash: https://www.gnu.org/software/bash/
- GNU coreutils: https://www.gnu.org/software/coreutils/
- gh auth login docs: https://cli.github.com/manual/gh_auth_login

Additionally, for remove_github_packages.sh you must provide a GitHub Personal Access Token (classic) with the following scopes:
- read:packages
- write:packages
- delete:packages

Export it as an environment variable before running:

```bash
export GITHUB_PACKAGES_TOKEN=ghp_XXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

## Usage
You can run scripts directly with bash path/to/script.sh or mark them executable:

```bash
chmod +x *.sh
./delete_merged_branches.sh
```

Run each script from within the target repository directory (where applicable).

---

### 1) delete_merged_branches.sh
Removes remote and local branches corresponding to closed pull requests in the current GitHub repository. The script:
- Collects existing remote branches on origin.
- Lists closed PRs via gh and extracts their head branch names.
- For each such branch, if it still exists on origin, deletes it remotely and locally; if it was already removed, it skips.

Notes and requirements:
- Requires gh authenticated against GitHub: gh auth login.
- Needs origin to point to the GitHub repo (used by git ls-remote and deletes).
- Uses PR state "closed" (includes merged and manually closed PRs). Review before running if you rely on non-merged but closed branches.

Example:
```bash
# From inside your repo
./delete_merged_branches.sh
```

---

### 2) main_to_master.sh
Safely switches a repository’s default branch from main to master and updates related configuration.

What it does:
- Verifies a clean working tree and that you are authenticated with gh.
- Ensures local main exists and is up to date; creates or fast-forwards master from main and pushes it.
- Rewrites GitHub Actions workflow branch filters from main to master (commits and pushes those changes).
- If GitHub Pages is using legacy branch-based publishing on main, reconfigures it to master (keeps the same path).
- Sets the repository default branch to master on GitHub.
- Rebases local branches created off main onto master and force-pushes them with lease when they have upstreams.
- Retargets open PRs whose base is main to base master.
- Safety gates: will NOT delete main if there are open PRs targeting main, branch protection on main is enabled, or any remaining references to main are detected in workflows.

Dependencies:
- git, gh, jq, sed, find

Usage:
```bash
# From inside your repo (ensure working tree is clean)
./main_to_master.sh
```
If a rebase conflict occurs for one of your local branches, follow the on-screen instructions, resolve, and re-run.

---

### 3) remove_github_packages.sh
Deletes UNTAGGED container versions from the GitHub Container Registry (GHCR) for a given owner/package, preserving all tagged versions.

Config (edit in the script or export before running):
- GITHUB_OWNER: user or organization (default: temirov)
- PACKAGE_NAME: container package name in GHCR (default: llm-proxy)
- OWNER_TYPE: "user" or "org" (default: user)
- GITHUB_PACKAGES_TOKEN: token with read/write/delete:packages scopes (must be exported)
- DRY_RUN: set to 1 to preview deletions without performing them

Usage examples:
```bash
# Preview without deleting
export GITHUB_PACKAGES_TOKEN=ghp_XXXXXXXXXXXXXXXXXXXXXXXXXXXX
DRY_RUN=1 ./remove_github_packages.sh

# Actually delete untagged versions for a specific owner/package
export GITHUB_PACKAGES_TOKEN=ghp_XXXXXXXXXXXXXXXXXXXXXXXXXXXX
GITHUB_OWNER=my-org PACKAGE_NAME=my-image OWNER_TYPE=org ./remove_github_packages.sh
```

Requirements:
- curl and jq installed
- The token must have read:packages, write:packages, delete:packages

Safety:
- Only untagged versions are deleted; tagged versions are preserved by design.

## Installation tips
- Optionally place these scripts in a directory on your PATH (e.g., ~/bin) and make them executable.
- Always review scripts before running in sensitive environments.

## Troubleshooting
- gh not authenticated: run gh auth login
- Permission errors when deleting branches: ensure you have push rights to the repository.
- 403/404 from GHCR API: verify OWNER_TYPE (user vs org), package name, and that GITHUB_PACKAGES_TOKEN has the required scopes.
- jq not found: install via your package manager (e.g., brew install jq on macOS).
