package audit_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	audit "github.com/temirov/git_scripts/internal/audit"
)

const (
	auditMissingRootsErrorMessageConstant = "no repository roots provided; specify --root or configure defaults"
	auditWhitespaceRootArgumentConstant   = "   "
	auditRootFlagNameConstant             = "--root"
	auditConfigurationMissingSubtestName  = "configuration_and_flags_missing"
	auditWhitespaceRootFlagSubtestName    = "flag_provided_without_roots"
)

func TestCommandBuilderDisplaysHelpWhenRootsMissing(testInstance *testing.T) {
	testInstance.Parallel()

	testCases := []struct {
		name          string
		configuration audit.CommandConfiguration
		arguments     []string
	}{
		{
			name:          auditConfigurationMissingSubtestName,
			configuration: audit.CommandConfiguration{},
			arguments:     []string{},
		},
		{
			name:          auditWhitespaceRootFlagSubtestName,
			configuration: audit.CommandConfiguration{},
			arguments: []string{
				auditRootFlagNameConstant,
				auditWhitespaceRootArgumentConstant,
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		testInstance.Run(testCase.name, func(subTest *testing.T) {
			subTest.Parallel()

			builder := audit.CommandBuilder{
				LoggerProvider:        func() *zap.Logger { return zap.NewNop() },
				ConfigurationProvider: func() audit.CommandConfiguration { return testCase.configuration },
			}

			command, buildError := builder.Build()
			require.NoError(subTest, buildError)

			command.SetContext(context.Background())
			command.SetArgs(testCase.arguments)

			outputBuffer := &strings.Builder{}
			command.SetOut(outputBuffer)
			command.SetErr(outputBuffer)

			executionError := command.Execute()
			require.Error(subTest, executionError)
			require.Equal(subTest, auditMissingRootsErrorMessageConstant, executionError.Error())
			require.Contains(subTest, outputBuffer.String(), command.UseLine())
		})
	}
}
