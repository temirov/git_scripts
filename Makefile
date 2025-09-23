GO_SOURCES := $(shell find . -name '*.go' -not -path "./vendor/*")
UNIT_PACKAGES := $(shell go list ./... | grep -v '/tests$$')

.PHONY: format check-format lint test test-unit test-integration build ci

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

test-unit:
	go test $(UNIT_PACKAGES)

test-integration:
	go test ./tests

test: test-unit test-integration

build:
	mkdir -p bin
	go build -o bin/git-scripts .

ci: check-format lint test
