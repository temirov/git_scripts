package audit

const (
	commandUseConstant                          = "audit"
	commandShortDescription                     = "Audit and reconcile local GitHub repositories"
	commandLongDescription                      = "Scans git repositories for GitHub remotes and produces audit reports or applies reconciliation actions."
	flagRootNameConstant                        = "roots"
	flagRootDescriptionConstant                 = "Repository roots to scan (repeatable; nested paths ignored)"
	missingRootsErrorMessageConstant            = "no repository roots provided; specify --roots or configure defaults"
	normalizeRepositoryPathErrorMessageConstant = "failed to normalize repository path"
	debugDiscoveredTemplate                     = "DEBUG: discovered %d candidate repos under: %s\n"
	debugCheckingTemplate                       = "DEBUG: checking %s\n"
	csvHeaderFinalRepository                    = "final_github_repo"
	csvHeaderFolderName                         = "folder_name"
	csvHeaderNameMatches                        = "name_matches"
	csvHeaderRemoteDefault                      = "remote_default_branch"
	csvHeaderLocalBranch                        = "local_branch"
	csvHeaderInSync                             = "in_sync"
	csvHeaderRemoteProtocol                     = "remote_protocol"
	csvHeaderOriginCanonical                    = "origin_matches_canonical"
	gitIsInsideWorkTreeFlagConstant             = "--is-inside-work-tree"
	gitTrueOutputConstant                       = "true"
	notGitHubRemoteMessageConstant              = "not a github remote"
)
