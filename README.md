# Git/GitHub helper scripts

A small collection of practical Bash scripts that automate common Git and GitHub maintenance tasks.

- `delete_merged_branches.sh` — cleans up branches whose pull requests are closed, removing them both on origin and
  locally (if they still exist).
- `main_to_master.sh` — migrates a repository from default branch "main" to "master" safely and thoroughly.
- `remove_github_packages.sh` — deletes UNTAGGED versions of a GHCR container package while preserving tagged images.
- `audit_repos.sh` — audits GitHub repos across folders, optionally **renames local folders** to match the final GitHub
  repo name, **updates `origin` if the repo was renamed/transferred on GitHub**, and **converts remote URL protocols** (
  `https`, `git`, `ssh`).

## Prerequisites

These scripts assume a Unix-like environment (macOS or Linux) with the following tools:

- Bash 4+ and coreutils (sed, find, awk)
- git
- GitHub CLI: gh (and authenticated: run `gh auth login`)
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

Additionally, for `remove_github_packages.sh` you must provide a GitHub Personal Access Token (classic) with the
following scopes:

- `read:packages`
- `write:packages`
- `delete:packages`

Export it as an environment variable before running:

```bash
export GITHUB_PACKAGES_TOKEN=ghp_XXXXXXXXXXXXXXXXXXXXXXXXXXXX
````

## Usage

You can run scripts directly with `bash path/to/script.sh` or mark them executable:

```bash
chmod +x *.sh
./delete_merged_branches.sh
```

Run each script from **within** the target repository directory (where applicable).

---

### 1) delete\_merged\_branches.sh

Removes remote and local branches corresponding to closed pull requests in the current GitHub repository. The script:

* Collects existing remote branches on origin.
* Lists closed PRs via `gh` and extracts their head branch names.
* For each such branch, if it still exists on origin, deletes it remotely and locally; if it was already removed, it
  skips.

Notes and requirements:

* Requires `gh` authenticated against GitHub: `gh auth login`.
* Needs `origin` to point to the GitHub repo (used by `git ls-remote` and deletes).
* Uses PR state **"closed"** (includes merged and manually closed PRs). Review before running if you rely on non-merged
  but closed branches.

Example:

```bash
# From inside your repo
./delete_merged_branches.sh
```

---

### 2) main\_to\_master.sh

Safely switches a repository’s default branch from `main` to `master` and updates related configuration.

What it does:

* Verifies a clean working tree and that you are authenticated with `gh`.
* Ensures local `main` exists and is up to date; creates or fast-forwards `master` from `main` and pushes it.
* Rewrites GitHub Actions workflow branch filters from `main` to `master` (commits and pushes those changes).
* If GitHub Pages is using legacy branch-based publishing on `main`, reconfigures it to `master` (keeps the same path).
* Sets the repository default branch to `master` on GitHub.
* Rebases local branches created off `main` onto `master` and force-pushes them with lease when they have upstreams.
* Retargets open PRs whose base is `main` to base `master`.
* **Safety gates:** will **NOT** delete `main` if there are open PRs targeting `main`, branch protection on `main` is
  enabled, or any remaining references to `main` are detected in workflows.

Dependencies:

* `git`, `gh`, `jq`, `sed`, `find`

Usage:

```bash
# From inside your repo (ensure working tree is clean)
./main_to_master.sh
```

If a rebase conflict occurs for one of your local branches, follow the on-screen instructions, resolve, and re-run.

---

### 3) remove\_github\_packages.sh

Deletes **UNTAGGED** container versions from the GitHub Container Registry (GHCR) for a given owner/package, preserving
all **tagged** versions.

Config (edit in the script or export before running):

* `GITHUB_OWNER`: user or organization (default: `temirov`)
* `PACKAGE_NAME`: container package name in GHCR (default: `llm-proxy`)
* `OWNER_TYPE`: `"user"` or `"org"` (default: `user`)
* `GITHUB_PACKAGES_TOKEN`: token with `read:packages`, `write:packages`, `delete:packages` scopes (must be exported)
* `DRY_RUN`: set to `1` to preview deletions without performing them

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

* `curl` and `jq` installed
* The token must have `read:packages`, `write:packages`, `delete:packages`

Safety:

* Only **untagged** versions are deleted; tagged versions are preserved by design.

---

### 4) audit\_repos.sh

Scans one or more directory trees for Git repositories whose `origin` points to **GitHub**, then:

* **Audit mode** (`--audit`): prints a CSV to **stdout** with repo facts.

    * `final_github_repo` is the **canonical owner/repo** after following GitHub redirects (renames/transfers) via the
      GitHub API. Forks remain forks (we do **not** collapse to parents).
    * Adds `origin_matches_canonical` (`yes`/`no`/`n/a`) indicating whether the current `origin` path already matches
      the canonical.
    * Computes `in_sync` only when on the remote default branch and the remote protocol is SSH-capable (`git` or `ssh`)
      to avoid HTTPS prompts.

* **Folder rename (filesystem)** (`--rename [--dry-run|--yes]`): renames local directories to match the **final GitHub
  repo name** (canonical).

    * `--dry-run` prints `PLAN-OK` / `PLAN-CASE-ONLY` / `PLAN-SKIP` without changing anything.
    * `--yes` applies without per-repo prompts.
    * `--require-clean` skips repos with a dirty worktree.

* **Update remote on true rename/transfer** (`--update-remote [--dry-run|--yes]`): if GitHub reports a **canonical
  redirect** (`owner/repo` changed), updates `origin` to the **same protocol** but **new canonical path**.

    * **Skip reasons are explicit:**

        * `already canonical` — `origin` already matches the canonical path.
        * `no upstream` — no canonical redirect found (e.g., a fork that wasn’t renamed).
        * `error` — could not parse current origin or construct the target URL.
    * `--dry-run` prints `PLAN-UPDATE-REMOTE` lines without changing anything.
    * `--yes` applies without per-repo prompts.

* **Remote protocol conversion**:

    * Convert `origin` URL **form** using:

        * `--protocol-from (https|git|ssh)`
        * `--protocol-to   (https|git|ssh)`
        * Optional: `--dry-run` and `--yes`
    * Protocols are treated as distinct wire forms:

        * **git**  → `git@github.com:owner/repo.git` (SCP-like)
        * **ssh**  → `ssh://git@github.com/owner/repo.git`
        * **https** → `https://github.com/owner/repo.git`
    * When a canonical redirect exists, conversion uses the **canonical** owner/repo; otherwise it preserves the
      original (forks remain forks).

