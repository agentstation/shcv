MAKEFLAGS += --no-print-directory

# Default target
.PHONY: all
all: help

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php


.PHONY: help
help: ## Display the list of targets and their descriptions
	@awk 'BEGIN {FS = ":.*##"; printf "\n\033[1mUsage:\033[0m\n  make \033[36m<target>\033[0m\n"} \
		/^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } \
		/^###/ { printf "  \033[90m%s\033[0m\n", substr($$0, 4) }' $(MAKEFILE_LIST)

##@ Tooling

.PHONY: devbox
devbox: ## Run Devbox shell
	@devbox shell

.PHONY: devbox-update
devbox-update: ## Update Devbox
	@devbox version update

.PHONY: install-devbox
install-devbox: ## Install Devbox
	@echo "Installing Devbox..."
	@curl -fsSL https://get.jetify.dev | bash


##@ Install Dependencies

.PHONY: install
install: go-mod-download

.PHONY: go-mod-download
go-mod-download: ## Download Go modules specified in go.mod
	@echo "Downloading go modules..."
	@go mod download

##@ Development

.PHONY: fmt
fmt: ## Format Go code according to gofmt standards (excluding vendor)
	@echo "Running go fmt..."
	@go fmt $(shell go list ./... | grep -v /vendor/)

.PHONY: generate
generate: ## Run all code generation and formatting tasks
	@$(MAKE) go-doc

.PHONY: go-doc
go-doc: ## Generate Go documentation in GitHub-flavored Markdown
	@echo "Generating Go documentation..."
	@gomarkdoc -f github -o "{{.Dir}}/README.md" ./pkg/shcv/... --repository.url https://github.com/agentstation/shcv --repository.default-branch master --repository.path /pkg/shcv

.PHONY: vet
vet: ## Run go vet to check for code issues
	@echo "Running go vet..."
	@go vet $(shell go list ./... | grep -v /vendor/)

.PHONY: lint
lint: ## Run golangci-lint to lint Go code (excluding vendor)
	@echo "Running golangci-lint..."
	@golangci-lint run --exclude-dirs-use-default

##@ Testing & Coverage

.PHONY: test
test: ## Run Go tests (excluding vendor)
	@echo "Running golang tests..."
	@go test -v $(shell go list ./... | grep -v /vendor/)

.PHONY: coverage
coverage: ## Run tests and generate coverage report
	@echo "Running tests and generating coverage report..."
	@go test -race -coverprofile=coverage.txt -covermode=atomic ./...

.PHONY: check
check: vet lint test ## Run all static code checks (vet, lint and test)

##@ Build

.PHONY: build
build: ## Build the shcv binary
	@$(MAKE) go-build
	@echo "Compiled shcv binary => tmp/bin/shcv"

.PHONY: build-install
build-install: ## Install dependencies and build the shcv binary
	@$(MAKE) install
	@$(MAKE) build

.PHONY: go-build
go-build: ## Build Go application
	@echo "Building shcv binary..."
	@mkdir -p tmp/bin
	@CGO_ENABLED=0 go build -o tmp/bin/shcv -ldflags="-s -w" ./cmd/shcv

##@ Clean Up

.PHONY: clean
clean: ## Remove any temporary generated files and clean up the workspace
	@echo "Cleaning up any temporary generated files..."
	@rm -rf tmp