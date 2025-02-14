.PHONY: lint build test mixed plain r1 r5

lint:
	golangci-lint run ./...

build:
	go build ./main.go

test:
	go test -cover ./...

# go run main.go migrate --dirname=examples/tree --connstr postgres://postgres:postgres@localhost:5432/bookstore --mode=mixed
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

.ONESHELL:
r1:
	export PGMIGRATE_CONNSTR=postgres://postgres:postgres@localhost:5432/bookstore
	export PGMIGRATE_DIRNAME=examples/tree
	go run main.go rollback 1 --mode=mixed --yes-i-really-mean-it

.ONESHELL:
r5:
	export PGMIGRATE_CONNSTR=postgres://postgres:postgres@localhost:5432/bookstore
	export PGMIGRATE_DIRNAME=examples/tree
	go run main.go rollback 5 --mode=mixed --yes-i-really-mean-it
