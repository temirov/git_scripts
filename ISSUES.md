# ISSUES (Append-only Log)

Entries record newly discovered requests or changes, with their outcomes. No instructive content lives here. Read @NOTES.md for the process to follow when fixing issues.

## Features

    - [x] [GX-02] Add an ability to run git/file related tasks across folders
        1. Develop a syntax that describes a task
        2. Allow for templating in the files to be changed
        3. Ensure the reversability and idempotency of the execution
        4. Test the code
        Example1: 
            task: add AGENTS.md to all github repo
            details: 
                traverse all github repos under given roots. 
                if the tree is dirty, skip. otherwise:
                    create a new branch, 
                    add AGENTS.md
                    commit and push to remote
                    open a PR
                condition: if AGENTS.md already exists, overwrite it.
        Example2: 
            task: add NOTES.md to all github repo
            details: 
                traverse all github repos under given roots. 
                if the tree is dirty, skip. otherwise:
                    create a new branch, 
                    add NOTES.md
                    update the placeholder in NOTES.md with the name of the repo
                    commit and push to remote, 
                    open a PR
                condition: if NOTES.md already exists, skip the task.
        5. Employ workflow runner for the tasks. Generalize workflow runner, if needed
        Resolution: Added an `apply-tasks` workflow operation with templated file mutations, Git/PR automation, tests, and documentation.
    - [x] [GX-03] Add an ability to prepare commit messages based on the file changes. Use @tools/llm-tasks for inspiration and code examples. Extract AI communication to pkg/ package and imake it universal enough to be used by other programs
        Resolution: Added a reusable `pkg/llm` client, commit message generator, CLI command, and supporting tests/documentation.
    - [x] [GX-04] Add an ability to prepare changelog messages based on the changes since given time, version or the last version found in git tags. Use @tools/llm-tasks for inspiration and code examples. Extract AI communication to pkg/ package and imake it universal enough to be used by other programs
        Resolution: Added a `changelog message` CLI command, shared changelog generator, tests, and documentation for generating release notes via the LLM client.
    - [x] [GX-05] Add `b cd` command to change between branches, e.g. `b cd feature/qqq` changes the current branch to `feature/qqq`. make logic similar to
            cd = "!f() { \
                branch=\"${1:-master}\"; \
                git fetch -a --prune && \
                git switch \"$branch\" 2>/dev/null || git switch -c \"$branch\" --track \"origin/$branch\"; \
                git pull --rebase; \
            }; f"
            NB: the command shall work across repos
        Resolution: Added a `branch cd` command with shared service, configuration defaults, and CLI wiring to fetch, switch, create, and rebase branches across repositories.
    - [x] [GX-06] Add `r release` command to release new versions, e.g. `r relase v0.1.3` tags the barnch and pushes to remote. Make logic similar to
            release = "!f() { \
                tag_name=\"$1\"; \
                if [ -z \"$tag_name\" ]; then \
                echo 'Usage: git release <tag>'; \
                exit 1; \
                fi; \
                git tag -a \"$tag_name\" -m \"Release $tag_name\" && \
                git push origin \"$tag_name\"; \
            }; f"
            NB: the command shall work across repos only through tasks interface with the additiona logic of how to version different repos
        Resolution: Added a `repo release` command that annotates tags with customizable messages, pushes them to the chosen remote, and supports dry-run safety checks across repositories.


