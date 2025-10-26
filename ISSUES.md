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
    - [ ] [GX-09] Improve the Command catalog in the @README.md. Reflect the current catalog of commands. Do not include any reference to the past and what the command used to be called. Users are the intended audience


## BugFixes

    - [ ] [GX-08] The required argument is missing in the help. I was expecting the help screen to be `gix repo release <tag> [flags]` and an explanation and an example of the tag.
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
    Same required argument with description and examples shall be in all commands that would require such argument. Analyze all command if there is a similar scenario and the arguments are missing in the help, and fix them.

## Maintenance

## Planning 
do not work on the issues below, not ready
