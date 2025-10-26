package commit

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/temirov/gix/internal/commitmsg"
	"github.com/temirov/gix/internal/repos/dependencies"
	"github.com/temirov/gix/internal/repos/shared"
	flagutils "github.com/temirov/gix/internal/utils/flags"
	rootutils "github.com/temirov/gix/internal/utils/roots"
	"github.com/temirov/gix/pkg/llm"
)

const (
	messageCommandUseName          = "message"
	messageCommandShortDescription = "Generate a commit message from local changes"
	diffSourceFlagName             = "diff-source"
	diffSourceFlagUsage            = "Diff source to summarize (staged|worktree)"
	maxTokensFlagName              = "max-tokens"
	maxTokensFlagUsage             = "Override the maximum completion tokens"
	temperatureFlagName            = "temperature"
	temperatureFlagUsage           = "Override the sampling temperature (0-2)"
	modelFlagName                  = "model"
	modelFlagUsage                 = "Override the model identifier"
	baseURLFlagName                = "base-url"
	baseURLFlagUsage               = "Override the LLM base URL"
	apiKeyEnvFlagName              = "api-key-env"
	apiKeyEnvFlagUsage             = "Environment variable providing the LLM API key"
	timeoutFlagName                = "timeout-seconds"
	timeoutFlagUsage               = "Override the LLM request timeout in seconds"
)

// ClientFactory builds chat clients from configuration.
type ClientFactory func(config llm.Config) (commitmsg.ChatClient, error)

// MessageCommandBuilder assembles the commit message command.
type MessageCommandBuilder struct {
	LoggerProvider               LoggerProvider
	GitExecutor                  shared.GitExecutor
	ConfigurationProvider        func() MessageConfiguration
	HumanReadableLoggingProvider func() bool
	ClientFactory                ClientFactory
}

// Build constructs the commit message command.
func (builder *MessageCommandBuilder) Build() (*cobra.Command, error) {
	command := &cobra.Command{
		Use:   messageCommandUseName,
		Short: messageCommandShortDescription,
		Args:  cobra.NoArgs,
		RunE:  builder.run,
	}

	command.Flags().String(diffSourceFlagName, "", diffSourceFlagUsage)
	command.Flags().Int(maxTokensFlagName, 0, maxTokensFlagUsage)
	command.Flags().Float64(temperatureFlagName, 0, temperatureFlagUsage)
	command.Flags().String(modelFlagName, "", modelFlagUsage)
	command.Flags().String(baseURLFlagName, "", baseURLFlagUsage)
	command.Flags().String(apiKeyEnvFlagName, "", apiKeyEnvFlagUsage)
	command.Flags().Int(timeoutFlagName, 0, timeoutFlagUsage)

	return command, nil
}

func (builder *MessageCommandBuilder) run(command *cobra.Command, arguments []string) error {
	configuration := builder.resolveConfiguration()

	executionFlags, _ := flagutils.ResolveExecutionFlags(command)
	dryRun := executionFlags.DryRun
	if executionFlags.DryRunSet {
		dryRun = executionFlags.DryRun
	}

	repositoryPath, rootError := selectRepositoryRoot(command, configuration)
	if rootError != nil {
		return rootError
	}

	diffSource, sourceError := resolveDiffSource(command, configuration)
	if sourceError != nil {
		return sourceError
	}

	maxTokens, maxTokensError := resolveMaxTokens(command, configuration)
	if maxTokensError != nil {
		return maxTokensError
	}

	temperaturePointer, temperatureError := resolveTemperature(command, configuration)
	if temperatureError != nil {
		return temperatureError
	}

	modelIdentifier := configuration.Model
	if command != nil {
		if flagValue, flagError := command.Flags().GetString(modelFlagName); flagError == nil && command.Flags().Changed(modelFlagName) {
			modelIdentifier = strings.TrimSpace(flagValue)
		}
	}
	if modelIdentifier == "" {
		return errors.New("model identifier must be provided via configuration or --model")
	}

	baseURL := configuration.BaseURL
	if command != nil {
		if flagValue, flagError := command.Flags().GetString(baseURLFlagName); flagError == nil && command.Flags().Changed(baseURLFlagName) {
			baseURL = strings.TrimSpace(flagValue)
		}
	}

	apiKeyEnv := configuration.APIKeyEnv
	if command != nil {
		if flagValue, flagError := command.Flags().GetString(apiKeyEnvFlagName); flagError == nil && command.Flags().Changed(apiKeyEnvFlagName) {
			apiKeyEnv = strings.TrimSpace(flagValue)
		}
	}
	if apiKeyEnv == "" {
		apiKeyEnv = defaultAPIKeyEnvironment
	}
	apiKey, apiKeyPresent := lookupEnvironmentValue(apiKeyEnv)
	if !apiKeyPresent || apiKey == "" {
		return fmt.Errorf("environment variable %s must be set with an API key", apiKeyEnv)
	}

	timeout := time.Duration(configuration.TimeoutSeconds) * time.Second
	if command != nil {
		if flagValue, flagError := command.Flags().GetInt(timeoutFlagName); flagError == nil && command.Flags().Changed(timeoutFlagName) {
			if flagValue <= 0 {
				return errors.New("timeout-seconds must be positive")
			}
			timeout = time.Duration(flagValue) * time.Second
		}
	}

	logger := resolveLogger(builder.LoggerProvider)
	humanReadable := false
	if builder.HumanReadableLoggingProvider != nil {
		humanReadable = builder.HumanReadableLoggingProvider()
	}
	gitExecutor, executorError := dependencies.ResolveGitExecutor(builder.GitExecutor, logger, humanReadable)
	if executorError != nil {
		return executorError
	}

	clientFactory := builder.ClientFactory
	if clientFactory == nil {
		clientFactory = func(config llm.Config) (commitmsg.ChatClient, error) {
			return llm.NewClient(config)
		}
	}

	client, clientError := clientFactory(llm.Config{
		BaseURL:             baseURL,
		APIKey:              apiKey,
		Model:               modelIdentifier,
		MaxCompletionTokens: configuration.MaxTokens,
		Temperature:         configuration.Temperature,
		RequestTimeout:      timeout,
	})
	if clientError != nil {
		return clientError
	}

	generator := commitmsg.Generator{
		GitExecutor: gitExecutor,
		Client:      client,
		Logger:      logger,
	}

	request, buildError := generator.BuildRequest(command.Context(), commitmsg.Options{
		RepositoryPath: repositoryPath,
		Source:         diffSource,
		MaxTokens:      maxTokens,
		Temperature:    temperaturePointer,
	})
	if buildError != nil {
		return buildError
	}

	if dryRun {
		output := command.OutOrStdout()
		fmt.Fprintln(output, request.Messages[0].Content)
		fmt.Fprintln(output)
		fmt.Fprintln(output, request.Messages[1].Content)
		return nil
	}

	response, chatError := client.Chat(command.Context(), request)
	if chatError != nil {
		return chatError
	}

	fmt.Fprintln(command.OutOrStdout(), strings.TrimSpace(response))
	return nil
}