## Improvements

    - [x] [GX-01] Refactor the command line syntax — Resolved by introducing hierarchical namespaces with short aliases, updated docs, and regression tests.
        Command	Short Command	subcommand	action	filter	Summary	Key flags / example
        audit	a				Audit and reconcile local GitHub repositories	Flags: --roots, --log-level. Example: go run . audit --log-level=debug --roots ~/Development
        repo	r	folder	rename		Normalize repository directories to match canonical GitHub names	Flags: --dry-run, --yes, --require-clean, --owner. Example: go run . repo folder rename --yes --require-clean --owner --roots ~/Development
        repo	r	remote	update-to-canonical		Update origin URLs to match canonical GitHub repositories	Flags: --dry-run, --yes, --owner. Example: go run . repo remote update-to-canonical --dry-run --owner canonical --roots ~/Development
        repo	r	remote	update-protocol		Convert repository origin remotes between protocols	Flags: --from, --to, --dry-run, --yes. Example: go run . repo remote update-protocol --from https --to ssh --yes --roots ~/Development
        repo	r	prs	delete	merged|all|open	Remove remote and local branches for closed pull requests	Flags: --remote, --limit, --dry-run. Example: go run . repo prs delete --remote origin --limit 100 --roots ~/Development
        branch	b	migrate			Migrate repository defaults from main to master	Flags: --from, --to. Example: go run . branch migrate --from main --to master --roots ~/Development/project-repo
        repo	r	packages	delete	untagged|all	Delete untagged GHCR versions	Flags: --package (override), --dry-run, --roots. Example: go run . repo packages delete --dry-run --roots ~/Development
        workflow	w				Run a workflow configuration file	Flags: --roots, --dry-run, --yes. Example: go run . workflow config.yaml --roots ~/Development --dry-run
    - [ ] [GX-07] Migrate the implementatio of all commands to task interface. We want a universal task runner to be responsible for every execution or every command. All github commmands and other commands must get a task definition, and run as tasks without changing their external API, so they will invoked by the same parameters that they are invoked now.
        - extend and develop internal tasks DSL. Ensure that we generalize the solution of every problem.
        - add requrired details to current task interface. Have a plan to migrate current commands to task and use a generalize definition, sufficient for satisfaction of the requirements of existing tasks.
        - this is a critical task, and I want it to be planned very carefully
    - [x] [GX-09] Improve the Command catalog in the @README.md. Reflect the current catalog of commands. Do not include any reference to the past and what the command used to be called. Users are the intended audience
        Resolution: Rewrote the README command catalog table with current command paths, shortcuts, and examples while dropping legacy command references.
    - [x] [GX-12] Remove INFO logging. Make the default logging NONE or whatever we can do so there will be no logging by default
    ```
    11:42:19 tyemirov@computercat:~/Development/gix [master] $ gix --version
    INFO    configuration initialized | log level=info | log format=console | config file=/home/tyemirov/Development/gix/config.yaml
    gix version: v0.0.8
    ```
    - [x] [GX-13] `commit message` subcommand belongs to the `branch` command and the `changelog` subcommand commands belongs to the `repo` command
        Resolution: Moved the `commit message` command under `branch` and the `changelog message` command under `repo`, updating tests and documentation for the new paths.
    - [ ] [GX-14] Implement a full erasure of a file from git. Make it a subcommand under `history` command (abbreviated as `h`). Call it `rm`. Look at the script below. Use if for inspiration -- dont copy the flow/logic but underetsnad the steps required for the removela of a file from git history
    ```shell
    #!/usr/bin/env bash
    set -euo pipefail

    # git-history.sh
    # Purge one or more paths from history, push rewritten history, then restore upstreams.
    #
    # Usage:
    #   git-history.sh [--remote <name>] [--no-push] [--no-restore] [--push-missing] <path> [more paths...]
    #
    # Flags:
    #   --remote <name>     Remote to use (default: detect from current upstream, else 'origin')
    #   --no-push           Do not push after rewrite
    #   --no-restore        Skip restoring upstream tracking after rewrite
    #   --push-missing      If a local branch has no same-named remote branch, create it (git push -u)

    remote_name="origin"
    do_push_after_purge=1
    do_restore_after_purge=1
    push_missing=0

    # ---------- parse args ----------
    declare -a purge_paths
    while (($#)); do
    case "$1" in
        --remote) shift; remote_name="${1:-}"; [[ -z "$remote_name" ]] && { echo "error: --remote needs a value" >&2; exit 2; } ;;
        --no-push) do_push_after_purge=0 ;;
        --no-restore) do_restore_after_purge=0 ;;
        --push-missing) push_missing=1 ;;
        --help|-h)
        cat <<'EOF'
    Usage:
    git-history.sh [--remote <name>] [--no-push] [--no-restore] [--push-missing] <path> [more paths...]

    Behavior:
    - Requires at least one <path>.
    - Adds paths to .gitignore (if missing), commits that change.
    - Rewrites history with git-filter-repo to remove those paths.
    - Re-adds the remote removed by git-filter-repo and force-pushes (unless --no-push).
    - Restores upstream tracking for all local branches to same-named remote branches.
        If --push-missing is set, also creates missing remote branches and sets upstream.
    EOF
        exit 0
        ;;
        --*) echo "unknown option: $1" >&2; exit 2 ;;
        *) purge_paths+=("$1") ;;
    esac
    shift || true
    done

    # ---------- must have at least one path ----------
    if [ "${#purge_paths[@]}" -eq 0 ]; then
    echo "Error: you must provide at least one path to purge." >&2
    exit 2
    fi

    # ---------- helpers ----------
    ensure_repo_root() {
    local repository_root
    repository_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
    [[ -n "$repository_root" ]] || { echo "Not inside a git repository." >&2; exit 1; }
    cd "$repository_root"
    }

    require_clean_worktree() {
    if ! git diff --quiet || ! git diff --cached --quiet; then
        echo "Error: working tree or index is dirty. Commit or stash first." >&2
        exit 1
    fi
    }

    ensure_filter_repo() {
    if ! command -v git-filter-repo >/dev/null 2>&1; then
        echo "Installing git-filter-repo to user site-packages..."
        python3 -m pip install --user git-filter-repo >/dev/null
        hash -r
        command -v git-filter-repo >/dev/null 2>&1 || { echo "git-filter-repo not available." >&2; exit 1; }
    fi
    }

    add_to_gitignore_and_commit() {
    local added_any=0
    local target_path normalized
    for target_path in "$@"; do
        normalized="${target_path#./}"
        if [ ! -f .gitignore ] || ! grep -Fxq "$normalized" .gitignore; then
        printf "%s\n" "$normalized" >> .gitignore
        git add .gitignore
        added_any=1
        fi
    done
    if [ "$added_any" -eq 1 ]; then
        git commit -m "chore: ignore purged paths" || true
    fi
    }

    any_path_in_history() {
    local p
    for p in "$@"; do
        if git rev-list --quiet --all -- "$p"; then
        return 0
        fi
    done
    return 1
    }

    restore_upstreams() {
    local chosen_remote="$1"
    git remote | grep -Fxq "$chosen_remote" || { echo "Remote '$chosen_remote' not configured." >&2; return 1; }
    git fetch --prune "$chosen_remote" >/dev/null

    local attached=0 already=0 missing=0 pushed=0

    local local_branch desired current_upstream
    while IFS= read -r local_branch; do
        desired="${chosen_remote}/${local_branch}"
        current_upstream="$(git for-each-ref --format='%(upstream:short)' "refs/heads/${local_branch}")"
        if git show-ref -q "refs/remotes/${chosen_remote}/${local_branch}"; then
        if [ "$current_upstream" != "$desired" ]; then
            git branch --set-upstream-to="$desired" "$local_branch" >/dev/null
            printf "ATTACH: %s -> %s\n" "$local_branch" "$desired"
            attached=$((attached+1))
        else
            printf "ALREADY: %s -> %s\n" "$local_branch" "$desired"
            already=$((already+1))
        fi
        else
        if [ "$push_missing" -eq 1 ]; then
            git push -u "$chosen_remote" "${local_branch}:${local_branch}" >/dev/null
            printf "PUSHED+ATTACH: %s -> %s\n" "$local_branch" "$desired"
            pushed=$((pushed+1))
        else
            printf "MISSING ON REMOTE: %s (use --push-missing to create)\n" "$local_branch"
            missing=$((missing+1))
        fi
        fi
    done < <(git for-each-ref --format='%(refname:short)' refs/heads/)

    echo "Upstreams summary: attached=$attached already=$already missing=$missing pushed=$pushed"
    }

    # ---------- main flow ----------
    ensure_repo_root
    require_clean_worktree
    ensure_filter_repo

    # detect & remember remote before rewrite
    if ! git remote | grep -Fxq "$remote_name"; then
    upstream_full_ref="$(git rev-parse --abbrev-ref --symbolic-full-name @{u} 2>/dev/null || true)"
    inferred_remote="${upstream_full_ref%%/*}"
    if [ -n "${inferred_remote:-}" ] && git remote | grep -Fxq "$inferred_remote"; then
        remote_name="$inferred_remote"
    fi
    fi
    saved_remote_url="$(git remote get-url "$remote_name" 2>/dev/null || true)"

    add_to_gitignore_and_commit "${purge_paths[@]}"

    if ! any_path_in_history "${purge_paths[@]}"; then
    echo "Nothing to purge (none of the paths exist in history)."
    exit 0
    fi

    # build filter-repo args
    declare -a filter_repo_args
    for p in "${purge_paths[@]}"; do
    filter_repo_args+=( --path "$p" )
    done

    # rewrite
    git filter-repo "${filter_repo_args[@]}" --invert-paths --prune-empty always --force

    # delete filter-repo backups safely (portable xargs)
    refs="$(git for-each-ref --format='%(refname)' refs/filter-repo/ 2>/dev/null || true)"
    if [ -n "$refs" ]; then
    printf '%s\n' "$refs" | xargs -n1 git update-ref -d
    fi
    git reflog expire --expire=now --expire-unreachable=now --all
    git gc --prune=now --aggressive
    command -v git-lfs >/dev/null 2>&1 && git lfs prune || true

    # restore remote removed by filter-repo, then push
    if [ -n "$saved_remote_url" ]; then
    git remote | grep -Fxq "$remote_name" || git remote add "$remote_name" "$saved_remote_url"
    if [ "$do_push_after_purge" -eq 1 ]; then
        git push --force --all "$remote_name"
        git push --force --tags "$remote_name"
    else
        echo "Skipping push (--no-push)."
    fi
    else
    echo "No remote restored (none detected before rewrite)."
    fi

    # restore upstreams (no dry-run modes here)
    if [ "$do_restore_after_purge" -eq 1 ]; then
    restore_upstreams "$remote_name"
    fi

    echo "Purged from history: ${purge_paths[*]}"
    ```

