package execshell

import (
	"fmt"
	"strings"
)

type messageStage int

const (
	messageStageStart messageStage = iota
	messageStageSuccess
	messageStageFailure
	messageStageExecutionFailure
)

const (
	genericStartTemplateConstant            = "Running %s"
	genericSuccessTemplateConstant          = "Completed %s"
	genericFailureTemplateConstant          = "%s failed with exit code %d%s"
	genericExecutionFailureTemplateConstant = "%s failed: %s"
	commandLabelTemplateConstant            = "%s%s"
	workingDirectorySuffixTemplateConstant  = " (in %s)"
	commandArgumentsJoinSeparatorConstant   = " "
	standardErrorSuffixTemplateConstant     = ": %s"
	unknownFailureMessageConstant           = "unknown error"
	emptyStringConstant                     = ""
	defaultWorkingDirectoryLabelConstant    = "current directory"
	fallbackUnknownValueLabelConstant       = "unknown"
)

const (
	gitRevParseSubcommandNameConstant     = "rev-parse"
	gitWorkTreeFlagConstant               = "--is-inside-work-tree"
	gitAbbrevRefFlagConstant              = "--abbrev-ref"
	gitSymbolicFullNameFlagConstant       = "--symbolic-full-name"
	gitUpstreamReferenceConstant          = "@{u}"
	gitHeadReferenceConstant              = "HEAD"
	gitRemoteSubcommandNameConstant       = "remote"
	gitRemoteGetURLSubcommandNameConstant = "get-url"
	gitRemoteSetURLSubcommandNameConstant = "set-url"
	gitStatusSubcommandNameConstant       = "status"
	gitStatusPorcelainFlagConstant        = "--porcelain"
	gitCheckoutSubcommandNameConstant     = "checkout"
	gitBranchSubcommandNameConstant       = "branch"
	gitDeleteFlagConstant                 = "--delete"
	gitForceFlagConstant                  = "--force"
	gitFetchSubcommandNameConstant        = "fetch"
	gitPushSubcommandNameConstant         = "push"
	gitLSRemoteSubcommandNameConstant     = "ls-remote"
	gitSymrefFlagConstant                 = "--symref"
	gitHeadsFlagConstant                  = "--heads"
	gitAddSubcommandNameConstant          = "add"
	gitCommitSubcommandNameConstant       = "commit"
	gitMessageFlagConstant                = "-m"
)

const (
	gitWorkTreeStartTemplateConstant                                = "Analyzing repository at %s"
	gitWorkTreeSuccessTemplateConstant                              = "%s is a Git repository"
	gitWorkTreeFailureTemplateConstant                              = "Could not confirm %s is a Git repository (exit code %d%s)"
	gitWorkTreeExecutionFailureTemplateConstant                     = "Could not analyze %s: %s"
	gitRemoteLookupStartTemplateConstant                            = "Checking %s remote for %s"
	gitRemoteLookupSuccessTemplateConstant                          = "%s remote for %s points to %s"
	gitRemoteLookupFailureTemplateConstant                          = "Failed to read %s remote for %s (exit code %d%s)"
	gitRemoteLookupExecutionFailureTemplateConstant                 = "Unable to read %s remote for %s: %s"
	gitRemoteUpdateStartTemplateConstant                            = "Updating %s remote for %s to %s"
	gitRemoteUpdateSuccessTemplateConstant                          = "%s remote for %s now points to %s"
	gitRemoteUpdateFailureTemplateConstant                          = "Failed to update %s remote for %s to %s (exit code %d%s)"
	gitRemoteUpdateExecutionFailureTemplateConstant                 = "Unable to update %s remote for %s to %s: %s"
	gitCurrentBranchStartTemplateConstant                           = "Identifying current branch in %s"
	gitCurrentBranchSuccessTemplateConstant                         = "Current branch in %s is %s"
	gitCurrentBranchDetachedSuccessTemplateConstant                 = "%s is in a detached HEAD state"
	gitCurrentBranchFailureTemplateConstant                         = "Failed to identify current branch in %s (exit code %d%s)"
	gitCurrentBranchExecutionFailureTemplateConstant                = "Unable to identify current branch in %s: %s"
	gitUpstreamBranchStartTemplateConstant                          = "Checking upstream branch configuration in %s"
	gitUpstreamBranchSuccessTemplateConstant                        = "Upstream branch in %s is %s"
	gitUpstreamBranchMissingSuccessTemplateConstant                 = "No upstream branch configured in %s"
	gitUpstreamBranchFailureTemplateConstant                        = "Failed to check upstream branch configuration in %s (exit code %d%s)"
	gitUpstreamBranchExecutionFailureTemplateConstant               = "Unable to check upstream branch configuration in %s: %s"
	gitRevisionStartTemplateConstant                                = "Resolving %s in %s"
	gitRevisionSuccessTemplateConstant                              = "%s in %s resolved to %s"
	gitRevisionEmptySuccessTemplateConstant                         = "%s in %s did not resolve to a revision"
	gitRevisionFailureTemplateConstant                              = "Failed to resolve %s in %s (exit code %d%s)"
	gitRevisionExecutionFailureTemplateConstant                     = "Unable to resolve %s in %s: %s"
	gitStatusStartTemplateConstant                                  = "Reviewing working tree status in %s"
	gitStatusSuccessTemplateConstant                                = "Collected working tree status for %s"
	gitStatusFailureTemplateConstant                                = "Failed to review working tree status in %s (exit code %d%s)"
	gitStatusExecutionFailureTemplateConstant                       = "Unable to review working tree status in %s: %s"
	gitCheckoutStartTemplateConstant                                = "Switching %s to branch %s"
	gitCheckoutSuccessTemplateConstant                              = "%s now on branch %s"
	gitCheckoutFailureTemplateConstant                              = "Failed to switch %s to branch %s (exit code %d%s)"
	gitCheckoutExecutionFailureTemplateConstant                     = "Unable to switch %s to branch %s: %s"
	gitBranchDeletionStartTemplateConstant                          = "Removing local branch %s in %s"
	gitBranchForceDeletionStartTemplateConstant                     = "Force removing local branch %s in %s"
	gitBranchDeletionSuccessTemplateConstant                        = "Removed local branch %s in %s"
	gitBranchDeletionFailureTemplateConstant                        = "Failed to remove local branch %s in %s (exit code %d%s)"
	gitBranchDeletionExecutionFailureTemplateConstant               = "Unable to remove local branch %s in %s: %s"
	gitBranchCreationStartTemplateConstant                          = "Creating branch %s in %s"
	gitBranchCreationWithStartPointStartTemplateConstant            = "Creating branch %s from %s in %s"
	gitBranchCreationSuccessTemplateConstant                        = "Created branch %s in %s"
	gitBranchCreationWithStartPointSuccessTemplateConstant          = "Created branch %s from %s in %s"
	gitBranchCreationFailureTemplateConstant                        = "Failed to create branch %s in %s (exit code %d%s)"
	gitBranchCreationWithStartPointFailureTemplateConstant          = "Failed to create branch %s from %s in %s (exit code %d%s)"
	gitBranchCreationExecutionFailureTemplateConstant               = "Unable to create branch %s in %s: %s"
	gitBranchCreationWithStartPointExecutionFailureTemplateConstant = "Unable to create branch %s from %s in %s: %s"
	gitFetchStartTemplateConstant                                   = "Fetching %s from %s in %s"
	gitFetchWithoutRefsStartTemplateConstant                        = "Fetching from %s in %s"
	gitFetchSuccessTemplateConstant                                 = "Fetched %s from %s in %s"
	gitFetchWithoutRefsSuccessTemplateConstant                      = "Fetched from %s in %s"
	gitFetchFailureTemplateConstant                                 = "Failed to fetch %s from %s in %s (exit code %d%s)"
	gitFetchWithoutRefsFailureTemplateConstant                      = "Failed to fetch from %s in %s (exit code %d%s)"
	gitFetchExecutionFailureTemplateConstant                        = "Unable to fetch %s from %s in %s: %s"
	gitFetchWithoutRefsExecutionFailureTemplateConstant             = "Unable to fetch from %s in %s: %s"
	gitFetchAllRemotesLabelConstant                                 = "all remotes"
	gitPushStartTemplateConstant                                    = "Pushing %s to %s from %s"
	gitPushSuccessTemplateConstant                                  = "Pushed %s to %s from %s"
	gitPushFailureTemplateConstant                                  = "Failed to push %s to %s from %s (exit code %d%s)"
	gitPushExecutionFailureTemplateConstant                         = "Unable to push %s to %s from %s: %s"
	gitPushDeletionStartTemplateConstant                            = "Deleting remote branch %s from %s in %s"
	gitPushDeletionSuccessTemplateConstant                          = "Deleted remote branch %s from %s in %s"
	gitPushDeletionFailureTemplateConstant                          = "Failed to delete remote branch %s from %s in %s (exit code %d%s)"
	gitPushDeletionExecutionFailureTemplateConstant                 = "Unable to delete remote branch %s from %s in %s: %s"
	gitLSRemoteDefaultBranchStartTemplateConstant                   = "Checking default branch on %s from %s"
	gitLSRemoteDefaultBranchSuccessTemplateConstant                 = "Retrieved default branch information for %s from %s"
	gitLSRemoteDefaultBranchFailureTemplateConstant                 = "Failed to check default branch on %s from %s (exit code %d%s)"
	gitLSRemoteDefaultBranchExecutionFailureTemplateConstant        = "Unable to check default branch on %s from %s: %s"
	gitLSRemoteHeadsStartTemplateConstant                           = "Listing branches on %s from %s"
	gitLSRemoteHeadsSuccessTemplateConstant                         = "Listed branches on %s from %s"
	gitLSRemoteHeadsFailureTemplateConstant                         = "Failed to list branches on %s from %s (exit code %d%s)"
	gitLSRemoteHeadsExecutionFailureTemplateConstant                = "Unable to list branches on %s from %s: %s"
	gitLSRemoteGenericStartTemplateConstant                         = "Querying remote references on %s from %s"
	gitLSRemoteGenericSuccessTemplateConstant                       = "Queried remote references on %s from %s"
	gitLSRemoteGenericFailureTemplateConstant                       = "Failed to query remote references on %s from %s (exit code %d%s)"
	gitLSRemoteGenericExecutionFailureTemplateConstant              = "Unable to query remote references on %s from %s: %s"
	gitAddStartTemplateConstant                                     = "Staging %s in %s"
	gitAddSuccessTemplateConstant                                   = "Staged %s in %s"
	gitAddFailureTemplateConstant                                   = "Failed to stage %s in %s (exit code %d%s)"
	gitAddExecutionFailureTemplateConstant                          = "Unable to stage %s in %s: %s"
	gitCommitStartTemplateConstant                                  = "Creating commit in %s with message %q"
	gitCommitSuccessTemplateConstant                                = "Created commit in %s with message %q"
	gitCommitFailureTemplateConstant                                = "Failed to create commit in %s with message %q (exit code %d%s)"
	gitCommitExecutionFailureTemplateConstant                       = "Unable to create commit in %s with message %q: %s"
)

