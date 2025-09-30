package remotes_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/temirov/gix/internal/repos/remotes"
	"github.com/temirov/gix/internal/repos/shared"
)

type stubGitManager struct {
	urlsSet  []string
	setError error
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
	result shared.ConfirmationResult
	err    error
}

func (prompter stubPrompter) Confirm(prompt string) (shared.ConfirmationResult, error) {
	if prompter.err != nil {
		return shared.ConfirmationResult{}, prompter.err
	}
	return prompter.result, nil
}

const (
	remotesTestRepositoryPath          = "/tmp/project"
	remotesTestCurrentOriginURL        = "https://github.com/origin/example.git"
	remotesTestOriginOwnerRepository   = "origin/example"
	remotesTestCanonicalOwnerRepo      = "canonical/example"
	remotesTestCanonicalURL            = "https://github.com/canonical/example.git"
	remotesTestSkippedOriginMessage    = "UPDATE-REMOTE-SKIP: %s (error: could not parse origin owner/repo)\n"
	remotesTestSkippedCanonicalMessage = "UPDATE-REMOTE-SKIP: %s (no upstream: no canonical redirect found)\n"
	remotesTestPlanMessage             = "PLAN-UPDATE-REMOTE: %s origin %s → %s\n"
	remotesTestDeclinedMessage         = "UPDATE-REMOTE-SKIP: user declined for %s\n"
	remotesTestPromptErrorMessage      = "UPDATE-REMOTE-SKIP: %s (error: could not construct target URL)\n"
	remotesTestSuccessMessage          = "UPDATE-REMOTE-DONE: %s origin now %s\n"
)

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
				RepositoryPath:           remotesTestRepositoryPath,
				OriginOwnerRepository:    "",
				CanonicalOwnerRepository: remotesTestCanonicalOwnerRepo,
				RemoteProtocol:           shared.RemoteProtocolHTTPS,
			},
			gitManager:      &stubGitManager{},
			expectedOutput:  fmt.Sprintf(remotesTestSkippedOriginMessage, remotesTestRepositoryPath),
			expectedUpdates: 0,
		},
		{
			name: "skip_canonical_missing",
			options: remotes.Options{
				RepositoryPath:           remotesTestRepositoryPath,
				OriginOwnerRepository:    remotesTestOriginOwnerRepository,
				CanonicalOwnerRepository: "",
				RemoteProtocol:           shared.RemoteProtocolHTTPS,
			},
			gitManager:      &stubGitManager{},
			expectedOutput:  fmt.Sprintf(remotesTestSkippedCanonicalMessage, remotesTestRepositoryPath),
			expectedUpdates: 0,
		},
		{
			name: "dry_run_plan",
			options: remotes.Options{
				RepositoryPath:           remotesTestRepositoryPath,
				CurrentOriginURL:         remotesTestCurrentOriginURL,
				OriginOwnerRepository:    remotesTestOriginOwnerRepository,
				CanonicalOwnerRepository: remotesTestCanonicalOwnerRepo,
				RemoteProtocol:           shared.RemoteProtocolHTTPS,
				DryRun:                   true,
			},
			gitManager:      &stubGitManager{},
			expectedOutput:  fmt.Sprintf(remotesTestPlanMessage, remotesTestRepositoryPath, remotesTestCurrentOriginURL, remotesTestCanonicalURL),
			expectedUpdates: 0,
		},
		{
			name: "prompter_declines",
			options: remotes.Options{
				RepositoryPath:           remotesTestRepositoryPath,
				CurrentOriginURL:         remotesTestCurrentOriginURL,
				OriginOwnerRepository:    remotesTestOriginOwnerRepository,
				CanonicalOwnerRepository: remotesTestCanonicalOwnerRepo,
				RemoteProtocol:           shared.RemoteProtocolHTTPS,
			},
			gitManager:      &stubGitManager{},
			prompter:        stubPrompter{result: shared.ConfirmationResult{Confirmed: false}},
			expectedOutput:  fmt.Sprintf(remotesTestDeclinedMessage, remotesTestRepositoryPath),
			expectedUpdates: 0,
		},
		{
			name: "prompter_accepts_once",
			options: remotes.Options{
				RepositoryPath:           remotesTestRepositoryPath,
				CurrentOriginURL:         remotesTestCurrentOriginURL,
				OriginOwnerRepository:    remotesTestOriginOwnerRepository,
				CanonicalOwnerRepository: remotesTestCanonicalOwnerRepo,
				RemoteProtocol:           shared.RemoteProtocolHTTPS,
			},
			gitManager:      &stubGitManager{},
			prompter:        stubPrompter{result: shared.ConfirmationResult{Confirmed: true}},
			expectedOutput:  fmt.Sprintf(remotesTestSuccessMessage, remotesTestRepositoryPath, remotesTestCanonicalURL),
			expectedUpdates: 1,
		},
		{
			name: "prompter_accepts_all",
			options: remotes.Options{
				RepositoryPath:           remotesTestRepositoryPath,
				CurrentOriginURL:         remotesTestCurrentOriginURL,
				OriginOwnerRepository:    remotesTestOriginOwnerRepository,
				CanonicalOwnerRepository: remotesTestCanonicalOwnerRepo,
				RemoteProtocol:           shared.RemoteProtocolHTTPS,
			},
			gitManager:      &stubGitManager{},
			prompter:        stubPrompter{result: shared.ConfirmationResult{Confirmed: true, ApplyToAll: true}},
			expectedOutput:  fmt.Sprintf(remotesTestSuccessMessage, remotesTestRepositoryPath, remotesTestCanonicalURL),
			expectedUpdates: 1,
		},
		{
			name: "prompter_error",
			options: remotes.Options{
				RepositoryPath:           remotesTestRepositoryPath,
				CurrentOriginURL:         remotesTestCurrentOriginURL,
				OriginOwnerRepository:    remotesTestOriginOwnerRepository,
				CanonicalOwnerRepository: remotesTestCanonicalOwnerRepo,
				RemoteProtocol:           shared.RemoteProtocolHTTPS,
			},
			gitManager:      &stubGitManager{},
			prompter:        stubPrompter{err: fmt.Errorf("prompt failed")},
			expectedOutput:  fmt.Sprintf(remotesTestPromptErrorMessage, remotesTestRepositoryPath),
			expectedUpdates: 0,
		},
		{
			name: "assume_yes_updates_without_prompt",
			options: remotes.Options{
				RepositoryPath:           remotesTestRepositoryPath,
				CurrentOriginURL:         remotesTestCurrentOriginURL,
				OriginOwnerRepository:    remotesTestOriginOwnerRepository,
				CanonicalOwnerRepository: remotesTestCanonicalOwnerRepo,
				RemoteProtocol:           shared.RemoteProtocolHTTPS,
				AssumeYes:                true,
			},
			gitManager:      &stubGitManager{},
			expectedOutput:  fmt.Sprintf(remotesTestSuccessMessage, remotesTestRepositoryPath, remotesTestCanonicalURL),
			expectedUpdates: 1,
		},
	}

	for _, testCase := range testCases {
		testInstance.Run(testCase.name, func(testingInstance *testing.T) {
			outputBuffer := &bytes.Buffer{}
			executor := remotes.NewExecutor(remotes.Dependencies{
				GitManager: testCase.gitManager,
				Prompter:   testCase.prompter,
				Output:     outputBuffer,
			})

			executor.Execute(context.Background(), testCase.options)
			require.Equal(testingInstance, testCase.expectedOutput, outputBuffer.String())
			require.Len(testingInstance, testCase.gitManager.urlsSet, testCase.expectedUpdates)
		})
	}
}
