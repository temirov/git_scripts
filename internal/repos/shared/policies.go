package shared

import "strings"

// ConfirmationPolicy specifies how executors should handle user confirmations.
type ConfirmationPolicy int

const (
	// ConfirmationPrompt indicates the executor should prompt the user.
	ConfirmationPrompt ConfirmationPolicy = iota
	// ConfirmationAssumeYes indicates the executor should continue without prompting.
	ConfirmationAssumeYes
)

// ConfirmationPolicyFromBool converts legacy boolean flags into a policy.
func ConfirmationPolicyFromBool(assumeYes bool) ConfirmationPolicy {
	if assumeYes {
		return ConfirmationAssumeYes
	}
	return ConfirmationPrompt
}

// ShouldPrompt reports whether the executor must prompt the user.
func (policy ConfirmationPolicy) ShouldPrompt() bool {
	return policy != ConfirmationAssumeYes
}

// ShouldAssumeYes reports whether prompting can be skipped.
func (policy ConfirmationPolicy) ShouldAssumeYes() bool {
	return policy == ConfirmationAssumeYes
}

// CleanWorktreePolicy describes expectations for repository cleanliness.
type CleanWorktreePolicy int

const (
	// CleanWorktreeRequired enforces clean worktrees prior to executing an operation.
	CleanWorktreeRequired CleanWorktreePolicy = iota
	// CleanWorktreeOptional allows dirty worktrees.
	CleanWorktreeOptional
)

// CleanWorktreePolicyFromBool converts legacy boolean flags into a policy value.
func CleanWorktreePolicyFromBool(requireClean bool) CleanWorktreePolicy {
	if requireClean {
		return CleanWorktreeRequired
	}
	return CleanWorktreeOptional
}

// RequireClean reports whether a clean worktree is mandatory.
func (policy CleanWorktreePolicy) RequireClean() bool {
	return policy == CleanWorktreeRequired
}

// ParseOwnerRepositoryOptional normalizes and validates owner/repo tuples, returning nil when empty.
func ParseOwnerRepositoryOptional(raw string) (*OwnerRepository, error) {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, nil
	}
	ownerRepository, ownerRepositoryError := NewOwnerRepository(trimmed)
	if ownerRepositoryError != nil {
		return nil, ownerRepositoryError
	}
	return &ownerRepository, nil
}

// ParseOwnerSlugOptional normalizes owner slugs, returning nil when empty.
func ParseOwnerSlugOptional(raw string) (*OwnerSlug, error) {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, nil
	}
	slug, slugError := NewOwnerSlug(trimmed)
	if slugError != nil {
		return nil, slugError
	}
	return &slug, nil
}

// ParseRemoteURLOptional normalizes remote URLs, returning nil when empty.
func ParseRemoteURLOptional(raw string) (*RemoteURL, error) {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, nil
	}
	remoteURL, remoteURLError := NewRemoteURL(trimmed)
	if remoteURLError != nil {
		return nil, remoteURLError
	}
	return &remoteURL, nil
}
