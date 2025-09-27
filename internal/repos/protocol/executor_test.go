package protocol_test

import (
	"bytes"
	"context"
	"fmt"
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

const (
	protocolTestRepositoryPath     = "/tmp/project"
	protocolTestOriginOwnerRepo    = "origin/example"
	protocolTestCanonicalOwnerRepo = "canonical/example"
	protocolTestOriginURL          = "https://github.com/origin/example.git"
	protocolTestTargetURL          = "ssh://git@github.com/canonical/example.git"
	protocolTestOwnerRepoError     = "ERROR: cannot derive owner/repo for protocol conversion in %s\n"
	protocolTestPlanMessage        = "PLAN-CONVERT: %s origin %s → %s\n"
	protocolTestDeclinedMessage    = "CONVERT-SKIP: user declined for %s\n"
	protocolTestSuccessMessage     = "CONVERT-DONE: %s origin now %s\n"
)

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
				RepositoryPath:           protocolTestRepositoryPath,
				OriginOwnerRepository:    "",
				CanonicalOwnerRepository: "",
				CurrentProtocol:          shared.RemoteProtocolHTTPS,
				TargetProtocol:           shared.RemoteProtocolSSH,
			},
			gitManager:     &stubGitManager{currentURL: protocolTestOriginURL},
			expectedErrors: fmt.Sprintf(protocolTestOwnerRepoError, protocolTestRepositoryPath),
		},
		{
			name: "dry_run_plan",
			options: protocol.Options{
				RepositoryPath:           protocolTestRepositoryPath,
				OriginOwnerRepository:    protocolTestOriginOwnerRepo,
				CanonicalOwnerRepository: protocolTestCanonicalOwnerRepo,
				CurrentProtocol:          shared.RemoteProtocolHTTPS,
				TargetProtocol:           shared.RemoteProtocolSSH,
				DryRun:                   true,
			},
			gitManager:     &stubGitManager{currentURL: protocolTestOriginURL},
			expectedOutput: fmt.Sprintf(protocolTestPlanMessage, protocolTestRepositoryPath, protocolTestOriginURL, protocolTestTargetURL),
		},
		{
			name: "prompter_declines",
			options: protocol.Options{
				RepositoryPath:           protocolTestRepositoryPath,
				OriginOwnerRepository:    protocolTestOriginOwnerRepo,
				CanonicalOwnerRepository: protocolTestCanonicalOwnerRepo,
				CurrentProtocol:          shared.RemoteProtocolHTTPS,
				TargetProtocol:           shared.RemoteProtocolSSH,
			},
			gitManager:     &stubGitManager{currentURL: protocolTestOriginURL},
			prompter:       stubPrompter{response: false},
			expectedOutput: fmt.Sprintf(protocolTestDeclinedMessage, protocolTestRepositoryPath),
		},
		{
			name: "update_success",
			options: protocol.Options{
				RepositoryPath:           protocolTestRepositoryPath,
				OriginOwnerRepository:    protocolTestOriginOwnerRepo,
				CanonicalOwnerRepository: protocolTestCanonicalOwnerRepo,
				CurrentProtocol:          shared.RemoteProtocolHTTPS,
				TargetProtocol:           shared.RemoteProtocolSSH,
				AssumeYes:                true,
			},
			gitManager:      &stubGitManager{currentURL: protocolTestOriginURL},
			expectedOutput:  fmt.Sprintf(protocolTestSuccessMessage, protocolTestRepositoryPath, protocolTestTargetURL),
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