const (
	githubRepoSubcommandNameConstant                  = "repo"
	githubRepoViewSubcommandNameConstant              = "view"
	githubPullRequestSubcommandNameConstant           = "pr"
	githubPullRequestListSubcommandNameConstant       = "list"
	githubPullRequestEditSubcommandNameConstant       = "edit"
	githubAPICommandNameConstant                      = "api"
	githubRepoFlagConstant                            = "--repo"
	githubStateFlagConstant                           = "--state"
	githubBaseFlagConstant                            = "--base"
	githubLimitFlagConstant                           = "--limit"
	githubMethodFlagConstant                          = "-X"
	githubFieldFlagConstant                           = "-f"
	githubInputFlagConstant                           = "--input"
	githubPagesEndpointSubstringConstant              = "/pages"
	githubBranchesEndpointSubstringConstant           = "/branches/"
	githubProtectionEndpointSuffixConstant            = "/protection"
	githubDefaultBranchFieldPrefixConstant            = "default_branch="
	githubPagesUpdateMethodConstant                   = "PUT"
	githubPagesReadMethodConstant                     = "GET"
	githubDefaultBranchUpdateMethodConstant           = "PATCH"
	githubBranchProtectionMethodConstant              = "GET"
	githubCurrentRepositoryLabelConstant              = "current repository"
	githubRepoViewIdentificationArgumentCountConstant = 2
)

const (
	githubRepoViewStartTemplateConstant                              = "Retrieving repository details for %s"
	githubRepoViewSuccessTemplateConstant                            = "Retrieved repository details for %s"
	githubRepoViewFailureTemplateConstant                            = "Failed to retrieve repository details for %s (exit code %d%s)"
	githubRepoViewExecutionFailureTemplateConstant                   = "Unable to retrieve repository details for %s: %s"
	githubPullRequestListStartTemplateConstant                       = "Listing %s pull requests for %s targeting %s"
	githubPullRequestListStartWithoutRepoTemplateConstant            = "Listing %s pull requests in the current repository targeting %s"
	githubPullRequestListSuccessTemplateConstant                     = "Listed %s pull requests for %s targeting %s"
	githubPullRequestListSuccessWithoutRepoTemplateConstant          = "Listed %s pull requests in the current repository targeting %s"
	githubPullRequestListFailureTemplateConstant                     = "Failed to list %s pull requests for %s targeting %s (exit code %d%s)"
	githubPullRequestListFailureWithoutRepoTemplateConstant          = "Failed to list %s pull requests in the current repository targeting %s (exit code %d%s)"
	githubPullRequestListExecutionFailureTemplateConstant            = "Unable to list %s pull requests for %s targeting %s: %s"
	githubPullRequestListExecutionFailureWithoutRepoTemplateConstant = "Unable to list %s pull requests in the current repository targeting %s: %s"
	githubPullRequestEditStartTemplateConstant                       = "Updating pull request #%d in %s to base %s"
	githubPullRequestEditSuccessTemplateConstant                     = "Updated pull request #%d in %s to base %s"
	githubPullRequestEditFailureTemplateConstant                     = "Failed to update pull request #%d in %s to base %s (exit code %d%s)"
	githubPullRequestEditExecutionFailureTemplateConstant            = "Unable to update pull request #%d in %s to base %s: %s"
	githubPagesUpdateStartTemplateConstant                           = "Updating GitHub Pages configuration for %s"
	githubPagesUpdateSuccessTemplateConstant                         = "Updated GitHub Pages configuration for %s"
	githubPagesUpdateFailureTemplateConstant                         = "Failed to update GitHub Pages configuration for %s (exit code %d%s)"
	githubPagesUpdateExecutionFailureTemplateConstant                = "Unable to update GitHub Pages configuration for %s: %s"
	githubPagesReadStartTemplateConstant                             = "Checking GitHub Pages configuration for %s"
	githubPagesReadSuccessTemplateConstant                           = "Read GitHub Pages configuration for %s"
	githubPagesReadFailureTemplateConstant                           = "Failed to check GitHub Pages configuration for %s (exit code %d%s)"
	githubPagesReadExecutionFailureTemplateConstant                  = "Unable to check GitHub Pages configuration for %s: %s"
	githubDefaultBranchUpdateStartTemplateConstant                   = "Setting default branch for %s to %s"
	githubDefaultBranchUpdateSuccessTemplateConstant                 = "Set default branch for %s to %s"
	githubDefaultBranchUpdateFailureTemplateConstant                 = "Failed to set default branch for %s to %s (exit code %d%s)"
	githubDefaultBranchUpdateExecutionFailureTemplateConstant        = "Unable to set default branch for %s to %s: %s"
	githubBranchProtectionStartTemplateConstant                      = "Checking branch protection for %s on %s"
	githubBranchProtectionSuccessTemplateConstant                    = "Confirmed branch protection for %s on %s"
	githubBranchProtectionFailureTemplateConstant                    = "Failed to check branch protection for %s on %s (exit code %d%s)"
	githubBranchProtectionExecutionFailureTemplateConstant           = "Unable to check branch protection for %s on %s: %s"
)