func (builder *MessageCommandBuilder) resolveConfiguration() MessageConfiguration {
	if builder.ConfigurationProvider == nil {
		return DefaultMessageConfiguration()
	}
	return builder.ConfigurationProvider().Sanitize()
}

func selectRepositoryRoot(command *cobra.Command, configuration MessageConfiguration) (string, error) {
	flagRoots, flagError := rootutils.FlagValues(command)
	if flagError != nil {
		return "", flagError
	}
	flagRoots = rootutils.SanitizeConfigured(flagRoots)
	configurationRoots := rootutils.SanitizeConfigured(configuration.Roots)

	var roots []string
	switch {
	case len(flagRoots) > 0:
		roots = flagRoots
	case len(configurationRoots) > 0:
		roots = configurationRoots
	default:
		roots = []string{"."}
	}

	if len(roots) != 1 {
		return "", fmt.Errorf("commit message command requires exactly one repository root (received %d)", len(roots))
	}

	trimmed := strings.TrimSpace(roots[0])
	if trimmed == "" {
		return "", errors.New("repository root cannot be empty")
	}
	return trimmed, nil
}

func resolveDiffSource(command *cobra.Command, configuration MessageConfiguration) (commitmsg.DiffSource, error) {
	value := strings.TrimSpace(configuration.DiffSource)
	if command != nil {
		if flagValue, flagError := command.Flags().GetString(diffSourceFlagName); flagError == nil && command.Flags().Changed(diffSourceFlagName) {
			value = strings.TrimSpace(flagValue)
		}
	}
	value = strings.ToLower(value)
	switch value {
	case "", string(commitmsg.DiffSourceStaged):
		return commitmsg.DiffSourceStaged, nil
	case string(commitmsg.DiffSourceWorktree):
		return commitmsg.DiffSourceWorktree, nil
	default:
		return "", fmt.Errorf("unsupported diff source %q (expected staged or worktree)", value)
	}
}

func resolveMaxTokens(command *cobra.Command, configuration MessageConfiguration) (int, error) {
	maxTokens := configuration.MaxTokens
	if command != nil {
		if flagValue, flagError := command.Flags().GetInt(maxTokensFlagName); flagError == nil && command.Flags().Changed(maxTokensFlagName) {
			if flagValue < 0 {
				return 0, errors.New("max-tokens must be zero or positive")
			}
			maxTokens = flagValue
		}
	}
	return maxTokens, nil
}

func resolveTemperature(command *cobra.Command, configuration MessageConfiguration) (*float64, error) {
	if command != nil {
		if flagValue, flagError := command.Flags().GetFloat64(temperatureFlagName); flagError == nil && command.Flags().Changed(temperatureFlagName) {
			if flagValue < 0 {
				return nil, errors.New("temperature cannot be negative")
			}
			return &flagValue, nil
		}
	}
	if configuration.Temperature != 0 {
		value := configuration.Temperature
		if value < 0 {
			return nil, errors.New("temperature cannot be negative")
		}
		return &value, nil
	}
	return nil, nil
}
