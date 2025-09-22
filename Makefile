GO_SOURCES := $(shell find . -name '*.go' -not -path "./vendor/*")

.PHONY: format check-format lint test ci

format:
	gofmt -w $(GO_SOURCES)

check-format:
	@formatted_files="$$(gofmt -l $(GO_SOURCES))"; \
	if [ -n "$$formatted_files" ]; then \
		echo "Go files require formatting:"; \
		echo "$$formatted_files"; \
		exit 1; \
	fi

lint:
	go vet ./...

test:
	go test ./...

ci: check-format lint test