// CommandMessageFormatter builds human-readable messages for command lifecycle events.
type CommandMessageFormatter struct{}

// BuildStartedMessage formats the message describing a command about to run.
func (formatter CommandMessageFormatter) BuildStartedMessage(command ShellCommand) string {
	return formatter.buildMessage(command, ExecutionResult{}, nil, messageStageStart)
}

// BuildSuccessMessage formats the message describing a completed command with a zero exit code.
func (formatter CommandMessageFormatter) BuildSuccessMessage(command ShellCommand) string {
	return formatter.buildMessage(command, ExecutionResult{}, nil, messageStageSuccess)
}

// BuildFailureMessage formats the message describing a command that returned a non-zero exit code.
func (formatter CommandMessageFormatter) BuildFailureMessage(command ShellCommand, result ExecutionResult) string {
	return formatter.buildMessage(command, result, nil, messageStageFailure)
}

// BuildExecutionFailureMessage formats the message describing an unexpected execution failure.
func (formatter CommandMessageFormatter) BuildExecutionFailureMessage(command ShellCommand, failure error) string {
	return formatter.buildMessage(command, ExecutionResult{}, failure, messageStageExecutionFailure)
}

func (formatter CommandMessageFormatter) shouldLogStartMessage(command ShellCommand) bool {
	if command.Name != CommandGitHub {
		return true
	}
	if formatter.isGitHubRepoViewCommand(command.Details.Arguments) {
		return false
	}
	return true
}

func (formatter CommandMessageFormatter) isGitHubRepoViewCommand(arguments []string) bool {
	if len(arguments) < githubRepoViewIdentificationArgumentCountConstant {
		return false
	}
	primaryArgument := strings.TrimSpace(arguments[0])
	secondaryArgument := strings.TrimSpace(arguments[1])
	return primaryArgument == githubRepoSubcommandNameConstant && secondaryArgument == githubRepoViewSubcommandNameConstant
}

func (formatter CommandMessageFormatter) buildMessage(command ShellCommand, result ExecutionResult, failure error, stage messageStage) string {
	switch command.Name {
	case CommandGit:
		return formatter.describeGitMessage(command, result, failure, stage)
	case CommandGitHub:
		return formatter.describeGitHubMessage(command, result, failure, stage)
	case CommandCurl:
		return formatter.buildGenericMessage(command, result, failure, stage)
	default:
		return formatter.buildGenericMessage(command, result, failure, stage)
	}
}

func (formatter CommandMessageFormatter) describeGitMessage(command ShellCommand, result ExecutionResult, failure error, stage messageStage) string {
	if len(command.Details.Arguments) == 0 {
		return formatter.buildGenericMessage(command, result, failure, stage)
	}

	subcommand := strings.TrimSpace(command.Details.Arguments[0])
	switch subcommand {
	case gitRevParseSubcommandNameConstant:
		return formatter.describeGitRevParseMessage(command, result, failure, stage)
	case gitRemoteSubcommandNameConstant:
		return formatter.describeGitRemoteMessage(command, result, failure, stage)
	case gitStatusSubcommandNameConstant:
		return formatter.describeGitStatusMessage(command, result, failure, stage)
	case gitCheckoutSubcommandNameConstant:
		return formatter.describeGitCheckoutMessage(command, result, failure, stage)
	case gitBranchSubcommandNameConstant:
		return formatter.describeGitBranchMessage(command, result, failure, stage)
	case gitFetchSubcommandNameConstant:
		return formatter.describeGitFetchMessage(command, result, failure, stage)
	case gitPushSubcommandNameConstant:
		return formatter.describeGitPushMessage(command, result, failure, stage)
	case gitLSRemoteSubcommandNameConstant:
		return formatter.describeGitLSRemoteMessage(command, result, failure, stage)
	case gitAddSubcommandNameConstant:
		return formatter.describeGitAddMessage(command, result, failure, stage)
	case gitCommitSubcommandNameConstant:
		return formatter.describeGitCommitMessage(command, result, failure, stage)
	default:
		return formatter.buildGenericMessage(command, result, failure, stage)
	}
}

