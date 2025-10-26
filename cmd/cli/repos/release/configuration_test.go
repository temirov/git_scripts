package release

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultCommandConfigurationProvidesRepositoryRoot(t *testing.T) {
	configuration := DefaultCommandConfiguration()
	require.Equal(t, []string{"."}, configuration.RepositoryRoots)

	sanitized := configuration.Sanitize()
	require.Equal(t, []string{"."}, sanitized.RepositoryRoots)
}