#### Prerequisites

* `bash`, `git`, `find` (+ coreutils), and `gh` (authenticated: `gh auth login`).
* Run from anywhere; pass one or more roots to scan (defaults to `.`).

#### Output discipline

* **Audit CSV** goes to **stdout** (and only when `--audit` is used *alone*).
* **Plans/success messages** (`PLAN-UPDATE-REMOTE`, `PLAN-CONVERT`, `PLAN-OK`, `Renamed`, `CONVERT-DONE`,
  `UPDATE-REMOTE-DONE`, etc.) go to **stdout**.
* **Errors** go to **stderr**.

#### CSV columns (in `--audit`)

```
final_github_repo,folder_name,name_matches,remote_default_branch,local_branch,in_sync,remote_protocol,origin_matches_canonical
```

* `final_github_repo` — **canonical** `owner/repo` after redirects (resolved via GitHub API).
* `folder_name` — current local directory name of the repo.
* `name_matches` — `yes` if `folder_name == repo`, else `no`.
* `remote_default_branch` — default branch on GitHub (e.g., `main`, `master`).
* `local_branch` — current local branch (or `DETACHED`).
* `in_sync` — `yes` / `no` / `n/a`. Computed only when:

    * `local_branch == remote_default_branch`, **and**
    * remote protocol is **git** or **ssh** (avoids HTTPS password prompts),
    * then the script fetches that branch and compares commit SHAs.
* `remote_protocol` — one of `git`, `ssh`, `https`, or `other`.
* `origin_matches_canonical` — `yes` if current `origin` owner/repo equals the canonical; `no` if different; `n/a` if
  unknown.

#### Common use cases

Audit all repos under `~/Development`:

```bash
./audit_repos.sh --audit ~/Development > audit.csv
```

Preview folder renames (no changes):

```bash
./audit_repos.sh --rename --dry-run ~/Development
```

Apply folder renames with confirmation per repo:

```bash
./audit_repos.sh --rename ~/Development
```

Apply folder renames without prompts, requiring clean worktrees:

```bash
./audit_repos.sh --rename --yes --require-clean ~/Development
```

**Update remotes on true rename/transfer:**

Dry-run:

```bash
./audit_repos.sh --update-remote --dry-run ~/Development
```

Apply (confirm per repo):

```bash
./audit_repos.sh --update-remote ~/Development
```

Apply without prompts:

```bash
./audit_repos.sh --update-remote --yes ~/Development
```

**Convert remote protocols:**

Dry-run convert **https → git@**:

```bash
./audit_repos.sh --protocol-from https --protocol-to git --dry-run ~/Development
```

Convert **git@ → ssh://** with confirmation:

```bash
./audit_repos.sh --protocol-from git --protocol-to ssh ~/Development
```

Convert **ssh:// → https** across all repos, no prompts:

```bash
./audit_repos.sh --protocol-from ssh --protocol-to https --yes ~/Development
```

#### Notes & safety

* Discovery finds any directory containing a `.git` dir or file (worktrees supported).
* Only GitHub remotes are processed (`github.com` in URL).
* HTTPS fetches are **not** attempted in `in_sync` to avoid credential prompts.
* Rename is **filesystem-level** (moves the repo directory). Case-only renames are handled safely via a two-step move on
  case-insensitive filesystems.
* `--update-remote` **does not** re-point forks to their upstream; it only acts when GitHub reports a **real redirect
  ** (rename or transfer) for the **same repository**.
