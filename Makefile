APP_NAME 	 := gopgmigrate
OUTPUT   	 := $(APP_NAME)
INSTALL_DIR  := /usr/local/bin

ifeq ($(OS),Windows_NT)
	OUTPUT := $(APP_NAME).exe
endif

.PHONY: build
build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/$(OUTPUT) ./cmd/gopgmigrate/

.PHONY: lint
lint:
	golangci-lint run --output.tab.path=stdout

.PHONY: install
install: build
	@echo "Installing bin/$(OUTPUT) to $(INSTALL_DIR)..."
	@sudo chmod +x bin/$(OUTPUT) && sudo cp bin/$(OUTPUT) $(INSTALL_DIR)

.PHONY: test
test:
	go test -v -race -cover -count=1 -timeout=5m ./...

COMPOSE      = docker compose -f test/integration/environ/docker-compose.yml

.PHONY: test-integration
test-integration:
	$(COMPOSE) up -d
	@until docker exec pg-primary pg_isready -U test >/dev/null 2>&1; do sleep 1; done
	go test -v -race -count=1 -timeout=5m -tags integration ./test/integration/...; \
		EXIT=$$?; \
		$(COMPOSE) down -v; \
		exit $$EXIT

.PHONY: clean
clean:
	@rm -rf bin/ dist/ *.log

.PHONY: snapshot
snapshot:
	GORELEASER_FORCE_TOKEN=github goreleaser release --skip sign --skip publish --snapshot --clean
