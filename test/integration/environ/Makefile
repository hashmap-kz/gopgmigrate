PG_MAJOR ?= 17
COMPOSE = PG_MAJOR=$(PG_MAJOR) docker compose
COMPOSE_FILE = docker-compose.yml

.PHONY: up down restart

up:
	$(COMPOSE) -f $(COMPOSE_FILE) up -d --build
	@echo "Cluster is starting..."

down:
	$(COMPOSE) -f $(COMPOSE_FILE) down -v
	@echo "Cluster is stopped."

restart:
	@rm -rf bin
	@sh -c 'cd ../../../ && make build && mv bin test/integration/environ'
	$(MAKE) down
	sleep 2
	$(MAKE) up

default: up
