package remotes_test

import (
        "bytes"
        "context"
        "testing"

        "github.com/stretchr/testify/require"

        "github.com/temirov/git_scripts/internal/repos/remotes"
        "github.com/temirov/git_scripts/internal/repos/shared"
)

type stubGitManager struct {
        urlsSet    []string
        setError   error
}

func (manager *stubGitManager) CheckCleanWorktree(ctx context.Context, repositoryPath string) (bool, error) {
        return true, nil
}

func (manager *stubGitManager) GetCurrentBranch(ctx context.Context, repositoryPath string) (string, error) {
        return "main", nil
}

func (manager *stubGitManager) GetRemoteURL(ctx context.Context, repositoryPath string, remoteName string) (string, error) {
        return "", nil
}

func (manager *stubGitManager) SetRemoteURL(ctx context.Context, repositoryPath string, remoteName string, remoteURL string) error {
        if manager.setError != nil {
                return manager.setError
        }
        manager.urlsSet = append(manager.urlsSet, remoteURL)
        return nil
}

type stubPrompter struct {
        response bool
        err      error
}

func (prompter stubPrompter) Confirm(prompt string) (bool, error) {
        if prompter.err != nil {
                return false, prompter.err
        }
        return prompter.response, nil
}

func TestExecutorBehaviors(testInstance *testing.T) {
        testCases := []struct {
                name            string
                options         remotes.Options
                gitManager      *stubGitManager
                prompter        shared.ConfirmationPrompter
                expectedOutput  string
                expectedUpdates int
        }{
                {
                        name: "skip_missing_origin",
                        options: remotes.Options{
                                RepositoryPath:        "/tmp/project",
                                OriginOwnerRepository: "",
                                CanonicalOwnerRepository: "canonical/example",
                                RemoteProtocol:        shared.RemoteProtocolHTTPS,
                        },
                        gitManager:     &stubGitManager{},
                        expectedOutput:  "UPDATE-REMOTE-SKIP: /tmp/project (error: could not parse origin owner/repo)\n",
                        expectedUpdates: 0,
                },
                {
                        name: "skip_canonical_missing",
                        options: remotes.Options{
                                RepositoryPath:        "/tmp/project",
                                OriginOwnerRepository: "origin/example",
                                CanonicalOwnerRepository: "",
                                RemoteProtocol:        shared.RemoteProtocolHTTPS,
                        },
                        gitManager:     &stubGitManager{},
                        expectedOutput:  "UPDATE-REMOTE-SKIP: /tmp/project (no upstream: no canonical redirect found)\n",
                        expectedUpdates: 0,
                },
                {
                        name: "dry_run_plan",
                        options: remotes.Options{
                                RepositoryPath:            "/tmp/project",
                                CurrentOriginURL:          "https://github.com/origin/example.git",
                                OriginOwnerRepository:     "origin/example",
                                CanonicalOwnerRepository:  "canonical/example",
                                RemoteProtocol:            shared.RemoteProtocolHTTPS,
                                DryRun:                    true,
                        },
                        gitManager:     &stubGitManager{},
                        expectedOutput:  "PLAN-UPDATE-REMOTE: /tmp/project origin https://github.com/origin/example.git → https://github.com/canonical/example.git\n",
                        expectedUpdates: 0,
                },
                {
                        name: "prompter_declines",
                        options: remotes.Options{
                                RepositoryPath:            "/tmp/project",
                                CurrentOriginURL:          "https://github.com/origin/example.git",
                                OriginOwnerRepository:     "origin/example",
                                CanonicalOwnerRepository:  "canonical/example",
                                RemoteProtocol:            shared.RemoteProtocolHTTPS,
                        },
                        gitManager:     &stubGitManager{},
                        prompter:       stubPrompter{response: false},
                        expectedOutput:  "UPDATE-REMOTE-SKIP: user declined for /tmp/project\n",
                        expectedUpdates: 0,
                },
                {
                        name: "update_success",
                        options: remotes.Options{
                                RepositoryPath:            "/tmp/project",
                                CurrentOriginURL:          "https://github.com/origin/example.git",
                                OriginOwnerRepository:     "origin/example",
                                CanonicalOwnerRepository:  "canonical/example",
                                RemoteProtocol:            shared.RemoteProtocolHTTPS,
                                AssumeYes:                 true,
                        },
                        gitManager:     &stubGitManager{},
                        expectedOutput:  "UPDATE-REMOTE-DONE: /tmp/project origin now https://github.com/canonical/example.git\n",
                        expectedUpdates: 1,
                },
        }

        for _, testCase := range testCases {
                testInstance.Run(testCase.name, func(testInstance *testing.T) {
                        outputBuffer := &bytes.Buffer{}
                        executor := remotes.NewExecutor(remotes.Dependencies{
                                GitManager: testCase.gitManager,
                                Prompter:   testCase.prompter,
                                Output:     outputBuffer,
                        })

                        executor.Execute(context.Background(), testCase.options)
                        require.Equal(testInstance, testCase.expectedOutput, outputBuffer.String())
                        require.Len(testInstance, testCase.gitManager.urlsSet, testCase.expectedUpdates)
                })
        }
}
