package shared_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/temirov/gix/internal/repos/shared"
)

func TestNewRepositoryPath(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{name: "valid_path", input: "/tmp/repo", expected: "/tmp/repo"},
		{name: "strips_whitespace", input: "   /tmp/repo  ", expected: "/tmp/repo"},
		{name: "rejects_empty", input: "", expectError: true},
		{name: "rejects_newline", input: "/tmp/repo\n", expectError: true},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := shared.NewRepositoryPath(testCase.input)
			if testCase.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, testCase.expected, result.String())
		})
	}
}

func TestNewOwnerSlug(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		input       string
		expect      string
		expectError bool
	}{
		{name: "valid_owner", input: "Temirov", expect: "Temirov"},
		{name: "trims_owner", input: "  org-name ", expect: "org-name"},
		{name: "rejects_empty", input: "  ", expectError: true},
		{name: "rejects_slash", input: "owner/name", expectError: true},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := shared.NewOwnerSlug(testCase.input)
			if testCase.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, testCase.expect, result.String())
		})
	}
}

func TestNewRepositoryName(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		input       string
		expect      string
		expectError bool
	}{
		{name: "valid_name", input: "gix", expect: "gix"},
		{name: "trims_name", input: " gix-cli ", expect: "gix-cli"},
		{name: "rejects_empty", input: "", expectError: true},
		{name: "rejects_slash", input: "owner/repo", expectError: true},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := shared.NewRepositoryName(testCase.input)
			if testCase.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, testCase.expect, result.String())
		})
	}
}

func TestNewOwnerRepository(t *testing.T) {
	t.Parallel()

	ownerRepo, err := shared.NewOwnerRepository("owner/repo")
	require.NoError(t, err)
	require.Equal(t, "owner", ownerRepo.Owner().String())
	require.Equal(t, "repo", ownerRepo.Repository().String())

	_, err = shared.NewOwnerRepository("invalid")
	require.Error(t, err)
}

func TestNewRemoteURL(t *testing.T) {
	t.Parallel()

	result, err := shared.NewRemoteURL("https://github.com/owner/repo.git")
	require.NoError(t, err)
	require.Equal(t, "https://github.com/owner/repo.git", result.String())

	_, err = shared.NewRemoteURL("  ")
	require.Error(t, err)
}

func TestNewRemoteName(t *testing.T) {
	t.Parallel()

	value, err := shared.NewRemoteName("origin")
	require.NoError(t, err)
	require.Equal(t, "origin", value.String())

	_, err = shared.NewRemoteName("invalid name")
	require.Error(t, err)
}

func TestNewBranchName(t *testing.T) {
	t.Parallel()

	name, err := shared.NewBranchName("feature/new-ui")
	require.NoError(t, err)
	require.Equal(t, "feature/new-ui", name.String())

	_, err = shared.NewBranchName("with space")
	require.Error(t, err)
}

func TestParseRemoteProtocol(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		input       string
		expect      shared.RemoteProtocol
		expectError bool
	}{
		{name: "git_protocol", input: "git", expect: shared.RemoteProtocolGit},
		{name: "ssh_protocol", input: "SSH", expect: shared.RemoteProtocolSSH},
		{name: "https_protocol", input: " https ", expect: shared.RemoteProtocolHTTPS},
		{name: "empty_defaults_other", input: " ", expect: shared.RemoteProtocolOther},
		{name: "unknown_error", input: "svn", expectError: true},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := shared.ParseRemoteProtocol(testCase.input)
			if testCase.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, testCase.expect, result)
		})
	}

	require.NoError(t, shared.RemoteProtocolSSH.Validate())
	require.Error(t, shared.RemoteProtocol("invalid").Validate())
}
