# Notes

## Role

You are a staff level full stack engineer. Your task is to **re-evaluate and refactor the GIX repository** according to the coding standards already written in **AGENTS.md**.

## Context

* AGENTS.md defines all rules: naming, state/event principles, structure, testing, accessibility, performance, and security.
* The repo uses Alpine.js, CDN scripts only, no bundlers.
* Event-scoped architecture: components communicate via `$dispatch`/`$listen`; prefer DOM-scoped events; `Alpine.store` only for true shared domain state.
* The backend uses Go language ecosystem

## Your tasks

1. **Read AGENTS.md first** → treat it as the *authoritative style guide*.
2. **Scan the codebase** → identify violations (inline handlers, globals, duplicated strings, lack of constants, cross-component state leakage, etc.).
3. **Generate PLAN.md** → bullet list of problems and refactors needed, scoped by file. PLAN.md is a part of PR metadata. It's a transient document outlining the work on a given issue.
4. **Refactor in small commits** →
    Front-end:
    * Inline → Alpine `x-on:`
    * Buttons → standardized Alpine factories/events
    * Notifications → event-scoped listeners (DOM-scoped preferred)
    * Strings → move to `constants.js`
    * Utilities → extract into `/js/utils/`
    * Composition → normalize `/js/app.js` as Alpine composition root
    Backend:
    * Use "object-oreinted" stye of functions attached to structs
    * Prioritize data-driven solutions over imperative approach
    * Design and use shared components
5. **Tests** → Add/adjust Puppeteer tests for key flows (button → event → notification; cross-panel isolation). Prioritize end-2-end and integration tests.
6. **Docs** → Update README and MIGRATION.md with new event contracts, removed globals, and developer instructions.
7. **Timeouts**  Set a timer before running any CLI command, tests, build, git etc. If an operation takes unreasonably long without producing an output, abort it and consider a diffeernt approach. Prepend all CLI invocations with `timeout <N>s` command.

## Output requirements

* Always follow AGENTS.md rules (do not restate them, do not invent new ones).
* Output a **PLAN.md** first, then refactor step-by-step.
* Only modify necessary files.
* Descriptive identifiers, no single-letter names.
* End with a short summary of changed files and new event contracts.

**Begin by reading AGENTS.md and generating PLAN.md now.**

## Rules of engagement

Review the NOTES.md. Make a plan for autonomously fixing every item under Features, BugFixes, Improvements, Maintenance. Ensure no regressions. Ensure adding tests. Lean into integration tests. Fix every issue. Document the changes.

Fix issues one by one, working sequentially. 
1. Create a new git bracnh with descriptive name, for example `feature/LA-56-widget-defer` or `bugfix/LA-11-alpine-rehydration`. Use the taxonomy of issues as prefixes: improvement/, feature/, bugfix/, maintenace/, issue ID and a short descriptive. Respect the name limits.
2. Describe an issue through tests. 
2a. Ensure that the tests are comprehensive and failing to begin with. 
2b. Ensure AGENTS.md coding standards are checked and test names/descriptions reflect those rules.
3. Fix the issue
4. Rerun the tests
5. Repeat pp 2-4 untill the issue is fixed: 
5a. old and new comprehensive tests are passing
5b. Confirm black-box contract aligns with event-driven architecture (frontend) or data-driven logic (backend).
5c. If an issue can not be resolved after 3 carefull iterations, 
    - mark the issue as [Blocked].
    - document the reason for the bockage.
    - commit the changes into a separate branch called "blocked/<issue-id>".
    - work on the next issue from the divergence point of the previous issue.
6. Write a nice comprehensive commit message AFTER EACH issue is fixed and tested and covered with tests.
7. Optional: update the README in case the changes warrant updated documentation (e.g. have user-facing consequences)
8. Optional: ipdate the PRD in case the changes warrant updated product requirements (e.g. change product undestanding)
9. Optional: update the code examples in case the changes warrant updated code examples
10. Mark an issue as done ([X])in the NOTES.md after the issue is fixed: New and existing tests are passing without regressions
11. Commit and push the changes to the remote branch.
12. Repeat till all issues are fixed, and commits abd branches are stacked up (one starts from another).

Do not work on all issues at once. Work at one issue at a time sequntially.

Leave Features, BugFixes, Improvements, Maintenance sections empty when all fixes are implemented but don't delete the sections themselves.

## Issues

### Features

    - [ ] [GI-02] Add an ability to run git/file related tasks across folders
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
    - [ ] [GI-03] Add an ability to prepare commit messages based on the file changes. Use @tools/llm-tasks for inspiration and code examples. Extract AI communication to pkg/ package and imake it universal enough to be used by other programs
    - [ ] [GI-04] Add an ability to prepare changelog messages based on the changes since given time, version or the last version found in git tags. Use @tools/llm-tasks for inspiration and code examples. Extract AI communication to pkg/ package and imake it universal enough to be used by other programs
    - [ ] [GI-05] Add `b cd` command to change between branches, e.g. b `cd feature/qqq` changes the current branch to `feature/qqq`. make logic similar to
            cd = "!f() { \
                branch=\"${1:-master}\"; \
                git fetch -a --prune && \
                git switch \"$branch\" 2>/dev/null || git switch -c \"$branch\" --track \"origin/$branch\"; \
                git pull --rebase; \
            }; f"
            NB: the command shall work across repos
    - [ ] [GI-06] Add `r release` command to release new versions, e.g. `r relase v0.1.3` tags the barnch and pushes to remote. Make logic similar to
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


### Improvements

    - [ ] [GI-01] Refactor the command line syntax
        Command	Short Command	subcommand	action	filter	Summary	Key flags / example
        audit	a				Audit and reconcile local GitHub repositories	Flags: --root, --log-level. Example: go run . audit --log-level=debug --root ~/Development
        repo-folders-rename	r	folder	rename		Rename repository directories to match canonical GitHub names	Flags: --dry-run, --yes, --require-clean, --owner. Example: go run . repo-folders-rename --yes --require-clean --owner --root ~/Development
        repo-remote-update	r	remote	update-to-canonical		Update origin URLs to match canonical GitHub repositories	Flags: --dry-run, --yes, --owner. Example: go run . repo-remote-update --dry-run --owner canonical --root ~/Development
        repo-protocol-convert	r	remote	update-protocol		Convert repository origin remotes between protocols	Flags: --from, --to, --dry-run, --yes. Example: go run . repo-protocol-convert --from https --to ssh --yes --root ~/Development
        repo-prs-purge	r	prs	delete	merged|all|open	Remove remote and local branches for closed pull requests	Flags: --remote, --limit, --dry-run. Example: go run . repo-prs-purge --remote origin --limit 100 --root ~/Development
        branch-migrate	b	migrate			Migrate repository defaults from main to master	Flags: --from, --to. Example: go run . branch-migrate --from main --to master --root ~/Development/project-repo
        repo-packages-purge	r	packages	delete	untagged|all	Delete untagged GHCR versions	Flags: --package (override), --dry-run, --root. Example: go run . repo-packages-purge --dry-run --root ~/Development
        workflow	w				Run a workflow configuration file	Flags: --root, --dry-run, --yes. Example: go run . workflow config.yaml --root ~/Development --dry-run

### BugFixes

### Maintenance