func (formatter CommandMessageFormatter) describeGitRevParseMessage(command ShellCommand, result ExecutionResult, failure error, stage messageStage) string {
	arguments := command.Details.Arguments
	workingDirectory := formatter.describeWorkingDirectory(command)

	if containsArgument(arguments, gitWorkTreeFlagConstant) {
		switch stage {
		case messageStageStart:
			return fmt.Sprintf(gitWorkTreeStartTemplateConstant, workingDirectory)
		case messageStageSuccess:
			return fmt.Sprintf(gitWorkTreeSuccessTemplateConstant, workingDirectory)
		case messageStageFailure:
			return fmt.Sprintf(gitWorkTreeFailureTemplateConstant, workingDirectory, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
		case messageStageExecutionFailure:
			return fmt.Sprintf(gitWorkTreeExecutionFailureTemplateConstant, workingDirectory, formatter.describeFailure(failure))
		}
	}

	if containsArgument(arguments, gitAbbrevRefFlagConstant) {
		if containsArgument(arguments, gitSymbolicFullNameFlagConstant) && containsArgument(arguments, gitUpstreamReferenceConstant) {
			switch stage {
			case messageStageStart:
				return fmt.Sprintf(gitUpstreamBranchStartTemplateConstant, workingDirectory)
			case messageStageSuccess:
				trimmed := strings.TrimSpace(result.StandardOutput)
				if len(trimmed) == 0 {
					return fmt.Sprintf(gitUpstreamBranchMissingSuccessTemplateConstant, workingDirectory)
				}
				return fmt.Sprintf(gitUpstreamBranchSuccessTemplateConstant, workingDirectory, trimmed)
			case messageStageFailure:
				return fmt.Sprintf(gitUpstreamBranchFailureTemplateConstant, workingDirectory, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
			case messageStageExecutionFailure:
				return fmt.Sprintf(gitUpstreamBranchExecutionFailureTemplateConstant, workingDirectory, formatter.describeFailure(failure))
			}
		}

		switch stage {
		case messageStageStart:
			return fmt.Sprintf(gitCurrentBranchStartTemplateConstant, workingDirectory)
		case messageStageSuccess:
			trimmed := strings.TrimSpace(result.StandardOutput)
			if strings.EqualFold(trimmed, gitHeadReferenceConstant) || len(trimmed) == 0 {
				return fmt.Sprintf(gitCurrentBranchDetachedSuccessTemplateConstant, workingDirectory)
			}
			return fmt.Sprintf(gitCurrentBranchSuccessTemplateConstant, workingDirectory, trimmed)
		case messageStageFailure:
			return fmt.Sprintf(gitCurrentBranchFailureTemplateConstant, workingDirectory, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
		case messageStageExecutionFailure:
			return fmt.Sprintf(gitCurrentBranchExecutionFailureTemplateConstant, workingDirectory, formatter.describeFailure(failure))
		}
	}

	reference := formatter.resolveRevisionReference(arguments)
	switch stage {
	case messageStageStart:
		return fmt.Sprintf(gitRevisionStartTemplateConstant, reference, workingDirectory)
	case messageStageSuccess:
		trimmed := strings.TrimSpace(result.StandardOutput)
		if len(trimmed) == 0 {
			return fmt.Sprintf(gitRevisionEmptySuccessTemplateConstant, reference, workingDirectory)
		}
		return fmt.Sprintf(gitRevisionSuccessTemplateConstant, reference, workingDirectory, trimmed)
	case messageStageFailure:
		return fmt.Sprintf(gitRevisionFailureTemplateConstant, reference, workingDirectory, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
	case messageStageExecutionFailure:
		return fmt.Sprintf(gitRevisionExecutionFailureTemplateConstant, reference, workingDirectory, formatter.describeFailure(failure))
	default:
		return formatter.buildGenericMessage(command, result, failure, stage)
	}
}

func (formatter CommandMessageFormatter) describeGitRemoteMessage(command ShellCommand, result ExecutionResult, failure error, stage messageStage) string {
	arguments := command.Details.Arguments
	workingDirectory := formatter.describeWorkingDirectory(command)
	remoteName := formatter.argumentAtIndex(arguments, 2)
	trimmedRemote := formatter.ensureValue(remoteName)

	if len(arguments) > 1 {
		subcommand := strings.TrimSpace(arguments[1])
		switch subcommand {
		case gitRemoteGetURLSubcommandNameConstant:
			remoteURL := strings.TrimSpace(result.StandardOutput)
			switch stage {
			case messageStageStart:
				return fmt.Sprintf(gitRemoteLookupStartTemplateConstant, trimmedRemote, workingDirectory)
			case messageStageSuccess:
				return fmt.Sprintf(gitRemoteLookupSuccessTemplateConstant, trimmedRemote, workingDirectory, formatter.ensureValue(remoteURL))
			case messageStageFailure:
				return fmt.Sprintf(gitRemoteLookupFailureTemplateConstant, trimmedRemote, workingDirectory, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
			case messageStageExecutionFailure:
				return fmt.Sprintf(gitRemoteLookupExecutionFailureTemplateConstant, trimmedRemote, workingDirectory, formatter.describeFailure(failure))
			}
		case gitRemoteSetURLSubcommandNameConstant:
			targetURL := formatter.argumentAtIndex(arguments, 3)
			trimmedURL := formatter.ensureValue(strings.TrimSpace(targetURL))
			switch stage {
			case messageStageStart:
				return fmt.Sprintf(gitRemoteUpdateStartTemplateConstant, trimmedRemote, workingDirectory, trimmedURL)
			case messageStageSuccess:
				return fmt.Sprintf(gitRemoteUpdateSuccessTemplateConstant, trimmedRemote, workingDirectory, trimmedURL)
			case messageStageFailure:
				return fmt.Sprintf(gitRemoteUpdateFailureTemplateConstant, trimmedRemote, workingDirectory, trimmedURL, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
			case messageStageExecutionFailure:
				return fmt.Sprintf(gitRemoteUpdateExecutionFailureTemplateConstant, trimmedRemote, workingDirectory, trimmedURL, formatter.describeFailure(failure))
			}
		}
	}

	return formatter.buildGenericMessage(command, result, failure, stage)
}

func (formatter CommandMessageFormatter) describeGitStatusMessage(command ShellCommand, result ExecutionResult, failure error, stage messageStage) string {
	workingDirectory := formatter.describeWorkingDirectory(command)
	switch stage {
	case messageStageStart:
		return fmt.Sprintf(gitStatusStartTemplateConstant, workingDirectory)
	case messageStageSuccess:
		return fmt.Sprintf(gitStatusSuccessTemplateConstant, workingDirectory)
	case messageStageFailure:
		return fmt.Sprintf(gitStatusFailureTemplateConstant, workingDirectory, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
	case messageStageExecutionFailure:
		return fmt.Sprintf(gitStatusExecutionFailureTemplateConstant, workingDirectory, formatter.describeFailure(failure))
	default:
		return formatter.buildGenericMessage(command, result, failure, stage)
	}
}

func (formatter CommandMessageFormatter) describeGitCheckoutMessage(command ShellCommand, result ExecutionResult, failure error, stage messageStage) string {
	branchName := formatter.argumentAtIndex(command.Details.Arguments, 1)
	workingDirectory := formatter.describeWorkingDirectory(command)
	trimmedBranch := formatter.ensureValue(strings.TrimSpace(branchName))
	switch stage {
	case messageStageStart:
		return fmt.Sprintf(gitCheckoutStartTemplateConstant, workingDirectory, trimmedBranch)
	case messageStageSuccess:
		return fmt.Sprintf(gitCheckoutSuccessTemplateConstant, workingDirectory, trimmedBranch)
	case messageStageFailure:
		return fmt.Sprintf(gitCheckoutFailureTemplateConstant, workingDirectory, trimmedBranch, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
	case messageStageExecutionFailure:
		return fmt.Sprintf(gitCheckoutExecutionFailureTemplateConstant, workingDirectory, trimmedBranch, formatter.describeFailure(failure))
	default:
		return formatter.buildGenericMessage(command, result, failure, stage)
	}
}

func (formatter CommandMessageFormatter) describeGitBranchMessage(command ShellCommand, result ExecutionResult, failure error, stage messageStage) string {
	arguments := command.Details.Arguments
	workingDirectory := formatter.describeWorkingDirectory(command)
	branchName := formatter.extractBranchName(arguments)
	trimmedBranch := formatter.ensureValue(strings.TrimSpace(branchName))
	hasDeleteFlag := containsArgument(arguments, gitDeleteFlagConstant)
	hasForceFlag := containsArgument(arguments, gitForceFlagConstant)
	startPoint := formatter.extractBranchStartPoint(arguments)
	trimmedStartPoint := formatter.ensureValue(strings.TrimSpace(startPoint))

	if hasDeleteFlag {
		switch stage {
		case messageStageStart:
			if hasForceFlag {
				return fmt.Sprintf(gitBranchForceDeletionStartTemplateConstant, trimmedBranch, workingDirectory)
			}
			return fmt.Sprintf(gitBranchDeletionStartTemplateConstant, trimmedBranch, workingDirectory)
		case messageStageSuccess:
			return fmt.Sprintf(gitBranchDeletionSuccessTemplateConstant, trimmedBranch, workingDirectory)
		case messageStageFailure:
			return fmt.Sprintf(gitBranchDeletionFailureTemplateConstant, trimmedBranch, workingDirectory, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
		case messageStageExecutionFailure:
			return fmt.Sprintf(gitBranchDeletionExecutionFailureTemplateConstant, trimmedBranch, workingDirectory, formatter.describeFailure(failure))
		}
	}

	if len(strings.TrimSpace(trimmedStartPoint)) > 0 {
		switch stage {
		case messageStageStart:
			return fmt.Sprintf(gitBranchCreationWithStartPointStartTemplateConstant, trimmedBranch, trimmedStartPoint, workingDirectory)
		case messageStageSuccess:
			return fmt.Sprintf(gitBranchCreationWithStartPointSuccessTemplateConstant, trimmedBranch, trimmedStartPoint, workingDirectory)
		case messageStageFailure:
			return fmt.Sprintf(gitBranchCreationWithStartPointFailureTemplateConstant, trimmedBranch, trimmedStartPoint, workingDirectory, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
		case messageStageExecutionFailure:
			return fmt.Sprintf(gitBranchCreationWithStartPointExecutionFailureTemplateConstant, trimmedBranch, trimmedStartPoint, workingDirectory, formatter.describeFailure(failure))
		}
	}

	switch stage {
	case messageStageStart:
		return fmt.Sprintf(gitBranchCreationStartTemplateConstant, trimmedBranch, workingDirectory)
	case messageStageSuccess:
		return fmt.Sprintf(gitBranchCreationSuccessTemplateConstant, trimmedBranch, workingDirectory)
	case messageStageFailure:
		return fmt.Sprintf(gitBranchCreationFailureTemplateConstant, trimmedBranch, workingDirectory, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
	case messageStageExecutionFailure:
		return fmt.Sprintf(gitBranchCreationExecutionFailureTemplateConstant, trimmedBranch, workingDirectory, formatter.describeFailure(failure))
	default:
		return formatter.buildGenericMessage(command, result, failure, stage)
	}
}

func (formatter CommandMessageFormatter) describeGitFetchMessage(command ShellCommand, result ExecutionResult, failure error, stage messageStage) string {
	workingDirectory := formatter.describeWorkingDirectory(command)
	remoteName, references := formatter.extractRemoteAndReferences(command.Details.Arguments[1:])
	trimmedRemote := strings.TrimSpace(remoteName)
	if len(trimmedRemote) == 0 {
		trimmedRemote = gitFetchAllRemotesLabelConstant
	}
	joinedReferences := formatter.joinReferences(references)

	switch stage {
	case messageStageStart:
		if len(joinedReferences) > 0 {
			return fmt.Sprintf(gitFetchStartTemplateConstant, joinedReferences, trimmedRemote, workingDirectory)
		}
		return fmt.Sprintf(gitFetchWithoutRefsStartTemplateConstant, trimmedRemote, workingDirectory)
	case messageStageSuccess:
		if len(joinedReferences) > 0 {
			return fmt.Sprintf(gitFetchSuccessTemplateConstant, joinedReferences, trimmedRemote, workingDirectory)
		}
		return fmt.Sprintf(gitFetchWithoutRefsSuccessTemplateConstant, trimmedRemote, workingDirectory)
	case messageStageFailure:
		if len(joinedReferences) > 0 {
			return fmt.Sprintf(gitFetchFailureTemplateConstant, joinedReferences, trimmedRemote, workingDirectory, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
		}
		return fmt.Sprintf(gitFetchWithoutRefsFailureTemplateConstant, trimmedRemote, workingDirectory, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
	case messageStageExecutionFailure:
		if len(joinedReferences) > 0 {
			return fmt.Sprintf(gitFetchExecutionFailureTemplateConstant, joinedReferences, trimmedRemote, workingDirectory, formatter.describeFailure(failure))
		}
		return fmt.Sprintf(gitFetchWithoutRefsExecutionFailureTemplateConstant, trimmedRemote, workingDirectory, formatter.describeFailure(failure))
	default:
		return formatter.buildGenericMessage(command, result, failure, stage)
	}
}

func (formatter CommandMessageFormatter) describeGitPushMessage(command ShellCommand, result ExecutionResult, failure error, stage messageStage) string {
	workingDirectory := formatter.describeWorkingDirectory(command)
	arguments := command.Details.Arguments
	remoteName := formatter.argumentAtIndex(arguments, 1)
	trimmedRemote := formatter.ensureValue(strings.TrimSpace(remoteName))
	deletionTarget := formatter.extractDeletionTarget(arguments)
	trimmedDeletionTarget := formatter.ensureValue(strings.TrimSpace(deletionTarget))

	if len(trimmedDeletionTarget) > 0 {
		switch stage {
		case messageStageStart:
			return fmt.Sprintf(gitPushDeletionStartTemplateConstant, trimmedDeletionTarget, trimmedRemote, workingDirectory)
		case messageStageSuccess:
			return fmt.Sprintf(gitPushDeletionSuccessTemplateConstant, trimmedDeletionTarget, trimmedRemote, workingDirectory)
		case messageStageFailure:
			return fmt.Sprintf(gitPushDeletionFailureTemplateConstant, trimmedDeletionTarget, trimmedRemote, workingDirectory, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
		case messageStageExecutionFailure:
			return fmt.Sprintf(gitPushDeletionExecutionFailureTemplateConstant, trimmedDeletionTarget, trimmedRemote, workingDirectory, formatter.describeFailure(failure))
		}
	}

	branchReference := formatter.argumentAtIndex(arguments, 2)
	trimmedBranch := formatter.ensureValue(strings.TrimSpace(branchReference))
	switch stage {
	case messageStageStart:
		return fmt.Sprintf(gitPushStartTemplateConstant, trimmedBranch, trimmedRemote, workingDirectory)
	case messageStageSuccess:
		return fmt.Sprintf(gitPushSuccessTemplateConstant, trimmedBranch, trimmedRemote, workingDirectory)
	case messageStageFailure:
		return fmt.Sprintf(gitPushFailureTemplateConstant, trimmedBranch, trimmedRemote, workingDirectory, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
	case messageStageExecutionFailure:
		return fmt.Sprintf(gitPushExecutionFailureTemplateConstant, trimmedBranch, trimmedRemote, workingDirectory, formatter.describeFailure(failure))
	default:
		return formatter.buildGenericMessage(command, result, failure, stage)
	}
}

func (formatter CommandMessageFormatter) describeGitLSRemoteMessage(command ShellCommand, result ExecutionResult, failure error, stage messageStage) string {
	workingDirectory := formatter.describeWorkingDirectory(command)
	arguments := command.Details.Arguments
	remoteName := formatter.extractRemoteForLSRemote(arguments)
	trimmedRemote := formatter.ensureValue(remoteName)
	hasSymref := containsArgument(arguments, gitSymrefFlagConstant)
	listsHeads := containsArgument(arguments, gitHeadsFlagConstant)

	switch stage {
	case messageStageStart:
		switch {
		case hasSymref:
			return fmt.Sprintf(gitLSRemoteDefaultBranchStartTemplateConstant, trimmedRemote, workingDirectory)
		case listsHeads:
			return fmt.Sprintf(gitLSRemoteHeadsStartTemplateConstant, trimmedRemote, workingDirectory)
		default:
			return fmt.Sprintf(gitLSRemoteGenericStartTemplateConstant, trimmedRemote, workingDirectory)
		}
	case messageStageSuccess:
		switch {
		case hasSymref:
			return fmt.Sprintf(gitLSRemoteDefaultBranchSuccessTemplateConstant, trimmedRemote, workingDirectory)
		case listsHeads:
			return fmt.Sprintf(gitLSRemoteHeadsSuccessTemplateConstant, trimmedRemote, workingDirectory)
		default:
			return fmt.Sprintf(gitLSRemoteGenericSuccessTemplateConstant, trimmedRemote, workingDirectory)
		}
	case messageStageFailure:
		switch {
		case hasSymref:
			return fmt.Sprintf(gitLSRemoteDefaultBranchFailureTemplateConstant, trimmedRemote, workingDirectory, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
		case listsHeads:
			return fmt.Sprintf(gitLSRemoteHeadsFailureTemplateConstant, trimmedRemote, workingDirectory, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
		default:
			return fmt.Sprintf(gitLSRemoteGenericFailureTemplateConstant, trimmedRemote, workingDirectory, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
		}
	case messageStageExecutionFailure:
		switch {
		case hasSymref:
			return fmt.Sprintf(gitLSRemoteDefaultBranchExecutionFailureTemplateConstant, trimmedRemote, workingDirectory, formatter.describeFailure(failure))
		case listsHeads:
			return fmt.Sprintf(gitLSRemoteHeadsExecutionFailureTemplateConstant, trimmedRemote, workingDirectory, formatter.describeFailure(failure))
		default:
			return fmt.Sprintf(gitLSRemoteGenericExecutionFailureTemplateConstant, trimmedRemote, workingDirectory, formatter.describeFailure(failure))
		}
	default:
		return formatter.buildGenericMessage(command, result, failure, stage)
	}
}

func (formatter CommandMessageFormatter) describeGitAddMessage(command ShellCommand, result ExecutionResult, failure error, stage messageStage) string {
	workingDirectory := formatter.describeWorkingDirectory(command)
	targetPath := formatter.extractFirstNonFlagArgument(command.Details.Arguments[1:])
	trimmedTarget := formatter.ensureValue(targetPath)
	switch stage {
	case messageStageStart:
		return fmt.Sprintf(gitAddStartTemplateConstant, trimmedTarget, workingDirectory)
	case messageStageSuccess:
		return fmt.Sprintf(gitAddSuccessTemplateConstant, trimmedTarget, workingDirectory)
	case messageStageFailure:
		return fmt.Sprintf(gitAddFailureTemplateConstant, trimmedTarget, workingDirectory, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
	case messageStageExecutionFailure:
		return fmt.Sprintf(gitAddExecutionFailureTemplateConstant, trimmedTarget, workingDirectory, formatter.describeFailure(failure))
	default:
		return formatter.buildGenericMessage(command, result, failure, stage)
	}
}

func (formatter CommandMessageFormatter) describeGitCommitMessage(command ShellCommand, result ExecutionResult, failure error, stage messageStage) string {
	workingDirectory := formatter.describeWorkingDirectory(command)
	commitMessage := formatter.extractCommitMessage(command.Details.Arguments)
	switch stage {
	case messageStageStart:
		return fmt.Sprintf(gitCommitStartTemplateConstant, workingDirectory, commitMessage)
	case messageStageSuccess:
		return fmt.Sprintf(gitCommitSuccessTemplateConstant, workingDirectory, commitMessage)
	case messageStageFailure:
		return fmt.Sprintf(gitCommitFailureTemplateConstant, workingDirectory, commitMessage, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
	case messageStageExecutionFailure:
		return fmt.Sprintf(gitCommitExecutionFailureTemplateConstant, workingDirectory, commitMessage, formatter.describeFailure(failure))
	default:
		return formatter.buildGenericMessage(command, result, failure, stage)
	}
}

func (formatter CommandMessageFormatter) describeGitHubMessage(command ShellCommand, result ExecutionResult, failure error, stage messageStage) string {
	if len(command.Details.Arguments) == 0 {
		return formatter.buildGenericMessage(command, result, failure, stage)
	}

	primary := strings.TrimSpace(command.Details.Arguments[0])
	switch primary {
	case githubRepoSubcommandNameConstant:
		return formatter.describeGitHubRepoCommand(command, result, failure, stage)
	case githubPullRequestSubcommandNameConstant:
		return formatter.describeGitHubPullRequestCommand(command, result, failure, stage)
	case githubAPICommandNameConstant:
		return formatter.describeGitHubAPICommand(command, result, failure, stage)
	default:
		return formatter.buildGenericMessage(command, result, failure, stage)
	}
}

func (formatter CommandMessageFormatter) describeGitHubRepoCommand(command ShellCommand, result ExecutionResult, failure error, stage messageStage) string {
	arguments := command.Details.Arguments
	if len(arguments) < 3 {
		return formatter.buildGenericMessage(command, result, failure, stage)
	}
	subcommand := strings.TrimSpace(arguments[1])
	repository := formatter.ensureValue(strings.TrimSpace(arguments[2]))

	if subcommand != githubRepoViewSubcommandNameConstant {
		return formatter.buildGenericMessage(command, result, failure, stage)
	}

	switch stage {
	case messageStageStart:
		return fmt.Sprintf(githubRepoViewStartTemplateConstant, repository)
	case messageStageSuccess:
		return fmt.Sprintf(githubRepoViewSuccessTemplateConstant, repository)
	case messageStageFailure:
		return fmt.Sprintf(githubRepoViewFailureTemplateConstant, repository, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
	case messageStageExecutionFailure:
		return fmt.Sprintf(githubRepoViewExecutionFailureTemplateConstant, repository, formatter.describeFailure(failure))
	default:
		return formatter.buildGenericMessage(command, result, failure, stage)
	}
}

func (formatter CommandMessageFormatter) describeGitHubPullRequestCommand(command ShellCommand, result ExecutionResult, failure error, stage messageStage) string {
	arguments := command.Details.Arguments
	if len(arguments) < 2 {
		return formatter.buildGenericMessage(command, result, failure, stage)
	}
	subcommand := strings.TrimSpace(arguments[1])

	switch subcommand {
	case githubPullRequestListSubcommandNameConstant:
		return formatter.describeGitHubPullRequestList(command, result, failure, stage)
	case githubPullRequestEditSubcommandNameConstant:
		return formatter.describeGitHubPullRequestEdit(command, result, failure, stage)
	default:
		return formatter.buildGenericMessage(command, result, failure, stage)
	}
}

func (formatter CommandMessageFormatter) describeGitHubPullRequestList(command ShellCommand, result ExecutionResult, failure error, stage messageStage) string {
	arguments := command.Details.Arguments
	state := formatter.ensureValue(findFlagValue(arguments, githubStateFlagConstant))
	baseBranch := formatter.ensureValue(findFlagValue(arguments, githubBaseFlagConstant))
	repository := strings.TrimSpace(findFlagValue(arguments, githubRepoFlagConstant))
	hasRepositoryFlag := len(repository) > 0
	trimmedRepository := repository
	if !hasRepositoryFlag {
		trimmedRepository = githubCurrentRepositoryLabelConstant
	}

	switch stage {
	case messageStageStart:
		if hasRepositoryFlag {
			return fmt.Sprintf(githubPullRequestListStartTemplateConstant, state, trimmedRepository, baseBranch)
		}
		return fmt.Sprintf(githubPullRequestListStartWithoutRepoTemplateConstant, state, baseBranch)
	case messageStageSuccess:
		if hasRepositoryFlag {
			return fmt.Sprintf(githubPullRequestListSuccessTemplateConstant, state, trimmedRepository, baseBranch)
		}
		return fmt.Sprintf(githubPullRequestListSuccessWithoutRepoTemplateConstant, state, baseBranch)
	case messageStageFailure:
		if hasRepositoryFlag {
			return fmt.Sprintf(githubPullRequestListFailureTemplateConstant, state, trimmedRepository, baseBranch, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
		}
		return fmt.Sprintf(githubPullRequestListFailureWithoutRepoTemplateConstant, state, baseBranch, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
	case messageStageExecutionFailure:
		if hasRepositoryFlag {
			return fmt.Sprintf(githubPullRequestListExecutionFailureTemplateConstant, state, trimmedRepository, baseBranch, formatter.describeFailure(failure))
		}
		return fmt.Sprintf(githubPullRequestListExecutionFailureWithoutRepoTemplateConstant, state, baseBranch, formatter.describeFailure(failure))
	default:
		return formatter.buildGenericMessage(command, result, failure, stage)
	}
}

func (formatter CommandMessageFormatter) describeGitHubPullRequestEdit(command ShellCommand, result ExecutionResult, failure error, stage messageStage) string {
	arguments := command.Details.Arguments
	if len(arguments) < 3 {
		return formatter.buildGenericMessage(command, result, failure, stage)
	}
	pullRequestNumber := parseIntegerArgument(arguments[2])
	repository := formatter.ensureValue(strings.TrimSpace(findFlagValue(arguments, githubRepoFlagConstant)))
	if len(repository) == 0 {
		repository = githubCurrentRepositoryLabelConstant
	}
	baseBranch := formatter.ensureValue(findFlagValue(arguments, githubBaseFlagConstant))

	switch stage {
	case messageStageStart:
		return fmt.Sprintf(githubPullRequestEditStartTemplateConstant, pullRequestNumber, repository, baseBranch)
	case messageStageSuccess:
		return fmt.Sprintf(githubPullRequestEditSuccessTemplateConstant, pullRequestNumber, repository, baseBranch)
	case messageStageFailure:
		return fmt.Sprintf(githubPullRequestEditFailureTemplateConstant, pullRequestNumber, repository, baseBranch, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
	case messageStageExecutionFailure:
		return fmt.Sprintf(githubPullRequestEditExecutionFailureTemplateConstant, pullRequestNumber, repository, baseBranch, formatter.describeFailure(failure))
	default:
		return formatter.buildGenericMessage(command, result, failure, stage)
	}
}

func (formatter CommandMessageFormatter) describeGitHubAPICommand(command ShellCommand, result ExecutionResult, failure error, stage messageStage) string {
	arguments := command.Details.Arguments
	if len(arguments) < 2 {
		return formatter.buildGenericMessage(command, result, failure, stage)
	}

	endpoint := strings.TrimSpace(arguments[1])
	method := strings.TrimSpace(findFlagValue(arguments, githubMethodFlagConstant))

	switch {
	case strings.Contains(endpoint, githubPagesEndpointSubstringConstant):
		repository := formatter.extractRepositoryFromEndpoint(endpoint, githubPagesEndpointSubstringConstant)
		if method == githubPagesUpdateMethodConstant {
			switch stage {
			case messageStageStart:
				return fmt.Sprintf(githubPagesUpdateStartTemplateConstant, repository)
			case messageStageSuccess:
				return fmt.Sprintf(githubPagesUpdateSuccessTemplateConstant, repository)
			case messageStageFailure:
				return fmt.Sprintf(githubPagesUpdateFailureTemplateConstant, repository, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
			case messageStageExecutionFailure:
				return fmt.Sprintf(githubPagesUpdateExecutionFailureTemplateConstant, repository, formatter.describeFailure(failure))
			}
		}
		switch stage {
		case messageStageStart:
			return fmt.Sprintf(githubPagesReadStartTemplateConstant, repository)
		case messageStageSuccess:
			return fmt.Sprintf(githubPagesReadSuccessTemplateConstant, repository)
		case messageStageFailure:
			return fmt.Sprintf(githubPagesReadFailureTemplateConstant, repository, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
		case messageStageExecutionFailure:
			return fmt.Sprintf(githubPagesReadExecutionFailureTemplateConstant, repository, formatter.describeFailure(failure))
		}
	case strings.Contains(endpoint, githubProtectionEndpointSuffixConstant) && strings.Contains(endpoint, githubBranchesEndpointSubstringConstant):
		repository, branch := formatter.extractRepositoryAndBranchFromProtectionEndpoint(endpoint)
		switch stage {
		case messageStageStart:
			return fmt.Sprintf(githubBranchProtectionStartTemplateConstant, branch, repository)
		case messageStageSuccess:
			return fmt.Sprintf(githubBranchProtectionSuccessTemplateConstant, branch, repository)
		case messageStageFailure:
			return fmt.Sprintf(githubBranchProtectionFailureTemplateConstant, branch, repository, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
		case messageStageExecutionFailure:
			return fmt.Sprintf(githubBranchProtectionExecutionFailureTemplateConstant, branch, repository, formatter.describeFailure(failure))
		}
	default:
		if method == githubDefaultBranchUpdateMethodConstant {
			repository := formatter.extractRepositoryFromEndpoint(endpoint, emptyStringConstant)
			defaultBranchField := findFlagValue(arguments, githubFieldFlagConstant)
			branchValue := formatter.extractDefaultBranchValue(defaultBranchField)
			switch stage {
			case messageStageStart:
				return fmt.Sprintf(githubDefaultBranchUpdateStartTemplateConstant, repository, branchValue)
			case messageStageSuccess:
				return fmt.Sprintf(githubDefaultBranchUpdateSuccessTemplateConstant, repository, branchValue)
			case messageStageFailure:
				return fmt.Sprintf(githubDefaultBranchUpdateFailureTemplateConstant, repository, branchValue, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
			case messageStageExecutionFailure:
				return fmt.Sprintf(githubDefaultBranchUpdateExecutionFailureTemplateConstant, repository, branchValue, formatter.describeFailure(failure))
			}
		}
	}

	return formatter.buildGenericMessage(command, result, failure, stage)
}

func (formatter CommandMessageFormatter) buildGenericMessage(command ShellCommand, result ExecutionResult, failure error, stage messageStage) string {
	commandLabel := formatter.formatCommandLabel(command)
	switch stage {
	case messageStageStart:
		return fmt.Sprintf(genericStartTemplateConstant, commandLabel)
	case messageStageSuccess:
		return fmt.Sprintf(genericSuccessTemplateConstant, commandLabel)
	case messageStageFailure:
		return fmt.Sprintf(genericFailureTemplateConstant, commandLabel, result.ExitCode, formatter.formatStandardErrorSuffix(result.StandardError))
	case messageStageExecutionFailure:
		return fmt.Sprintf(genericExecutionFailureTemplateConstant, commandLabel, formatter.describeFailure(failure))
	default:
		return emptyStringConstant
	}
}

func (formatter CommandMessageFormatter) formatCommandLabel(command ShellCommand) string {
	commandLabel := string(command.Name)
	if len(command.Details.Arguments) > 0 {
		commandLabel = fmt.Sprintf("%s %s", commandLabel, strings.Join(command.Details.Arguments, commandArgumentsJoinSeparatorConstant))
	}
	workingDirectorySuffix := formatter.formatWorkingDirectorySuffix(command)
	return fmt.Sprintf(commandLabelTemplateConstant, commandLabel, workingDirectorySuffix)
}

func (formatter CommandMessageFormatter) formatWorkingDirectorySuffix(command ShellCommand) string {
	trimmedWorkingDirectory := strings.TrimSpace(command.Details.WorkingDirectory)
	if len(trimmedWorkingDirectory) == 0 {
		return emptyStringConstant
	}
	return fmt.Sprintf(workingDirectorySuffixTemplateConstant, trimmedWorkingDirectory)
}

func (formatter CommandMessageFormatter) formatStandardErrorSuffix(standardError string) string {
	trimmedStandardError := strings.TrimSpace(standardError)
	if len(trimmedStandardError) == 0 {
		return emptyStringConstant
	}
	return fmt.Sprintf(standardErrorSuffixTemplateConstant, trimmedStandardError)
}

func (formatter CommandMessageFormatter) describeWorkingDirectory(command ShellCommand) string {
	trimmedWorkingDirectory := strings.TrimSpace(command.Details.WorkingDirectory)
	if len(trimmedWorkingDirectory) == 0 {
		return defaultWorkingDirectoryLabelConstant
	}
	return trimmedWorkingDirectory
}

func (formatter CommandMessageFormatter) describeFailure(failure error) string {
	if failure == nil {
		return unknownFailureMessageConstant
	}
	return failure.Error()
}

func containsArgument(arguments []string, value string) bool {
	for _, argument := range arguments {
		if strings.TrimSpace(argument) == value {
			return true
		}
	}
	return false
}

func (formatter CommandMessageFormatter) resolveRevisionReference(arguments []string) string {
	if len(arguments) == 0 {
		return fallbackUnknownValueLabelConstant
	}
	lastArgument := strings.TrimSpace(arguments[len(arguments)-1])
	if len(lastArgument) == 0 {
		return fallbackUnknownValueLabelConstant
	}
	return lastArgument
}

func (formatter CommandMessageFormatter) argumentAtIndex(arguments []string, index int) string {
	if index >= 0 && index < len(arguments) {
		return arguments[index]
	}
	return emptyStringConstant
}

func (formatter CommandMessageFormatter) ensureValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) == 0 {
		return fallbackUnknownValueLabelConstant
	}
	return trimmed
}

func (formatter CommandMessageFormatter) extractBranchName(arguments []string) string {
	for index := len(arguments) - 1; index >= 0; index-- {
		argument := strings.TrimSpace(arguments[index])
		if len(argument) == 0 {
			continue
		}
		if strings.HasPrefix(argument, "-") {
			continue
		}
		return argument
	}
	return emptyStringConstant
}

func (formatter CommandMessageFormatter) extractBranchStartPoint(arguments []string) string {
	if len(arguments) >= 3 {
		potentialStartPoint := strings.TrimSpace(arguments[len(arguments)-1])
		previousArgument := strings.TrimSpace(arguments[len(arguments)-2])
		if !strings.HasPrefix(potentialStartPoint, "-") && previousArgument != gitForceFlagConstant && previousArgument != gitDeleteFlagConstant {
			return potentialStartPoint
		}
	}
	return emptyStringConstant
}

func (formatter CommandMessageFormatter) extractRemoteAndReferences(arguments []string) (string, []string) {
	remoteName := emptyStringConstant
	references := []string{}
	for _, argument := range arguments {
		trimmed := strings.TrimSpace(argument)
		if len(trimmed) == 0 {
			continue
		}
		if strings.HasPrefix(trimmed, "-") {
			continue
		}
		if len(remoteName) == 0 {
			remoteName = trimmed
			continue
		}
		references = append(references, trimmed)
	}
	return remoteName, references
}

func (formatter CommandMessageFormatter) joinReferences(references []string) string {
	cleaned := make([]string, 0, len(references))
	for _, reference := range references {
		trimmed := strings.TrimSpace(reference)
		if len(trimmed) == 0 {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}
	return strings.Join(cleaned, ", ")
}

func (formatter CommandMessageFormatter) extractDeletionTarget(arguments []string) string {
	for index := 0; index < len(arguments); index++ {
		argument := strings.TrimSpace(arguments[index])
		if argument == "--delete" && index+1 < len(arguments) {
			return arguments[index+1]
		}
	}
	return emptyStringConstant
}

func (formatter CommandMessageFormatter) extractRemoteForLSRemote(arguments []string) string {
	for _, argument := range arguments {
		trimmed := strings.TrimSpace(argument)
		if len(trimmed) == 0 {
			continue
		}
		if strings.HasPrefix(trimmed, "-") {
			continue
		}
		return trimmed
	}
	return emptyStringConstant
}

func (formatter CommandMessageFormatter) extractFirstNonFlagArgument(arguments []string) string {
	for _, argument := range arguments {
		trimmed := strings.TrimSpace(argument)
		if len(trimmed) == 0 {
			continue
		}
		if strings.HasPrefix(trimmed, "-") {
			continue
		}
		return trimmed
	}
	return emptyStringConstant
}

func (formatter CommandMessageFormatter) extractCommitMessage(arguments []string) string {
	for index := 0; index < len(arguments); index++ {
		if strings.TrimSpace(arguments[index]) == gitMessageFlagConstant && index+1 < len(arguments) {
			return strings.TrimSpace(arguments[index+1])
		}
	}
	return fallbackUnknownValueLabelConstant
}

func findFlagValue(arguments []string, flag string) string {
	for index := 0; index < len(arguments); index++ {
		if strings.TrimSpace(arguments[index]) == flag && index+1 < len(arguments) {
			return strings.TrimSpace(arguments[index+1])
		}
	}
	return emptyStringConstant
}

func parseIntegerArgument(argument string) int {
	trimmed := strings.TrimSpace(argument)
	value := 0
	for _, character := range trimmed {
		if character < '0' || character > '9' {
			return value
		}
		value = value*10 + int(character-'0')
	}
	return value
}

func (formatter CommandMessageFormatter) extractRepositoryFromEndpoint(endpoint string, suffix string) string {
	trimmed := strings.TrimPrefix(strings.TrimSpace(endpoint), "repos/")
	if len(trimmed) == 0 {
		return githubCurrentRepositoryLabelConstant
	}
	if len(suffix) > 0 && strings.HasSuffix(trimmed, strings.TrimPrefix(suffix, "/")) {
		trimmed = strings.TrimSuffix(trimmed, strings.TrimPrefix(suffix, "/"))
	}
	trimmed = strings.TrimSuffix(trimmed, "/")
	if len(trimmed) == 0 {
		return githubCurrentRepositoryLabelConstant
	}
	return trimmed
}

func (formatter CommandMessageFormatter) extractRepositoryAndBranchFromProtectionEndpoint(endpoint string) (string, string) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(endpoint), "repos/")
	if len(trimmed) == 0 {
		return githubCurrentRepositoryLabelConstant, fallbackUnknownValueLabelConstant
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) < 4 {
		return formatter.extractRepositoryFromEndpoint(endpoint, emptyStringConstant), fallbackUnknownValueLabelConstant
	}
	repository := strings.Join(parts[:2], "/")
	branch := parts[3]
	return repository, branch
}

func (formatter CommandMessageFormatter) extractDefaultBranchValue(fieldArgument string) string {
	trimmed := strings.TrimSpace(fieldArgument)
	if !strings.HasPrefix(trimmed, githubDefaultBranchFieldPrefixConstant) {
		return fallbackUnknownValueLabelConstant
	}
	value := strings.TrimPrefix(trimmed, githubDefaultBranchFieldPrefixConstant)
	if len(value) == 0 {
		return fallbackUnknownValueLabelConstant
	}
	return value
}
