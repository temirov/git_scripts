package repos

import (
	"github.com/spf13/cobra"

	"github.com/temirov/git_scripts/internal/execshell"
)

const (
	groupUseConstant      = "repos"
	groupShortDescription = "Manage collections of local repositories"
	groupLongDescription  = "repos groups subcommands that operate across multiple local repositories."
)

// CommandGroupBuilder assembles the repos command group.
type CommandGroupBuilder struct {
	LoggerProvider        LoggerProvider
	CommandEventsObserver execshell.CommandEventObserver
}

// Build constructs the repos command hierarchy.
func (builder *CommandGroupBuilder) Build() (*cobra.Command, error) {
	command := &cobra.Command{
		Use:   groupUseConstant,
		Short: groupShortDescription,
		Long:  groupLongDescription,
	}

	renameBuilder := RenameCommandBuilder{LoggerProvider: builder.LoggerProvider, CommandEventsObserver: builder.CommandEventsObserver}
	renameCommand, renameError := renameBuilder.Build()
	if renameError == nil {
		command.AddCommand(renameCommand)
	}

	remotesBuilder := RemotesCommandBuilder{LoggerProvider: builder.LoggerProvider, CommandEventsObserver: builder.CommandEventsObserver}
	remotesCommand, remotesError := remotesBuilder.Build()
	if remotesError == nil {
		command.AddCommand(remotesCommand)
	}

	protocolBuilder := ProtocolCommandBuilder{LoggerProvider: builder.LoggerProvider, CommandEventsObserver: builder.CommandEventsObserver}
	protocolCommand, protocolError := protocolBuilder.Build()
	if protocolError == nil {
		command.AddCommand(protocolCommand)
	}

	return command, nil
}
