package tests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	packagesIntegrationOwnerConstant                    = "integration-org"
	packagesIntegrationPackageConstant                  = "tooling"
	packagesIntegrationOwnerTypeConstant                = "org"
	packagesIntegrationTokenEnvNameConstant             = "PACKAGES_TOKEN"
	packagesIntegrationTokenReferenceConstant           = "env:PACKAGES_TOKEN"
	packagesIntegrationTokenValueConstant               = "packages-token-value"
	packagesIntegrationConfigFileNameConstant           = "config.yaml"
	packagesIntegrationConfigTemplateConstant           = "common:\n  log_level: error\ncli:\n  packages:\n    purge:\n      owner: %s\n      package: %s\n      owner_type: %s\n      token_source: %s\n      dry_run: %t\n      service_base_url: %s\n      page_size: %d\n"
	packagesIntegrationSubtestNameTemplateConstant      = "%d_%s"
	packagesIntegrationRunSubcommandConstant            = "run"
	packagesIntegrationModulePathConstant               = "."
	packagesIntegrationConfigFlagTemplateConstant       = "--config=%s"
	packagesIntegrationPackagesPurgeCommandNameConstant = "repo-packages-purge"
	packagesIntegrationCommandTimeout                   = 10 * time.Second
	packagesIntegrationPageSizeConstant                 = 3
	packagesIntegrationTaggedVersionIDConstant          = 101
	packagesIntegrationFirstUntaggedVersionIDConstant   = 202
	packagesIntegrationSecondUntaggedVersionIDConstant  = 303
	packagesIntegrationVersionsResponseTemplateConstant = `[
{"id":%d,"metadata":{"container":{"tags":["stable"]}}},
{"id":%d,"metadata":{"container":{"tags":[]}}},
{"id":%d,"metadata":{"container":{"tags":[]}}}
]`
	packagesIntegrationVersionsPathTemplateConstant  = "/orgs/%s/packages/container/%s/versions"
	packagesIntegrationDeletePathTemplateConstant    = "/orgs/%s/packages/container/%s/versions/%d"
	packagesIntegrationAuthorizationTemplateConstant = "Bearer %s"
)

type packagesIntegrationListRequest struct {
	path    string
	page    int
	perPage int
}

type packagesIntegrationDeleteRequest struct {
	path      string
	versionID int64
}

type packagesIntegrationServer struct {
	mutex                sync.Mutex
	pageOnePayload       string
	listRequests         []packagesIntegrationListRequest
	deleteRequests       []packagesIntegrationDeleteRequest
	authorizationHeaders []string
}

func newPackagesIntegrationServer(pageOnePayload string) *packagesIntegrationServer {
	return &packagesIntegrationServer{pageOnePayload: pageOnePayload}
}

