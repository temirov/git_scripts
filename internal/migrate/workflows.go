package migrate

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.uber.org/zap"
)

const (
	workflowsDirectoryMissingMessageConstant = "workflows directory not found; skipping rewrite"
	workflowFileFieldNameConstant            = "workflow_file"
	workflowsRootFieldNameConstant           = "workflows_root"
	rewriteLogMessageConstant                = "Rewriting workflow file"
	skipRewriteLogMessageConstant            = "No rewrites required"
	rewriteCompletionLogMessageConstant      = "Workflow rewrite completed"
	migratedWorkflowFilesFieldConstant       = "migrated_workflows"
	mainBranchWordBoundaryTemplateConstant   = `\b%s\b`
	inlineBranchesPatternTemplateConstant    = `(?m)(\s*branches\s*:\s*\[\s*)(["']?)(%s)(["']?)(\s*\])`
	listBranchesPatternTemplateConstant      = `(?m)^(\s*-\s*)(["']?)(%s)(["']?)\s*$`
	yamlExtensionConstant                    = ".yaml"
	ymlExtensionConstant                     = ".yml"
	inspectWorkflowsErrorTemplateConstant    = "unable to inspect workflows directory: %w"
	workflowsNotDirectoryTemplateConstant    = "workflows path is not a directory: %s"
	readWorkflowErrorTemplateConstant        = "unable to read workflow file %s: %w"
	statWorkflowErrorTemplateConstant        = "unable to stat workflow file %s: %w"
	writeWorkflowErrorTemplateConstant       = "unable to write workflow file %s: %w"
)

// WorkflowRewriter updates GitHub Actions workflows to target the desired branch.
type WorkflowRewriter struct {
	logger *zap.Logger
}

// NewWorkflowRewriter constructs a WorkflowRewriter.
func NewWorkflowRewriter(logger *zap.Logger) *WorkflowRewriter {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &WorkflowRewriter{logger: logger}
}

// Rewrite applies branch replacements across workflow files.
func (rewriter *WorkflowRewriter) Rewrite(_ context.Context, config WorkflowRewriteConfig) (WorkflowOutcome, error) {
	workflowsOutcome := WorkflowOutcome{UpdatedFiles: []string{}, RemainingMainReferences: false}

	workflowsRoot := filepath.Join(config.RepositoryPath, config.WorkflowsDirectory)
	directoryInfo, statError := os.Stat(workflowsRoot)
	if statError != nil {
		if errors.Is(statError, fs.ErrNotExist) {
			rewriter.logger.Info(workflowsDirectoryMissingMessageConstant, zap.String(workflowsRootFieldNameConstant, workflowsRoot))
			return workflowsOutcome, nil
		}
		return WorkflowOutcome{}, fmt.Errorf(inspectWorkflowsErrorTemplateConstant, statError)
	}

	if !directoryInfo.IsDir() {
		return WorkflowOutcome{}, fmt.Errorf(workflowsNotDirectoryTemplateConstant, workflowsRoot)
	}

	sourceBranch := string(config.SourceBranch)
	targetBranch := string(config.TargetBranch)

	inlinePattern := regexp.MustCompile(fmt.Sprintf(inlineBranchesPatternTemplateConstant, regexp.QuoteMeta(sourceBranch)))
	listPattern := regexp.MustCompile(fmt.Sprintf(listBranchesPatternTemplateConstant, regexp.QuoteMeta(sourceBranch)))
	wordPattern := regexp.MustCompile(fmt.Sprintf(mainBranchWordBoundaryTemplateConstant, regexp.QuoteMeta(sourceBranch)))

	walkError := filepath.WalkDir(workflowsRoot, func(path string, directoryEntry fs.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		if directoryEntry.IsDir() {
			return nil
		}
		if !isWorkflowFile(path) {
			return nil
		}

		fileOutcome, processingError := rewriter.processWorkflowFile(path, inlinePattern, listPattern, wordPattern, targetBranch)
		if processingError != nil {
			return processingError
		}

		if fileOutcome.updated {
			relativePath, relativeError := filepath.Rel(config.RepositoryPath, path)
			if relativeError != nil {
				relativePath = path
			}
			workflowsOutcome.UpdatedFiles = append(workflowsOutcome.UpdatedFiles, relativePath)
		}

		if fileOutcome.containsSource {
			workflowsOutcome.RemainingMainReferences = true
		}

		return nil
	})

	if walkError != nil {
		return WorkflowOutcome{}, walkError
	}

	rewriter.logger.Info(rewriteCompletionLogMessageConstant,
		zap.String(workflowsRootFieldNameConstant, workflowsRoot),
		zap.Strings(migratedWorkflowFilesFieldConstant, workflowsOutcome.UpdatedFiles),
	)

	return workflowsOutcome, nil
}

type workflowFileOutcome struct {
	updated        bool
	containsSource bool
}

func (rewriter *WorkflowRewriter) processWorkflowFile(filePath string, inlinePattern *regexp.Regexp, listPattern *regexp.Regexp, wordPattern *regexp.Regexp, targetBranch string) (workflowFileOutcome, error) {
	fileContent, readError := os.ReadFile(filePath)
	if readError != nil {
		return workflowFileOutcome{}, fmt.Errorf(readWorkflowErrorTemplateConstant, filePath, readError)
	}

	inlineReplacement := fmt.Sprintf("${1}${2}%s${4}${5}", targetBranch)
	listReplacement := fmt.Sprintf("${1}${2}%s${4}", targetBranch)
	updatedContent := inlinePattern.ReplaceAllString(string(fileContent), inlineReplacement)
	updatedContent = listPattern.ReplaceAllString(updatedContent, listReplacement)

	containsSource := wordPattern.MatchString(updatedContent)
	updated := updatedContent != string(fileContent)

	if !updated {
		rewriter.logger.Debug(skipRewriteLogMessageConstant, zap.String(workflowFileFieldNameConstant, filePath))
		return workflowFileOutcome{updated: false, containsSource: containsSource}, nil
	}

	fileInfo, infoError := os.Stat(filePath)
	if infoError != nil {
		return workflowFileOutcome{}, fmt.Errorf(statWorkflowErrorTemplateConstant, filePath, infoError)
	}

	writeError := os.WriteFile(filePath, []byte(updatedContent), fileInfo.Mode().Perm())
	if writeError != nil {
		return workflowFileOutcome{}, fmt.Errorf(writeWorkflowErrorTemplateConstant, filePath, writeError)
	}

	rewriter.logger.Info(rewriteLogMessageConstant, zap.String(workflowFileFieldNameConstant, filePath))

	return workflowFileOutcome{updated: true, containsSource: containsSource}, nil
}

func isWorkflowFile(path string) bool {
	lowerPath := strings.ToLower(path)
	return strings.HasSuffix(lowerPath, yamlExtensionConstant) || strings.HasSuffix(lowerPath, ymlExtensionConstant)
}
