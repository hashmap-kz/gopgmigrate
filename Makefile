.PHONY: lint fmt build test mixed plain r1 r5

lint:
	golangci-lint run ./...

fmt:
	gofumpt -w .

build:
	go build ./main.go

test: fmt
	go test -cover ./...

# go run main.go migrate --dirname=examples/tree --connstr postgres://postgres:postgres@localhost:5432/bookstore --mode=mixed
.ONESHELL:
migrate:
	export PGMIGRATE_CONNSTR=postgres://postgres:postgres@localhost:5432/bookstore
	export PGMIGRATE_DIRNAME=examples/tree
	go run main.go migrate

.ONESHELL:
r1:
	export PGMIGRATE_CONNSTR=postgres://postgres:postgres@localhost:5432/bookstore
	export PGMIGRATE_DIRNAME=examples/tree
	go run main.go rollback-count 1 --yes-i-really-mean-it

.ONESHELL:
r5:
	export PGMIGRATE_CONNSTR=postgres://postgres:postgres@localhost:5432/bookstore
	export PGMIGRATE_DIRNAME=examples/tree
	go run main.go rollback-count 5 --yes-i-really-mean-it
