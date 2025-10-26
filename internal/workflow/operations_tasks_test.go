package workflow

import (
	"context"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/temirov/gix/internal/audit"
	"github.com/temirov/gix/internal/execshell"
	"github.com/temirov/gix/internal/githubcli"
	"github.com/temirov/gix/internal/gitrepo"
)

func TestTaskPlannerBuildPlanRendersTemplates(testInstance *testing.T) {
	fileSystem := newFakeFileSystem(nil)
	environment := &Environment{FileSystem: fileSystem}

	inspection := audit.RepositoryInspection{
		Path:                "/repositories/sample",
		FinalOwnerRepo:      "octocat/sample",
		RemoteDefaultBranch: "main",
		LocalBranch:         "develop",
	}
	repository := NewRepositoryState(inspection)

	taskDefinition := TaskDefinition{
		Name:        "Add Docs",
		EnsureClean: true,
		Branch: TaskBranchDefinition{
			NameTemplate: "feature/{{ .Repository.Name }}/docs update",
		},
		Files: []TaskFileDefinition{{
			PathTemplate:    "docs/{{ .Repository.Name }}.md",
			ContentTemplate: "Repository: {{ .Repository.FullName }}",
			Mode:            taskFileModeOverwrite,
			Permissions:     defaultTaskFilePermissions,
		}},
		Commit: TaskCommitDefinition{
			MessageTemplate: " docs: update {{ .Task.Name }} ",
		},
	}

	templateData := buildTaskTemplateData(repository, taskDefinition)
	planner := newTaskPlanner(taskDefinition, templateData)

	plan, planError := planner.BuildPlan(environment, repository)
	require.NoError(testInstance, planError)

	require.False(testInstance, plan.skipped)
	require.Equal(testInstance, "feature-sample-docs-update", plan.branchName)
	require.Equal(testInstance, "main", plan.startPoint)
	require.Equal(testInstance, "docs: update Add Docs", plan.commitMessage)
	require.Len(testInstance, plan.fileChanges, 1)

	fileChange := plan.fileChanges[0]
	require.Equal(testInstance, "docs/sample.md", fileChange.relativePath)
	require.Equal(testInstance, filepath.Join(repository.Path, "docs/sample.md"), fileChange.absolutePath)
	require.True(testInstance, fileChange.apply)
	require.Equal(testInstance, []byte("Repository: octocat/sample"), fileChange.content)
	require.Equal(testInstance, defaultTaskFilePermissions, fileChange.permissions)
	require.Nil(testInstance, plan.pullRequest)
}

func TestTaskPlannerSkipWhenFileUnchanged(testInstance *testing.T) {
	repositoryPath := "/repositories/sample"
	existingContent := []byte("Repository: octocat/sample")
	fileSystem := newFakeFileSystem(map[string][]byte{
		filepath.Join(repositoryPath, "docs/sample.md"): existingContent,
	})
	environment := &Environment{FileSystem: fileSystem}

	inspection := audit.RepositoryInspection{
		Path:                repositoryPath,
		FinalOwnerRepo:      "octocat/sample",
		RemoteDefaultBranch: "main",
	}
	repository := NewRepositoryState(inspection)

	taskDefinition := TaskDefinition{
		Name: "Add Docs",
		Branch: TaskBranchDefinition{
			NameTemplate: "feature/{{ .Repository.Name }}",
		},
		Files: []TaskFileDefinition{{
			PathTemplate:    "docs/{{ .Repository.Name }}.md",
			ContentTemplate: "Repository: {{ .Repository.FullName }}",
			Mode:            taskFileModeOverwrite,
			Permissions:     defaultTaskFilePermissions,
		}},
		Commit: TaskCommitDefinition{},
	}

	templateData := buildTaskTemplateData(repository, taskDefinition)
	planner := newTaskPlanner(taskDefinition, templateData)

	plan, planError := planner.BuildPlan(environment, repository)
	require.NoError(testInstance, planError)

	require.True(testInstance, plan.skipped)
	require.Equal(testInstance, "no changes", plan.skipReason)
	require.Len(testInstance, plan.fileChanges, 1)
	require.False(testInstance, plan.fileChanges[0].apply)
	require.Equal(testInstance, "unchanged", plan.fileChanges[0].skipReason)
}

