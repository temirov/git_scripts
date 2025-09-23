package repos

import "github.com/spf13/cobra"

const (
	groupUseConstant      = "repos"
	groupShortDescription = "Manage collections of local repositories"
	groupLongDescription  = "repos groups subcommands that operate across multiple local repositories."
)

// CommandGroupBuilder assembles the repos command group.
type CommandGroupBuilder struct {
	LoggerProvider LoggerProvider
}

// Build constructs the repos command hierarchy.
func (builder *CommandGroupBuilder) Build() (*cobra.Command, error) {
	command := &cobra.Command{
		Use:   groupUseConstant,
		Short: groupShortDescription,
		Long:  groupLongDescription,
	}

	renameBuilder := RenameCommandBuilder{LoggerProvider: builder.LoggerProvider}
	renameCommand, renameError := renameBuilder.Build()
	if renameError == nil {
		command.AddCommand(renameCommand)
	}

	remotesBuilder := RemotesCommandBuilder{LoggerProvider: builder.LoggerProvider}
	remotesCommand, remotesError := remotesBuilder.Build()
	if remotesError == nil {
		command.AddCommand(remotesCommand)
	}

	protocolBuilder := ProtocolCommandBuilder{LoggerProvider: builder.LoggerProvider}
	protocolCommand, protocolError := protocolBuilder.Build()
	if protocolError == nil {
		command.AddCommand(protocolCommand)
	}

	return command, nil
}