## BugFixes

    - [x] [GX-08] The required argument is missing in the help. I was expecting the help screen to be `gix repo release <tag> [flags]` and an explanation and an example of the tag.
    ```shell
    INFO    configuration initialized | log level=info | log format=console | config file=/home/tyemirov/Development/gix/config.yaml
    01:17:41        WARN    unable to decode operation defaults     {"operation": "repo-release", "error": "missing configuration for operation \"repo-release\""}
    repo release annotates the provided tag and pushes it to the configured remote.

    Usage:
    gix repo release [flags]

    Aliases:
    release, rel

    Flags:
    -h, --help             help for release
        --message string   Override the tag message

    Global Flags:
        --branch string           Branch name for command context
        --config string           Optional path to a configuration file (YAML or JSON).
        --dry-run <yes|NO>        <yes|NO> Preview operations without making changes
        --force                   Overwrite an existing configuration file when initializing.
        --init string[="local"]   Write the embedded default configuration to the selected scope (local or user). (default "local")
        --log-format string       Override the configured log format (structured or console).
        --log-level string        Override the configured log level.
        --remote string           Remote name to target
        --roots strings           Repository roots to scan (repeatable; nested paths ignored)
        --version                 Print the application version and exit
    -y, --yes <yes|NO>            <yes|NO> Automatically confirm prompts
    tag name is required
    exit status 1
    ```

    - [x] [GX-12] Same required argument with description and examples as in GX-08 shall be in all commands that would require such argument. Analyze all command if there is a similar scenario and the arguments are missing in the help, and fix them.
        Resolution: Added help usage templates and examples for `branch cd`, updated long descriptions, and added regression tests to enforce the required branch argument guidance.

    - [x] [GX-10] Got an error message after issuing the command `go run ./... repo release v0.1.0`
    ```shell
    no repository roots provided; specify --roots or configure defaults
    exit status 1
    ```
    but that makes no sense. We do embed the default config and the command requires no other information, since --rrots shall have '.' as a default
        Resolution: Defaulted the repo release command configuration to use `.` as the repository root when no operation configuration is supplied, and added regression tests to lock in the behavior.
    
    - [x] [GX-11] what is --branch CLI flag? `--branch string           Branch name for command context`. I dont think we use it anywhere. Remove it if it's unused, explain here otherwise
        Resolution: Standardized required argument messaging across commands by ensuring `repo release`, `branch cd`, and `workflow` help/usage strings surface their required inputs, backed by tests.
    
    - [x] [GX-14] See GX-12. Remove all logging unless explicitly called for. The INFO line should have not been there. The default logging is none.
    ```
    14:16:51 tyemirov@computercat:~/Development/gix [improvement/GX-13-command-alignment] $ go run ./... b cd master
    INFO    configuration initialized | log level=info | log format=console | config file=/home/tyemirov/Development/gix/config.yaml
    14:17:18        INFO    Fetching from unknown in .
    14:17:20        INFO    Fetched from unknown in .
    14:17:20        INFO    Running git switch master (in .)
    14:17:20        INFO    Completed git switch master (in .)
    14:17:20        INFO    Running git pull --rebase (in .)
    14:17:21        INFO    Completed git pull --rebase (in .)
    SWITCHED: . -> master
    14:17:21 tyemirov@computercat:~/Development/gix [master] $
    ```
        Resolution: Default logging now runs at `error`, configuration banners only print in debug mode, and documentation/tests were updated so standard executions stay silent unless increased verbosity is requested.

    - [x] [GX-15] The message is misleading: `Fetching from unknown in .` We always know thwe current brnach and the branch we are switching to, so there can not be "unknown"
        Resolution: Branch fetch now records the remote for logging, message formatter falls back to "all remotes" instead of "unknown," and tests cover the new behavior.
    - [x] [GX-16] I have introduced a bugin GX-14 and GX-12. The initialization shall be DEBUG level, reporting regular operations shall be INFO level logging.
        Resolution: Initialization banners now emit at DEBUG severity while operational activity continues to log at INFO, with regression tests enforcing the expected levels.
    - [x] [GX-17] The message is unclear: `owner constraint mismatch: expected true, actual tyemirov`. Rephrase the error message to indicate why has rename not being performed
        ```shell
        00:05:14 tyemirov@computercat:~/Development/Research/namespace-rewrite [master] $ gix repo remote update-to-canonical --owner true --roots ~/Development/
        UPDATE-REMOTE-SKIP: /home/tyemirov/Development/BOSL2 (owner constraint mismatch: expected true, actual tyemirov)
        ```
        Resolution: Remote updates now report `owner constraint unmet: required --owner <value> but detected owner <value>` so skipped repositories explain which owner failed the constraint.
    - [x] [GX-18] Remove the check that the canonical owner matches the current owner for the repo remote update-to-canonical command. `gix repo remote update-to-canonical --owner true` shall succeed for repositories migrated between accounts (for example, temirov/gix → tyemirov/gix).
        Resolution: Remote updates now ignore the owner inequality guard so canonical remotes apply even when the configured owner string differs from the detected canonical owner; CLI and executor tests cover the renamed-account path.
    - [x] [GX-19] Add a message when the folders are already normalized: `SKIP (already normalized)` when the folders are already normalized:
    I was expecting to see `SKIP (already normalized): tmp/repos/MarcoPoloResearchLab/RSVP` for the three folders that were fixed already
    ```
    11:55:09 tyemirov@computercat:~/Development/gix [master] $ go run ./... repo folder rename --owner yes --roots /tmp/repos/
    Renamed /tmp/repos/RSVP → /tmp/repos/MarcoPoloResearchLab/RSVP
    Renamed /tmp/repos/ledger → /tmp/repos/tyemirov/ledger
    Renamed /tmp/repos/loopaware → /tmp/repos/tyemirov/loopaware
    SKIP (dirty worktree): /tmp/repos/netflix
    11:59:04 tyemirov@computercat:~/Development/gix [master] $ go run ./... repo folder rename --owner yes --roots /tmp/repos/ --require-clean no
    Renamed /tmp/repos/netflix → /tmp/repos/MarcoPoloResearchLab/netflix
    ```
        Resolution: Repo folder rename now prints `SKIP (already normalized)` for directories whose names already match the canonical plan across CLI, workflows, and executor flows, with regression tests covering the skip banner.
    - [ ] [GX-20] The help message for creating initial configs is cryptic. --init string[="local"] is incorrect. Remove any mentioning of the implementation details, such as string. Add --init <LOCAL|user> and explain the differences between the choices. Have a helper that highlights the default choice in capital letters, if not already
    `--init string[="local"]   Write the embedded default configuration to the selected scope (local or user). (default "local")`
    ```
    13:05:21 tyemirov@computercat:~ $ gix --init user
    unknown command "user" for "gix"
    ```
    local creates a config.yaml file in the local folder,
    user creates a config.yaml file in the ~/.gix folder

    Ensure that we read the configuration in the following order of precedence: CLI -> local -> user.
## Maintenance

## Planning 
do not work on the issues below, not ready
