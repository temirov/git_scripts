# ISSUES (Append-only Log)

Entries record newly discovered requests or changes, with their outcomes. No instructive content lives here. Read @NOTES.md for the process to follow when fixing issues.

## Features

    - [ ] [GX-02] Add an ability to run git/file related tasks across folders
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
    - [ ] [GX-03] Add an ability to prepare commit messages based on the file changes. Use @tools/llm-tasks for inspiration and code examples. Extract AI communication to pkg/ package and imake it universal enough to be used by other programs
    - [ ] [GX-04] Add an ability to prepare changelog messages based on the changes since given time, version or the last version found in git tags. Use @tools/llm-tasks for inspiration and code examples. Extract AI communication to pkg/ package and imake it universal enough to be used by other programs
    - [ ] [GX-05] Add `b cd` command to change between branches, e.g. `b cd feature/qqq` changes the current branch to `feature/qqq`. make logic similar to
            cd = "!f() { \
                branch=\"${1:-master}\"; \
                git fetch -a --prune && \
                git switch \"$branch\" 2>/dev/null || git switch -c \"$branch\" --track \"origin/$branch\"; \
                git pull --rebase; \
            }; f"
            NB: the command shall work across repos
    - [ ] [GX-06] Add `r release` command to release new versions, e.g. `r relase v0.1.3` tags the barnch and pushes to remote. Make logic similar to
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


## Improvements

    - [ ] [GX-01] Refactor the command line syntax
        Command	Short Command	subcommand	action	filter	Summary	Key flags / example
        audit	a				Audit and reconcile local GitHub repositories	Flags: --root, --log-level. Example: go run . audit --log-level=debug --root ~/Development
        repo-folders-rename	r	folder	rename		Rename repository directories to match canonical GitHub names	Flags: --dry-run, --yes, --require-clean, --owner. Example: go run . repo-folders-rename --yes --require-clean --owner --root ~/Development
        repo-remote-update	r	remote	update-to-canonical		Update origin URLs to match canonical GitHub repositories	Flags: --dry-run, --yes, --owner. Example: go run . repo-remote-update --dry-run --owner canonical --root ~/Development
        repo-protocol-convert	r	remote	update-protocol		Convert repository origin remotes between protocols	Flags: --from, --to, --dry-run, --yes. Example: go run . repo-protocol-convert --from https --to ssh --yes --root ~/Development
        repo-prs-purge	r	prs	delete	merged|all|open	Remove remote and local branches for closed pull requests	Flags: --remote, --limit, --dry-run. Example: go run . repo-prs-purge --remote origin --limit 100 --root ~/Development
        branch-migrate	b	migrate			Migrate repository defaults from main to master	Flags: --from, --to. Example: go run . branch-migrate --from main --to master --root ~/Development/project-repo
        repo-packages-purge	r	packages	delete	untagged|all	Delete untagged GHCR versions	Flags: --package (override), --dry-run, --root. Example: go run . repo-packages-purge --dry-run --root ~/Development
        workflow	w				Run a workflow configuration file	Flags: --root, --dry-run, --yes. Example: go run . workflow config.yaml --root ~/Development --dry-run

## BugFixes

## Maintenance

## Planning 
do not work on the issues below, not ready
