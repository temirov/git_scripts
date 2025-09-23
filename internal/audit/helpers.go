package audit

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
)

const (
	githubHostConstant                 = "github.com"
	gitProtocolPrefixConstant          = "git@github.com:"
	sshProtocolPrefixConstant          = "ssh://git@github.com/"
	httpsProtocolPrefixConstant        = "https://github.com/"
	gitSuffixConstant                  = ".git"
	repositoryOwnerSeparatorConstant   = "/"
	refsHeadsPrefixConstant            = "refs/heads/"
	upstreamReferenceCommandArgument   = "@{u}"
	gitFetchSubcommandConstant         = "fetch"
	gitQuietFlagConstant               = "-q"
	gitNoTagsFlagConstant              = "--no-tags"
	gitNoRecurseSubmodulesFlagConstant = "--no-recurse-submodules"
	gitRevParseSubcommandConstant      = "rev-parse"
	gitAbbrevRefFlagConstant           = "--abbrev-ref"
	gitSymbolicFullNameFlagConstant    = "--symbolic-full-name"
	gitHeadReferenceConstant           = "HEAD"
	gitLSRemoteSubcommandConstant      = "ls-remote"
	gitSymrefFlagConstant              = "--symref"
	gitReferenceSeparator              = "\t"
)

var errOwnerRepoNotDetected = errors.New("owner repository not detected")

func detectRemoteProtocol(remote string) RemoteProtocolType {
	switch {
	case strings.HasPrefix(remote, gitProtocolPrefixConstant):
		return RemoteProtocolGit
	case strings.HasPrefix(remote, sshProtocolPrefixConstant):
		return RemoteProtocolSSH
	case strings.HasPrefix(remote, httpsProtocolPrefixConstant):
		return RemoteProtocolHTTPS
	default:
		return RemoteProtocolOther
	}
}

func canonicalizeOwnerRepo(remote string) (string, error) {
	trimmed := strings.TrimSpace(remote)
	switch {
	case strings.HasPrefix(trimmed, gitProtocolPrefixConstant):
		trimmed = strings.TrimPrefix(trimmed, gitProtocolPrefixConstant)
	case strings.HasPrefix(trimmed, sshProtocolPrefixConstant):
		trimmed = strings.TrimPrefix(trimmed, sshProtocolPrefixConstant)
	case strings.HasPrefix(trimmed, httpsProtocolPrefixConstant):
		trimmed = strings.TrimPrefix(trimmed, httpsProtocolPrefixConstant)
	default:
		return "", errOwnerRepoNotDetected
	}

	trimmed = strings.TrimSuffix(trimmed, gitSuffixConstant)
	segments := strings.Split(trimmed, repositoryOwnerSeparatorConstant)
	if len(segments) < 2 {
		return "", errOwnerRepoNotDetected
	}
	owner := segments[0]
	repository := segments[1]
	if len(owner) == 0 || len(repository) == 0 {
		return "", errOwnerRepoNotDetected
	}
	return fmt.Sprintf("%s/%s", owner, repository), nil
}

func finalRepositoryName(ownerRepo string) string {
	segments := strings.Split(ownerRepo, repositoryOwnerSeparatorConstant)
	if len(segments) == 0 {
		return ""
	}
	return segments[len(segments)-1]
}

func buildRemoteURL(protocol RemoteProtocolType, ownerRepo string) (string, error) {
	trimmed := strings.TrimSpace(ownerRepo)
	if len(trimmed) == 0 {
		return "", errOwnerRepoNotDetected
	}
	switch protocol {
	case RemoteProtocolGit:
		return fmt.Sprintf("%s%s.git", gitProtocolPrefixConstant, trimmed), nil
	case RemoteProtocolSSH:
		return fmt.Sprintf("%s%s.git", sshProtocolPrefixConstant, trimmed), nil
	case RemoteProtocolHTTPS:
		return fmt.Sprintf("%s%s.git", httpsProtocolPrefixConstant, trimmed), nil
	default:
		return "", fmt.Errorf("unknown protocol %s", protocol)
	}
}

func ownerRepoCaseInsensitiveEqual(first string, second string) bool {
	return strings.EqualFold(first, second)
}

func sanitizeBranchName(branch string) string {
	trimmed := strings.TrimSpace(branch)
	if trimmed == gitHeadReferenceConstant {
		return "DETACHED"
	}
	return trimmed
}

func computeIntermediateRenamePath(oldPath string, timestamp int64) string {
	return fmt.Sprintf("%s.rename.%d", oldPath, timestamp)
}

func remoteFetchArguments(branch string) []string {
	return []string{
		gitFetchSubcommandConstant,
		gitQuietFlagConstant,
		gitNoTagsFlagConstant,
		gitNoRecurseSubmodulesFlagConstant,
		originRemoteNameConstant,
		branch,
	}
}

func upstreamReferenceArguments() []string {
	return []string{
		gitRevParseSubcommandConstant,
		gitAbbrevRefFlagConstant,
		gitSymbolicFullNameFlagConstant,
		upstreamReferenceCommandArgument,
	}
}

func headRevisionArguments() []string {
	return []string{
		gitRevParseSubcommandConstant,
		gitHeadReferenceConstant,
	}
}

func revisionArguments(reference string) []string {
	return []string{
		gitRevParseSubcommandConstant,
		reference,
	}
}

func fallbackRemoteRevisionReferences(branch string) []string {
	return []string{
		fmt.Sprintf("refs/remotes/%s/%s", originRemoteNameConstant, branch),
		fmt.Sprintf("%s/%s", originRemoteNameConstant, branch),
	}
}

func lsRemoteHeadArguments() []string {
	return []string{
		gitLSRemoteSubcommandConstant,
		gitSymrefFlagConstant,
		originRemoteNameConstant,
		gitHeadReferenceConstant,
	}
}

func isNotExistError(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}

func isCaseOnlyRename(oldPath string, newPath string) bool {
	return strings.EqualFold(oldPath, newPath) && oldPath != newPath
}