func TestTaskExecutorSkipsWhenBranchExists(testInstance *testing.T) {
	gitExecutor := &recordingGitExecutor{
		branchExists:  true,
		worktreeClean: true,
		currentBranch: "master",
	}
	fileSystem := newFakeFileSystem(nil)

	repositoryManager, managerError := gitrepo.NewRepositoryManager(gitExecutor)
	require.NoError(testInstance, managerError)

	githubClient, clientError := githubcli.NewClient(gitExecutor)
	require.NoError(testInstance, clientError)

	repository := NewRepositoryState(audit.RepositoryInspection{
		Path:                "/repositories/sample",
		FinalOwnerRepo:      "octocat/sample",
		RemoteDefaultBranch: "main",
	})

	taskDefinition := TaskDefinition{
		Name: "Add Docs",
		Files: []TaskFileDefinition{{
			PathTemplate:    "docs/{{ .Repository.Name }}.md",
			ContentTemplate: "Repository: {{ .Repository.FullName }}",
			Mode:            taskFileModeOverwrite,
			Permissions:     defaultTaskFilePermissions,
		}},
		Commit: TaskCommitDefinition{},
	}

	templateData := buildTaskTemplateData(repository, taskDefinition)
	planner := newTaskPlanner(taskDefinition, templateData)

	environment := &Environment{
		GitExecutor:       gitExecutor,
		RepositoryManager: repositoryManager,
		GitHubClient:      githubClient,
		FileSystem:        fileSystem,
	}

	plan, planError := planner.BuildPlan(environment, repository)
	require.NoError(testInstance, planError)

	executor := newTaskExecutor(environment, repository, plan)

	executionError := executor.Execute(context.Background())
	require.NoError(testInstance, executionError)

	require.Len(testInstance, fileSystem.files, 0)
	for commandIndex := range gitExecutor.commands {
		command := gitExecutor.commands[commandIndex].Arguments
		require.NotEqual(testInstance, "checkout", firstArgument(command))
		require.NotEqual(testInstance, "add", firstArgument(command))
		require.NotEqual(testInstance, "commit", firstArgument(command))
		require.NotEqual(testInstance, "push", firstArgument(command))
	}
}

func TestTaskExecutorAppliesChanges(testInstance *testing.T) {
	gitExecutor := &recordingGitExecutor{
		worktreeClean: true,
		currentBranch: "master",
	}
	fileSystem := newFakeFileSystem(nil)

	repositoryManager, managerError := gitrepo.NewRepositoryManager(gitExecutor)
	require.NoError(testInstance, managerError)

	githubClient, clientError := githubcli.NewClient(gitExecutor)
	require.NoError(testInstance, clientError)

	repository := NewRepositoryState(audit.RepositoryInspection{
		Path:                "/repositories/sample",
		FinalOwnerRepo:      "octocat/sample",
		RemoteDefaultBranch: "main",
		LocalBranch:         "master",
	})

	taskDefinition := TaskDefinition{
		Name:        "Add Docs",
		EnsureClean: true,
		Branch: TaskBranchDefinition{
			NameTemplate: "feature/{{ .Repository.Name }}-docs",
			PushRemote:   defaultTaskPushRemote,
		},
		Files: []TaskFileDefinition{{
			PathTemplate:    "docs/{{ .Repository.Name }}.md",
			ContentTemplate: "Repository: {{ .Repository.FullName }}",
			Mode:            taskFileModeOverwrite,
			Permissions:     defaultTaskFilePermissions,
		}},
		Commit: TaskCommitDefinition{
			MessageTemplate: "docs: update {{ .Task.Name }}",
		},
	}

	templateData := buildTaskTemplateData(repository, taskDefinition)
	planner := newTaskPlanner(taskDefinition, templateData)

	environment := &Environment{
		GitExecutor:       gitExecutor,
		RepositoryManager: repositoryManager,
		GitHubClient:      githubClient,
		FileSystem:        fileSystem,
	}

	plan, planError := planner.BuildPlan(environment, repository)
	require.NoError(testInstance, planError)
	require.False(testInstance, plan.skipped)

	executor := newTaskExecutor(environment, repository, plan)

	executionError := executor.Execute(context.Background())
	require.NoError(testInstance, executionError)

	expectedPath := filepath.Join(repository.Path, "docs/sample.md")
	require.Equal(testInstance, []byte("Repository: octocat/sample"), fileSystem.files[expectedPath])

	expectedCommands := [][]string{
		{"status", "--porcelain"},
		{"rev-parse", "--verify", "feature-sample-docs"},
		{"rev-parse", "--abbrev-ref", "HEAD"},
		{"checkout", "main"},
		{"checkout", "-B", "feature-sample-docs", "main"},
		{"add", "docs/sample.md"},
		{"commit", "-m", "docs: update Add Docs"},
		{"push", "--set-upstream", "origin", "feature-sample-docs"},
		{"checkout", "master"},
	}

	collected := make([][]string, 0, len(gitExecutor.commands))
	for commandIndex := range gitExecutor.commands {
		collected = append(collected, gitExecutor.commands[commandIndex].Arguments)
	}
	require.Equal(testInstance, expectedCommands, collected)
}

