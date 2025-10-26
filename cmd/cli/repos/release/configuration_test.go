package release

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultCommandConfigurationProvidesRepositoryRoot(t *testing.T) {
	configuration := DefaultCommandConfiguration()
	require.Empty(t, configuration.RepositoryRoots)

	sanitized := configuration.Sanitize()
	require.Empty(t, sanitized.RepositoryRoots)
}
