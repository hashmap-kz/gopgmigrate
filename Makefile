# Variables
APP_NAME 	 := gopgmigrate
OUTPUT   	 := $(APP_NAME)
COV_REPORT 	 := coverage.txt
TEST_FLAGS 	 := -v -race -timeout 30s
INSTALL_DIR  := /usr/local/bin

ifeq ($(OS),Windows_NT)
	OUTPUT := $(APP_NAME).exe
endif

.PHONY: build
build: gen ## Build the binary
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/$(OUTPUT) main.go

.PHONY: build-linux
build-linux: gen ## Build the binary
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/gopgmigrate main.go

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run --output.tab.path=stdout

.PHONY: gen
gen: ## Run go generate
	go generate ./...

.PHONY: install
install: build ## Install the binary to $(INSTALL_DIR)
	@echo "Installing bin/$(OUTPUT) to $(INSTALL_DIR)..."
	@install -m 0755 bin/$(OUTPUT) $(INSTALL_DIR)

.PHONY: snapshot
snapshot: ## Run snapshot build with goreleaser
	GORELEASER_FORCE_TOKEN=github goreleaser release --skip sign --skip publish --snapshot --clean

.PHONY: test
test: ## Run unit tests
	go test -v -cover ./...

.PHONY: test-cov
test-cov: ## Run tests with coverage report
	go test -coverprofile=$(COV_REPORT) ./...
	go tool cover -html=$(COV_REPORT)

.PHONY: test-integ
test-integ:
	@(cd test/integration/environ && bash run.sh)
	go test -count=1 -tags=integration -v ./test/integration/... | tee test-integ.log

.PHONY: clean
clean: ## Remove build artifacts and logs
	@rm -rf bin/ dist/ test/integration/environ/bin/ *.log

.PHONY: help
help: ## Show this help
	@echo "Usage: make <target>"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z0-9_.-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-24s\033[0m %s\n", $$1, $$2}'

## TODO: cleanup

.ONESHELL:
last:
	export PGMIGRATE_CONNSTR=postgres://postgres:postgres@localhost:5432/bookstore
	export PGMIGRATE_DIRNAME=examples/tree
	go run main.go last

.ONESHELL:
r1:
	export PGMIGRATE_CONNSTR=postgres://postgres:postgres@localhost:5432/bookstore
	export PGMIGRATE_DIRNAME=examples/tree
	go run main.go rollback-count 1

.ONESHELL:
r5:
	export PGMIGRATE_CONNSTR=postgres://postgres:postgres@localhost:5432/bookstore
	export PGMIGRATE_DIRNAME=examples/tree
	go run main.go rollback-count 5
