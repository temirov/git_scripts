package flags

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestBindRepositoryFlagsUsesDefaultsAndParsesValues(t *testing.T) {
	command := &cobra.Command{}

	values := BindRepositoryFlags(command, RepositoryFlagValues{Owner: "default-owner", Name: "default-name"}, RepositoryFlagDefinitions{
		Owner: RepositoryFlagDefinition{Name: "repository-owner", Usage: "Repository owner", Enabled: true},
		Name:  RepositoryFlagDefinition{Name: "repository-name", Usage: "Repository name", Enabled: true},
	})

	require.NotNil(t, values)
	require.Equal(t, "default-owner", values.Owner)
	require.Equal(t, "default-name", values.Name)

	parseError := command.ParseFlags([]string{"--repository-owner", "custom", "--repository-name", "sample"})
	require.NoError(t, parseError)
	require.Equal(t, "custom", values.Owner)
	require.Equal(t, "sample", values.Name)
}

func TestBindBranchFlagsUsesDefaultsAndParsesValues(t *testing.T) {
	command := &cobra.Command{}

	values := BindBranchFlags(command, BranchFlagValues{Name: "main"}, BranchFlagDefinition{Name: "branch", Usage: "Branch name", Enabled: true})

	require.NotNil(t, values)
	require.Equal(t, "main", values.Name)

	parseError := command.ParseFlags([]string{"--branch", "feature"})
	require.NoError(t, parseError)
	require.Equal(t, "feature", values.Name)
}

func TestBindRootFlagsUsesDefaultsAndParsesValues(t *testing.T) {
	command := &cobra.Command{}

	values := BindRootFlags(command, RootFlagValues{Roots: []string{"/tmp/default"}}, RootFlagDefinition{Enabled: true})

	require.NotNil(t, values)
	require.Equal(t, []string{"/tmp/default"}, values.Roots)

	parseError := command.ParseFlags([]string{"--" + DefaultRootFlagName, "/workspace", "--" + DefaultRootFlagName, "/projects"})
	require.NoError(t, parseError)
	require.Equal(t, []string{"/workspace", "/projects"}, values.Roots)
}