type fakeFileSystem struct {
	files map[string][]byte
}

func newFakeFileSystem(initial map[string][]byte) *fakeFileSystem {
	files := map[string][]byte{}
	for path, data := range initial {
		files[path] = append([]byte(nil), data...)
	}
	return &fakeFileSystem{files: files}
}

func (system *fakeFileSystem) Stat(path string) (fs.FileInfo, error) {
	data, exists := system.files[path]
	if !exists {
		return nil, fs.ErrNotExist
	}
	return fakeFileInfo{name: filepath.Base(path), size: int64(len(data))}, nil
}

func (system *fakeFileSystem) Rename(oldPath string, newPath string) error {
	data, exists := system.files[oldPath]
	if !exists {
		return fs.ErrNotExist
	}
	system.files[newPath] = append([]byte(nil), data...)
	delete(system.files, oldPath)
	return nil
}

func (system *fakeFileSystem) Abs(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	return filepath.Abs(path)
}

func (system *fakeFileSystem) MkdirAll(path string, permissions fs.FileMode) error {
	return nil
}

func (system *fakeFileSystem) ReadFile(path string) ([]byte, error) {
	data, exists := system.files[path]
	if !exists {
		return nil, fs.ErrNotExist
	}
	return append([]byte(nil), data...), nil
}

func (system *fakeFileSystem) WriteFile(path string, data []byte, permissions fs.FileMode) error {
	system.files[path] = append([]byte(nil), data...)
	return nil
}

type fakeFileInfo struct {
	name string
	size int64
}

func (info fakeFileInfo) Name() string      { return info.name }
func (info fakeFileInfo) Size() int64       { return info.size }
func (info fakeFileInfo) Mode() fs.FileMode { return 0 }
func (info fakeFileInfo) ModTime() time.Time {
	return time.Time{}
}
func (info fakeFileInfo) IsDir() bool { return false }
func (info fakeFileInfo) Sys() any    { return nil }

type recordingGitExecutor struct {
	commands       []execshell.CommandDetails
	githubCommands []execshell.CommandDetails
	branchExists   bool
	worktreeClean  bool
	currentBranch  string
}

func (executor *recordingGitExecutor) ExecuteGit(_ context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error) {
	executor.commands = append(executor.commands, details)
	if len(details.Arguments) == 0 {
		return execshell.ExecutionResult{}, nil
	}

	switch details.Arguments[0] {
	case "status":
		if executor.worktreeClean {
			return execshell.ExecutionResult{StandardOutput: ""}, nil
		}
		return execshell.ExecutionResult{StandardOutput: " M file.txt"}, nil
	case "rev-parse":
		if len(details.Arguments) >= 2 {
			switch details.Arguments[1] {
			case "--verify":
				if executor.branchExists {
					return execshell.ExecutionResult{}, nil
				}
				return execshell.ExecutionResult{}, execshell.CommandFailedError{
					Command: execshell.ShellCommand{Name: execshell.CommandGit, Details: details},
					Result:  execshell.ExecutionResult{ExitCode: 1},
				}
			case "--abbrev-ref":
				branch := executor.currentBranch
				if len(branch) == 0 {
					branch = "master"
				}
				return execshell.ExecutionResult{StandardOutput: branch}, nil
			}
		}
	}

	return execshell.ExecutionResult{}, nil
}

func (executor *recordingGitExecutor) ExecuteGitHubCLI(_ context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error) {
	executor.githubCommands = append(executor.githubCommands, details)
	return execshell.ExecutionResult{}, nil
}

func firstArgument(arguments []string) string {
	if len(arguments) == 0 {
		return ""
	}
	return arguments[0]
}
