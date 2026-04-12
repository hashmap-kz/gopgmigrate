# Variables
PG_MAJOR ?= 17
COMPOSE = PG_MAJOR=$(PG_MAJOR) docker compose
COMPOSE_FILE = docker-compose.yml

# Targets

.PHONY: up down ps logs clean restart

# Start the cluster
up:
	$(COMPOSE) -f $(COMPOSE_FILE) up -d --build
	@echo "✅ Cluster is starting..."

# Stop the cluster
down:
	$(COMPOSE) -f $(COMPOSE_FILE) down -v
	@echo "🛑 Cluster is stopped."

# Show running containers
ps:
	$(COMPOSE) -f $(COMPOSE_FILE) ps

# Show logs (follow)
logs:
	$(COMPOSE) -f $(COMPOSE_FILE) logs -f

# Restart the cluster
restart:
	@rm -rf bin
	@sh -c 'cd ../../../ && make build-linux && mv bin test/integration/environ'
	$(MAKE) down
	sleep 2
	$(MAKE) up

# Completely clean up volumes and containers
clean:
	$(COMPOSE) -f $(COMPOSE_FILE) down -v
	@echo "🧹 Containers and volumes are removed."

# Default target
default: up
