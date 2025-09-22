package githubcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/temirov/git_scripts/internal/execshell"
)

const (
	repoSubcommandConstant                  = "repo"
	viewSubcommandConstant                  = "view"
	pullRequestSubcommandConstant           = "pr"
	listSubcommandConstant                  = "list"
	apiSubcommandConstant                   = "api"
	jsonFlagConstant                        = "--json"
	repoFlagConstant                        = "--repo"
	stateFlagConstant                       = "--state"
	baseFlagConstant                        = "--base"
	limitFlagConstant                       = "--limit"
	methodFlagConstant                      = "-X"
	inputFlagConstant                       = "--input"
	stdinReferenceConstant                  = "-"
	acceptHeaderFlagConstant                = "-H"
	acceptHeaderValueConstant               = "Accept: application/vnd.github+json"
	repositoryFieldNameConstant             = "repository"
	baseBranchFieldNameConstant             = "base_branch"
	sourceBranchFieldNameConstant           = "source_branch"
	stateFieldNameConstant                  = "state"
	requiredValueMessageConstant            = "value required"
	executorNotConfiguredMessageConstant    = "github cli executor not configured"
	pullRequestLimitDefaultValueConstant    = 100
	pullRequestJSONFieldsConstant           = "number,title,headRefName"
	repoViewJSONFieldsConstant              = "defaultBranchRef,nameWithOwner,description"
	operationErrorMessageTemplateConstant   = "%s operation failed"
	operationErrorWithCauseTemplateConstant = "%s operation failed: %s"
	responseDecodingErrorTemplateConstant   = "%s response decoding failed: %s"
	payloadEncodingErrorTemplateConstant    = "%s payload encoding failed: %s"
	invalidInputErrorTemplateConstant       = "%s: %s"
	pagesEndpointTemplateConstant           = "repos/%s/pages"
	repositoryMetadataOperationNameConstant = OperationName("ResolveRepoMetadata")
	listPullRequestsOperationNameConstant   = OperationName("ListPullRequests")
	updatePagesOperationNameConstant        = OperationName("UpdatePagesConfig")
)

// OperationName describes a named GitHub CLI workflow supported by the client.
type OperationName string

// PullRequestState describes acceptable GitHub pull request states.
type PullRequestState string

// Pull request state enumerations.
const (
	PullRequestStateOpen   PullRequestState = PullRequestState("open")
	PullRequestStateClosed PullRequestState = PullRequestState("closed")
	PullRequestStateMerged PullRequestState = PullRequestState("merged")
)

// RepositoryMetadata contains key details resolved from GitHub.
type RepositoryMetadata struct {
	NameWithOwner string
	Description   string
	DefaultBranch string
}

// PullRequest represents minimal PR details returned by GitHub CLI.
type PullRequest struct {
	Number      int
	Title       string
	HeadRefName string
}

// PullRequestListOptions configures ListPullRequests queries.
type PullRequestListOptions struct {
	State       PullRequestState
	BaseBranch  string
	ResultLimit int
}

// PagesConfiguration describes the desired GitHub Pages configuration.
type PagesConfiguration struct {
	SourceBranch string
	SourcePath   string
}

// GitHubCommandExecutor is the minimal interface required from execshell.ShellExecutor.
type GitHubCommandExecutor interface {
	ExecuteGitHubCLI(executionContext context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error)
}

// Client coordinates GitHub CLI invocations through execshell.
type Client struct {
	executor GitHubCommandExecutor
}

var (
	// ErrExecutorNotConfigured indicates the client was constructed without an executor.
	ErrExecutorNotConfigured = errors.New(executorNotConfiguredMessageConstant)
)

// InvalidInputError surfaces validation issues for operation inputs.
type InvalidInputError struct {
	FieldName string
	Message   string
}

// Error describes the invalid input.
func (inputError InvalidInputError) Error() string {
	return fmt.Sprintf(invalidInputErrorTemplateConstant, inputError.FieldName, inputError.Message)
}

