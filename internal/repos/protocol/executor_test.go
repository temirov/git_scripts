package protocol_test

import (
        "bytes"
        "context"
        "testing"

        "github.com/stretchr/testify/require"

        "github.com/temirov/git_scripts/internal/repos/protocol"
        "github.com/temirov/git_scripts/internal/repos/shared"
)

type stubGitManager struct {
        currentURL string
        setURLs    []string
        getError   error
        setError   error
}

func (manager *stubGitManager) CheckCleanWorktree(ctx context.Context, repositoryPath string) (bool, error) {
        return true, nil
}

func (manager *stubGitManager) GetCurrentBranch(ctx context.Context, repositoryPath string) (string, error) {
        return "main", nil
}

func (manager *stubGitManager) GetRemoteURL(ctx context.Context, repositoryPath string, remoteName string) (string, error) {
        if manager.getError != nil {
                return "", manager.getError
        }
        return manager.currentURL, nil
}

func (manager *stubGitManager) SetRemoteURL(ctx context.Context, repositoryPath string, remoteName string, remoteURL string) error {
        if manager.setError != nil {
                return manager.setError
        }
        manager.setURLs = append(manager.setURLs, remoteURL)
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
                options         protocol.Options
                gitManager      *stubGitManager
                prompter        shared.ConfirmationPrompter
                expectedOutput  string
                expectedErrors  string
                expectedUpdates int
        }{
                {
                        name: "owner_repo_missing",
                        options: protocol.Options{
                                RepositoryPath:        "/tmp/project",
                                OriginOwnerRepository: "",
                                CanonicalOwnerRepository: "",
                                CurrentProtocol:       shared.RemoteProtocolHTTPS,
                                TargetProtocol:        shared.RemoteProtocolSSH,
                        },
                        gitManager:     &stubGitManager{currentURL: "https://github.com/origin/example.git"},
                        expectedErrors: "ERROR: cannot derive owner/repo for protocol conversion in /tmp/project\n",
                },
                {
                        name: "dry_run_plan",
                        options: protocol.Options{
                                RepositoryPath:           "/tmp/project",
                                OriginOwnerRepository:    "origin/example",
                                CanonicalOwnerRepository: "canonical/example",
                                CurrentProtocol:          shared.RemoteProtocolHTTPS,
                                TargetProtocol:           shared.RemoteProtocolSSH,
                                DryRun:                   true,
                        },
                        gitManager: &stubGitManager{currentURL: "https://github.com/origin/example.git"},
                        expectedOutput: "PLAN-CONVERT: /tmp/project origin https://github.com/origin/example.git → ssh://git@github.com/canonical/example.git\n",
                },
                {
                        name: "prompter_declines",
                        options: protocol.Options{
                                RepositoryPath:           "/tmp/project",
                                OriginOwnerRepository:    "origin/example",
                                CanonicalOwnerRepository: "canonical/example",
                                CurrentProtocol:          shared.RemoteProtocolHTTPS,
                                TargetProtocol:           shared.RemoteProtocolSSH,
                        },
                        gitManager:     &stubGitManager{currentURL: "https://github.com/origin/example.git"},
                        prompter:       stubPrompter{response: false},
                        expectedOutput: "CONVERT-SKIP: user declined for /tmp/project\n",
                },
                {
                        name: "update_success",
                        options: protocol.Options{
                                RepositoryPath:           "/tmp/project",
                                OriginOwnerRepository:    "origin/example",
                                CanonicalOwnerRepository: "canonical/example",
                                CurrentProtocol:          shared.RemoteProtocolHTTPS,
                                TargetProtocol:           shared.RemoteProtocolSSH,
                                AssumeYes:                true,
                        },
                        gitManager:     &stubGitManager{currentURL: "https://github.com/origin/example.git"},
                        expectedOutput: "CONVERT-DONE: /tmp/project origin now ssh://git@github.com/canonical/example.git\n",
                        expectedUpdates: 1,
                },
        }

        for _, testCase := range testCases {
                testInstance.Run(testCase.name, func(testInstance *testing.T) {
                        outputBuffer := &bytes.Buffer{}
                        errorBuffer := &bytes.Buffer{}

                        executor := protocol.NewExecutor(protocol.Dependencies{
                                GitManager: testCase.gitManager,
                                Prompter:   testCase.prompter,
                                Output:     outputBuffer,
                                Errors:     errorBuffer,
                        })

                        executor.Execute(context.Background(), testCase.options)
                        require.Equal(testInstance, testCase.expectedOutput, outputBuffer.String())
                        require.Equal(testInstance, testCase.expectedErrors, errorBuffer.String())
                        require.Len(testInstance, testCase.gitManager.setURLs, testCase.expectedUpdates)
                })
        }
}
