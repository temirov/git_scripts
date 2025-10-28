package cli

import (
	"context"

	"github.com/temirov/gix/internal/workflow"
)

// TaskRunnerExecutor coordinates workflow task execution.
type TaskRunnerExecutor interface {
	Run(ctx context.Context, roots []string, definitions []workflow.TaskDefinition, options workflow.RuntimeOptions) error
}

type taskRunnerAdapter struct {
	runner workflow.TaskRunner
}

func (adapter taskRunnerAdapter) Run(ctx context.Context, roots []string, definitions []workflow.TaskDefinition, options workflow.RuntimeOptions) error {
	return adapter.runner.Run(ctx, roots, definitions, options)
}

func resolveTaskRunner(factory func(workflow.Dependencies) TaskRunnerExecutor, dependencies workflow.Dependencies) TaskRunnerExecutor {
	if factory != nil {
		if executor := factory(dependencies); executor != nil {
			return executor
		}
	}
	return taskRunnerAdapter{runner: workflow.NewTaskRunner(dependencies)}
}