func (server *packagesIntegrationServer) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	server.mutex.Lock()
	server.authorizationHeaders = append(server.authorizationHeaders, request.Header.Get("Authorization"))
	server.mutex.Unlock()

	switch request.Method {
	case http.MethodGet:
		pageValue := request.URL.Query().Get("page")
		perPageValue := request.URL.Query().Get("per_page")
		pageNumber, pageParseError := strconv.Atoi(pageValue)
		if pageParseError != nil {
			responseWriter.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(responseWriter, "invalid page: %v", pageParseError)
			return
		}

		perPageNumber, perPageParseError := strconv.Atoi(perPageValue)
		if perPageParseError != nil {
			responseWriter.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(responseWriter, "invalid per_page: %v", perPageParseError)
			return
		}

		listRequest := packagesIntegrationListRequest{
			path:    request.URL.Path,
			page:    pageNumber,
			perPage: perPageNumber,
		}

		server.mutex.Lock()
		server.listRequests = append(server.listRequests, listRequest)
		server.mutex.Unlock()

		responseWriter.Header().Set("Content-Type", "application/json")
		if pageNumber == 1 {
			_, _ = fmt.Fprint(responseWriter, server.pageOnePayload)
			return
		}

		_, _ = fmt.Fprint(responseWriter, "[]")
	case http.MethodDelete:
		pathSegments := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
		if len(pathSegments) == 0 {
			responseWriter.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(responseWriter, "missing identifier")
			return
		}

		identifierText := pathSegments[len(pathSegments)-1]
		versionID, parseError := strconv.ParseInt(identifierText, 10, 64)
		if parseError != nil {
			responseWriter.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(responseWriter, "invalid version identifier: %v", parseError)
			return
		}

		deleteRequest := packagesIntegrationDeleteRequest{
			path:      request.URL.Path,
			versionID: versionID,
		}

		server.mutex.Lock()
		server.deleteRequests = append(server.deleteRequests, deleteRequest)
		server.mutex.Unlock()

		responseWriter.WriteHeader(http.StatusNoContent)
	default:
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (server *packagesIntegrationServer) snapshotListRequests() []packagesIntegrationListRequest {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	requests := make([]packagesIntegrationListRequest, len(server.listRequests))
	copy(requests, server.listRequests)
	return requests
}

func (server *packagesIntegrationServer) snapshotDeleteRequests() []packagesIntegrationDeleteRequest {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	requests := make([]packagesIntegrationDeleteRequest, len(server.deleteRequests))
	copy(requests, server.deleteRequests)
	return requests
}

func (server *packagesIntegrationServer) snapshotAuthorizationHeaders() []string {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	headers := make([]string, len(server.authorizationHeaders))
	copy(headers, server.authorizationHeaders)
	return headers
}

func TestPackagesCommandIntegration(testInstance *testing.T) {
	workingDirectory, workingDirectoryError := os.Getwd()
	require.NoError(testInstance, workingDirectoryError)
	repositoryRoot := filepath.Dir(workingDirectory)

	pageOnePayload := fmt.Sprintf(
		packagesIntegrationVersionsResponseTemplateConstant,
		packagesIntegrationTaggedVersionIDConstant,
		packagesIntegrationFirstUntaggedVersionIDConstant,
		packagesIntegrationSecondUntaggedVersionIDConstant,
	)

	testCases := []struct {
		name              string
		dryRun            bool
		expectedDeleteIDs []int64
	}{
		{
			name:              "purge_deletes_untagged_versions",
			dryRun:            false,
			expectedDeleteIDs: []int64{packagesIntegrationFirstUntaggedVersionIDConstant, packagesIntegrationSecondUntaggedVersionIDConstant},
		},
		{
			name:              "dry_run_skips_deletion",
			dryRun:            true,
			expectedDeleteIDs: nil,
		},
	}

	for testCaseIndex, testCase := range testCases {
		subtestName := fmt.Sprintf(packagesIntegrationSubtestNameTemplateConstant, testCaseIndex, testCase.name)
		testInstance.Run(subtestName, func(subtest *testing.T) {
			serverState := newPackagesIntegrationServer(pageOnePayload)
			server := httptest.NewServer(serverState)
			defer server.Close()

			configDirectory := subtest.TempDir()
			configPath := filepath.Join(configDirectory, packagesIntegrationConfigFileNameConstant)
			configContent := fmt.Sprintf(
				packagesIntegrationConfigTemplateConstant,
				packagesIntegrationOwnerConstant,
				packagesIntegrationPackageConstant,
				packagesIntegrationOwnerTypeConstant,
				packagesIntegrationTokenReferenceConstant,
				testCase.dryRun,
				server.URL,
				packagesIntegrationPageSizeConstant,
			)

			writeError := os.WriteFile(configPath, []byte(configContent), 0o600)
			require.NoError(subtest, writeError)

			subtest.Setenv(packagesIntegrationTokenEnvNameConstant, packagesIntegrationTokenValueConstant)

			arguments := []string{
				packagesIntegrationRunSubcommandConstant,
				packagesIntegrationModulePathConstant,
				fmt.Sprintf(packagesIntegrationConfigFlagTemplateConstant, configPath),
				packagesIntegrationPackagesPurgeCommandNameConstant,
			}

			pathVariable := os.Getenv("PATH")
			commandOptions := integrationCommandOptions{PathVariable: pathVariable}
			_ = runIntegrationCommand(subtest, repositoryRoot, commandOptions, packagesIntegrationCommandTimeout, arguments)

			listRequests := serverState.snapshotListRequests()
			require.Len(subtest, listRequests, 2)

			expectedVersionsPath := fmt.Sprintf(
				packagesIntegrationVersionsPathTemplateConstant,
				packagesIntegrationOwnerConstant,
				packagesIntegrationPackageConstant,
			)

			require.Equal(subtest, expectedVersionsPath, listRequests[0].path)
			require.Equal(subtest, 1, listRequests[0].page)
			require.Equal(subtest, packagesIntegrationPageSizeConstant, listRequests[0].perPage)

			require.Equal(subtest, expectedVersionsPath, listRequests[1].path)
			require.Equal(subtest, 2, listRequests[1].page)
			require.Equal(subtest, packagesIntegrationPageSizeConstant, listRequests[1].perPage)

			deleteRequests := serverState.snapshotDeleteRequests()
			if len(testCase.expectedDeleteIDs) == 0 {
				require.Empty(subtest, deleteRequests)
			} else {
				require.Len(subtest, deleteRequests, len(testCase.expectedDeleteIDs))
				for deleteIndex, deleteRequest := range deleteRequests {
					expectedIdentifier := testCase.expectedDeleteIDs[deleteIndex]
					require.Equal(subtest, expectedIdentifier, deleteRequest.versionID)
					expectedDeletePath := fmt.Sprintf(
						packagesIntegrationDeletePathTemplateConstant,
						packagesIntegrationOwnerConstant,
						packagesIntegrationPackageConstant,
						expectedIdentifier,
					)
					require.Equal(subtest, expectedDeletePath, deleteRequest.path)
				}
			}

			authorizationHeaders := serverState.snapshotAuthorizationHeaders()
			expectedAuthorization := fmt.Sprintf(packagesIntegrationAuthorizationTemplateConstant, packagesIntegrationTokenValueConstant)
			expectedHeaderCount := len(listRequests) + len(deleteRequests)
			require.Len(subtest, authorizationHeaders, expectedHeaderCount)
			for _, headerValue := range authorizationHeaders {
				require.Equal(subtest, expectedAuthorization, headerValue)
			}
		})
	}
}