// OperationError wraps execution issues for GitHub CLI operations.
type OperationError struct {
	Operation OperationName
	Cause     error
}

// Error describes the operation failure.
func (operationError OperationError) Error() string {
	if operationError.Cause == nil {
		return fmt.Sprintf(operationErrorMessageTemplateConstant, operationError.Operation)
	}
	return fmt.Sprintf(operationErrorWithCauseTemplateConstant, operationError.Operation, operationError.Cause)
}

// Unwrap exposes the underlying cause.
func (operationError OperationError) Unwrap() error {
	return operationError.Cause
}

// ResponseDecodingError indicates JSON decoding failures.
type ResponseDecodingError struct {
	Operation OperationName
	Cause     error
}

// Error describes the decoding failure.
func (decodingError ResponseDecodingError) Error() string {
	return fmt.Sprintf(responseDecodingErrorTemplateConstant, decodingError.Operation, decodingError.Cause)
}

// Unwrap exposes the underlying JSON error.
func (decodingError ResponseDecodingError) Unwrap() error {
	return decodingError.Cause
}

// PayloadEncodingError indicates JSON encoding issues.
type PayloadEncodingError struct {
	Operation OperationName
	Cause     error
}

// Error describes the encoding failure.
func (encodingError PayloadEncodingError) Error() string {
	return fmt.Sprintf(payloadEncodingErrorTemplateConstant, encodingError.Operation, encodingError.Cause)
}

// Unwrap exposes the underlying error.
func (encodingError PayloadEncodingError) Unwrap() error {
	return encodingError.Cause
}

// NewClient constructs a GitHub CLI client.
func NewClient(executor GitHubCommandExecutor) (*Client, error) {
	if executor == nil {
		return nil, ErrExecutorNotConfigured
	}
	return &Client{executor: executor}, nil
}

// ResolveRepoMetadata retrieves canonical metadata for a repository using gh repo view.
func (client *Client) ResolveRepoMetadata(executionContext context.Context, repository string) (RepositoryMetadata, error) {
	repositoryIdentifier := strings.TrimSpace(repository)
	if len(repositoryIdentifier) == 0 {
		return RepositoryMetadata{}, InvalidInputError{FieldName: repositoryFieldNameConstant, Message: requiredValueMessageConstant}
	}

	commandDetails := execshell.CommandDetails{
		Arguments: []string{
			repoSubcommandConstant,
			viewSubcommandConstant,
			repositoryIdentifier,
			jsonFlagConstant,
			repoViewJSONFieldsConstant,
		},
	}

	executionResult, executionError := client.executor.ExecuteGitHubCLI(executionContext, commandDetails)
	if executionError != nil {
		return RepositoryMetadata{}, OperationError{Operation: repositoryMetadataOperationNameConstant, Cause: executionError}
	}

	var response struct {
		NameWithOwner    string `json:"nameWithOwner"`
		Description      string `json:"description"`
		DefaultBranchRef struct {
			Name string `json:"name"`
		} `json:"defaultBranchRef"`
	}

	decodingError := json.Unmarshal([]byte(executionResult.StandardOutput), &response)
	if decodingError != nil {
		return RepositoryMetadata{}, ResponseDecodingError{Operation: repositoryMetadataOperationNameConstant, Cause: decodingError}
	}

	return RepositoryMetadata{
		NameWithOwner: response.NameWithOwner,
		Description:   response.Description,
		DefaultBranch: response.DefaultBranchRef.Name,
	}, nil
}

