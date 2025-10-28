package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/temirov/gix/internal/releases"
	"github.com/temirov/gix/internal/repos/shared"
)

const (
	taskActionCanonicalRemote    = "repo.remote.update"
	taskActionProtocolConversion = "repo.remote.convert-protocol"
	taskActionRenameDirectories  = "repo.folder.rename"
	taskActionBranchDefault      = "branch.default"
	taskActionReleaseTag         = "repo.release.tag"

	releaseActionMessageTemplate = "RELEASED: %s -> %s"
)

type taskActionHandlerFunc func(ctx context.Context, environment *Environment, repository *RepositoryState, parameters map[string]any) error

type taskActionExecutor struct {
	environment *Environment
	handlers    map[string]taskActionHandlerFunc
}

func newTaskActionExecutor(environment *Environment) taskActionExecutor {
	handlers := map[string]taskActionHandlerFunc{
		taskActionCanonicalRemote:    handleCanonicalRemoteAction,
		taskActionProtocolConversion: handleProtocolConversionAction,
		taskActionRenameDirectories:  handleRenameDirectoriesAction,
		taskActionBranchDefault:      handleBranchDefaultAction,
		taskActionReleaseTag:         handleReleaseTagAction,
	}
	return taskActionExecutor{environment: environment, handlers: handlers}
}

func (executor taskActionExecutor) execute(ctx context.Context, repository *RepositoryState, action taskAction) error {
	if executor.environment == nil || repository == nil {
		return nil
	}

	normalizedType := strings.ToLower(strings.TrimSpace(action.actionType))
	if len(normalizedType) == 0 {
		return nil
	}

	handler, exists := executor.handlers[normalizedType]
	if !exists {
		return fmt.Errorf("unsupported task action %s", action.actionType)
	}

	return handler(ctx, executor.environment, repository, action.parameters)
}

func handleCanonicalRemoteAction(ctx context.Context, environment *Environment, repository *RepositoryState, parameters map[string]any) error {
	reader := newOptionReader(parameters)
	ownerConstraint, _, ownerError := reader.stringValue("owner")
	if ownerError != nil {
		return ownerError
	}

	operation := &CanonicalRemoteOperation{OwnerConstraint: strings.TrimSpace(ownerConstraint)}
	state := &State{Repositories: []*RepositoryState{repository}}
	return operation.Execute(ctx, environment, state)
}

func handleProtocolConversionAction(ctx context.Context, environment *Environment, repository *RepositoryState, parameters map[string]any) error {
	reader := newOptionReader(parameters)

	targetValue, targetExists, targetError := reader.stringValue("to")
	if targetError != nil {
		return targetError
	}
	if !targetExists || len(targetValue) == 0 {
		return errors.New("protocol conversion action requires 'to'")
	}

	targetProtocol, parseTargetError := parseProtocolValue(targetValue)
	if parseTargetError != nil {
		return parseTargetError
	}

	fromProtocol := shared.RemoteProtocol(strings.TrimSpace(string(repository.Inspection.RemoteProtocol)))
	sourceValue, sourceExists, sourceError := reader.stringValue("from")
	if sourceError != nil {
		return sourceError
	}
	if sourceExists && len(sourceValue) > 0 {
		parsedSource, parseSourceError := parseProtocolValue(sourceValue)
		if parseSourceError != nil {
			return parseSourceError
		}
		fromProtocol = parsedSource
	}

	operation := &ProtocolConversionOperation{FromProtocol: fromProtocol, ToProtocol: targetProtocol}
	state := &State{Repositories: []*RepositoryState{repository}}
	return operation.Execute(ctx, environment, state)
}

func handleRenameDirectoriesAction(ctx context.Context, environment *Environment, repository *RepositoryState, parameters map[string]any) error {
	reader := newOptionReader(parameters)

	requireClean := true
	requireCleanExplicit := false
	if value, exists, err := reader.boolValue("require_clean"); err != nil {
		return err
	} else if exists {
		requireClean = value
		requireCleanExplicit = true
	}

	includeOwner := false
	if value, exists, err := reader.boolValue("include_owner"); err != nil {
		return err
	} else if exists {
		includeOwner = value
	}

	if requireClean && repository != nil && repository.HasNestedRepositories && repository.InitialCleanWorktree {
		requireClean = false
	}

	operation := &RenameOperation{RequireCleanWorktree: requireClean, IncludeOwner: includeOwner, requireCleanExplicit: requireCleanExplicit}
	state := &State{Repositories: []*RepositoryState{repository}}
	return operation.Execute(ctx, environment, state)
}

func handleBranchDefaultAction(ctx context.Context, environment *Environment, repository *RepositoryState, parameters map[string]any) error {
	reader := newOptionReader(parameters)

	targetBranchValue, _, targetBranchError := reader.stringValue("target")
	if targetBranchError != nil {
		return targetBranchError
	}

	sourceBranchValue, _, sourceBranchError := reader.stringValue("source")
	if sourceBranchError != nil {
		return sourceBranchError
	}

	remoteNameValue, remoteNameExists, remoteNameError := reader.stringValue("remote")
	if remoteNameError != nil {
		return remoteNameError
	}
	remoteName := defaultMigrationRemoteNameConstant
	if remoteNameExists && len(remoteNameValue) > 0 {
		remoteName = remoteNameValue
	}

	pushToRemote := true
	if value, exists, err := reader.boolValue("push"); err != nil {
		return err
	} else if exists {
		pushToRemote = value
	}

	deleteSource := false
	if value, exists, err := reader.boolValue("delete_source_branch"); err != nil {
		return err
	} else if exists {
		deleteSource = value
	}

	target := BranchMigrationTarget{
		RemoteName:         remoteName,
		SourceBranch:       sourceBranchValue,
		TargetBranch:       targetBranchValue,
		PushToRemote:       pushToRemote,
		DeleteSourceBranch: deleteSource,
	}

	operation := &BranchMigrationOperation{Targets: []BranchMigrationTarget{target}}
	state := &State{Repositories: []*RepositoryState{repository}}
	return operation.Execute(ctx, environment, state)
}

func handleReleaseTagAction(ctx context.Context, environment *Environment, repository *RepositoryState, parameters map[string]any) error {
	if environment == nil || repository == nil {
		return nil
	}

	reader := newOptionReader(parameters)

	tagValue, tagExists, tagError := reader.stringValue("tag")
	if tagError != nil {
		return tagError
	}
	if !tagExists || len(tagValue) == 0 {
		return errors.New("release action requires 'tag'")
	}

	messageValue, _, messageError := reader.stringValue("message")
	if messageError != nil {
		return messageError
	}

	remoteValue, _, remoteError := reader.stringValue("remote")
	if remoteError != nil {
		return remoteError
	}

	service, serviceError := releases.NewService(releases.ServiceDependencies{GitExecutor: environment.GitExecutor})
	if serviceError != nil {
		return serviceError
	}

	result, releaseError := service.Release(ctx, releases.Options{
		RepositoryPath: repository.Path,
		TagName:        tagValue,
		Message:        messageValue,
		RemoteName:     remoteValue,
		DryRun:         environment.DryRun,
	})
	if releaseError != nil {
		return releaseError
	}

	if environment.Output != nil {
		fmt.Fprintf(environment.Output, releaseActionMessageTemplate+"\n", result.RepositoryPath, result.TagName)
	}

	return nil
}
