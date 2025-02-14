.PHONY: lint build test mixed plain

lint:
	golangci-lint run ./...

build:
	go build ./main.go

test:
	go test -cover ./...

.ONESHELL:
mixed:
	export PGMIGRATE_CONNSTR=postgres://postgres:postgres@localhost:5432/bookstore
	export PGMIGRATE_DIRNAME=examples/tree
	go run main.go migrate --mode=mixed

.ONESHELL:
plain:
	export PGMIGRATE_CONNSTR=postgres://postgres:postgres@localhost:5432/bookstore
	export PGMIGRATE_DIRNAME=examples/tree
	go run main.go migrate --mode=plain