// ListPullRequests enumerates pull requests using gh pr list.
func (client *Client) ListPullRequests(executionContext context.Context, repository string, options PullRequestListOptions) ([]PullRequest, error) {
	repositoryIdentifier := strings.TrimSpace(repository)
	if len(repositoryIdentifier) == 0 {
		return nil, InvalidInputError{FieldName: repositoryFieldNameConstant, Message: requiredValueMessageConstant}
	}

	if len(strings.TrimSpace(options.BaseBranch)) == 0 {
		return nil, InvalidInputError{FieldName: baseBranchFieldNameConstant, Message: requiredValueMessageConstant}
	}

	if len(options.State) == 0 {
		return nil, InvalidInputError{FieldName: stateFieldNameConstant, Message: requiredValueMessageConstant}
	}

	resultLimit := options.ResultLimit
	if resultLimit <= 0 {
		resultLimit = pullRequestLimitDefaultValueConstant
	}

	commandDetails := execshell.CommandDetails{
		Arguments: []string{
			pullRequestSubcommandConstant,
			listSubcommandConstant,
			repoFlagConstant,
			repositoryIdentifier,
			stateFlagConstant,
			string(options.State),
			baseFlagConstant,
			options.BaseBranch,
			jsonFlagConstant,
			pullRequestJSONFieldsConstant,
			limitFlagConstant,
			strconv.Itoa(resultLimit),
		},
	}

	executionResult, executionError := client.executor.ExecuteGitHubCLI(executionContext, commandDetails)
	if executionError != nil {
		return nil, OperationError{Operation: listPullRequestsOperationNameConstant, Cause: executionError}
	}

	var response []struct {
		Number      int    `json:"number"`
		Title       string `json:"title"`
		HeadRefName string `json:"headRefName"`
	}

	decodingError := json.Unmarshal([]byte(executionResult.StandardOutput), &response)
	if decodingError != nil {
		return nil, ResponseDecodingError{Operation: listPullRequestsOperationNameConstant, Cause: decodingError}
	}

	pullRequests := make([]PullRequest, 0, len(response))
	for _, pullRequestEntry := range response {
		pullRequests = append(pullRequests, PullRequest{
			Number:      pullRequestEntry.Number,
			Title:       pullRequestEntry.Title,
			HeadRefName: pullRequestEntry.HeadRefName,
		})
	}

	return pullRequests, nil
}

// UpdatePagesConfig updates the GitHub Pages configuration using gh api.
func (client *Client) UpdatePagesConfig(executionContext context.Context, repository string, configuration PagesConfiguration) error {
	repositoryIdentifier := strings.TrimSpace(repository)
	if len(repositoryIdentifier) == 0 {
		return InvalidInputError{FieldName: repositoryFieldNameConstant, Message: requiredValueMessageConstant}
	}

	if len(strings.TrimSpace(configuration.SourceBranch)) == 0 {
		return InvalidInputError{FieldName: sourceBranchFieldNameConstant, Message: requiredValueMessageConstant}
	}

	payload := struct {
		Source struct {
			Branch string `json:"branch"`
			Path   string `json:"path"`
		} `json:"source"`
	}{}

	payload.Source.Branch = configuration.SourceBranch
	payload.Source.Path = configuration.SourcePath

	payloadBytes, encodingError := json.Marshal(payload)
	if encodingError != nil {
		return PayloadEncodingError{Operation: updatePagesOperationNameConstant, Cause: encodingError}
	}

	commandDetails := execshell.CommandDetails{
		Arguments: []string{
			apiSubcommandConstant,
			fmt.Sprintf(pagesEndpointTemplateConstant, repositoryIdentifier),
			methodFlagConstant,
			httpMethodPutConstant,
			inputFlagConstant,
			stdinReferenceConstant,
			acceptHeaderFlagConstant,
			acceptHeaderValueConstant,
		},
		StandardInput: payloadBytes,
	}

	_, executionError := client.executor.ExecuteGitHubCLI(executionContext, commandDetails)
	if executionError != nil {
		return OperationError{Operation: updatePagesOperationNameConstant, Cause: executionError}
	}

	return nil
}

const httpMethodPutConstant = "PUT"
