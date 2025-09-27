package gitrepo

import (
	"fmt"
	"strings"
)

const (
	sshProtocolPrefixConstant           = "ssh://"
	sshUserDelimiterConstant            = "@"
	sshPathDelimiterConstant            = ":"
	httpsProtocolPrefixConstant         = "https://"
	gitUserPrefixConstant               = "git@"
	pathSeparatorConstant               = "/"
	gitSuffixConstant                   = ".git"
	remoteURLParseErrorTemplateConstant = "%s: %s"
	invalidRemoteURLMessageConstant     = "invalid remote url"
	unknownProtocolMessageConstant      = "unsupported remote protocol"
)

// RemoteProtocol enumerates supported git remote protocols.
type RemoteProtocol string

// Supported remote protocols.
const (
	RemoteProtocolSSH   RemoteProtocol = RemoteProtocol("ssh")
	RemoteProtocolHTTPS RemoteProtocol = RemoteProtocol("https")
)

// RemoteURL represents a structured git remote URL.
type RemoteURL struct {
	Protocol   RemoteProtocol
	Host       string
	Owner      string
	Repository string
}

// RemoteURLParseError indicates a remote string could not be parsed.
type RemoteURLParseError struct {
	Input   string
	Message string
}

// Error describes the parse failure.
func (parseError RemoteURLParseError) Error() string {
	return fmt.Sprintf(remoteURLParseErrorTemplateConstant, parseError.Input, parseError.Message)
}

// UnsupportedProtocolError indicates the provided protocol cannot be formatted.
type UnsupportedProtocolError struct {
	Protocol RemoteProtocol
}

// Error describes the unsupported protocol.
func (protocolError UnsupportedProtocolError) Error() string {
	return fmt.Sprintf(remoteURLParseErrorTemplateConstant, protocolError.Protocol, unknownProtocolMessageConstant)
}

// ParseRemoteURL converts a textual remote URL into a structured representation.
func ParseRemoteURL(remote string) (RemoteURL, error) {
	trimmedRemote := strings.TrimSpace(remote)
	if len(trimmedRemote) == 0 {
		return RemoteURL{}, RemoteURLParseError{Input: remote, Message: requiredValueMessageConstant}
	}

	if strings.HasPrefix(trimmedRemote, sshProtocolPrefixConstant) {
		return parseSSHRemote(strings.TrimPrefix(trimmedRemote, sshProtocolPrefixConstant))
	}
	if strings.HasPrefix(trimmedRemote, gitUserPrefixConstant) {
		return parseSSHRemote(trimmedRemote)
	}
	if strings.HasPrefix(trimmedRemote, httpsProtocolPrefixConstant) {
		return parseHTTPSRemote(strings.TrimPrefix(trimmedRemote, httpsProtocolPrefixConstant))
	}

	return RemoteURL{}, RemoteURLParseError{Input: remote, Message: invalidRemoteURLMessageConstant}
}

func parseSSHRemote(remote string) (RemoteURL, error) {
	userSplitIndex := strings.Index(remote, sshUserDelimiterConstant)
	if userSplitIndex == -1 {
		return RemoteURL{}, RemoteURLParseError{Input: remote, Message: invalidRemoteURLMessageConstant}
	}
	hostAndPath := remote[userSplitIndex+1:]
	pathSplitIndex := strings.Index(hostAndPath, sshPathDelimiterConstant)
	var host string
	var path string
	if pathSplitIndex == -1 {
		slashIndex := strings.Index(hostAndPath, pathSeparatorConstant)
		if slashIndex == -1 {
			return RemoteURL{}, RemoteURLParseError{Input: remote, Message: invalidRemoteURLMessageConstant}
		}
		host = hostAndPath[:slashIndex]
		path = hostAndPath[slashIndex+1:]
	} else {
		host = hostAndPath[:pathSplitIndex]
		path = hostAndPath[pathSplitIndex+1:]
	}
	owner, repository, parseError := splitOwnerAndRepository(path)
	if parseError != nil {
		return RemoteURL{}, parseError
	}
	return RemoteURL{Protocol: RemoteProtocolSSH, Host: host, Owner: owner, Repository: repository}, nil
}

func parseHTTPSRemote(remote string) (RemoteURL, error) {
	pathComponents := strings.Split(remote, pathSeparatorConstant)
	if len(pathComponents) < 3 {
		return RemoteURL{}, RemoteURLParseError{Input: remote, Message: invalidRemoteURLMessageConstant}
	}
	host := pathComponents[0]
	owner := pathComponents[1]
	repository, parseError := normalizeRepositoryName(strings.Join(pathComponents[2:], pathSeparatorConstant))
	if parseError != nil {
		return RemoteURL{}, parseError
	}
	return RemoteURL{Protocol: RemoteProtocolHTTPS, Host: host, Owner: owner, Repository: repository}, nil
}

func splitOwnerAndRepository(path string) (string, string, error) {
	segments := strings.Split(path, pathSeparatorConstant)
	if len(segments) != 2 {
		return "", "", RemoteURLParseError{Input: path, Message: invalidRemoteURLMessageConstant}
	}
	repository, parseError := normalizeRepositoryName(segments[1])
	if parseError != nil {
		return "", "", parseError
	}
	return segments[0], repository, nil
}

func normalizeRepositoryName(repository string) (string, error) {
	trimmed := strings.TrimSuffix(repository, gitSuffixConstant)
	if len(trimmed) == 0 {
		return "", RemoteURLParseError{Input: repository, Message: invalidRemoteURLMessageConstant}
	}
	return trimmed, nil
}

// FormatRemoteURL creates a textual remote URL from a structured representation.
func FormatRemoteURL(remote RemoteURL) (string, error) {
	if len(strings.TrimSpace(remote.Host)) == 0 {
		return "", RemoteURLParseError{Input: remote.Host, Message: requiredValueMessageConstant}
	}
	if len(strings.TrimSpace(remote.Owner)) == 0 {
		return "", RemoteURLParseError{Input: remote.Owner, Message: requiredValueMessageConstant}
	}
	if len(strings.TrimSpace(remote.Repository)) == 0 {
		return "", RemoteURLParseError{Input: remote.Repository, Message: requiredValueMessageConstant}
	}

	switch remote.Protocol {
	case RemoteProtocolSSH:
		return fmt.Sprintf("%s%s%s%s%s%s", gitUserPrefixConstant, remote.Host, sshPathDelimiterConstant, remote.Owner, pathSeparatorConstant, remote.Repository+gitSuffixConstant), nil
	case RemoteProtocolHTTPS:
		return fmt.Sprintf("%s%s%s%s%s%s%s", httpsProtocolPrefixConstant, remote.Host, pathSeparatorConstant, remote.Owner, pathSeparatorConstant, remote.Repository, gitSuffixConstant), nil
	default:
		return "", UnsupportedProtocolError{Protocol: remote.Protocol}
	}
}
