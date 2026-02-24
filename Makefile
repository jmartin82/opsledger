.PHONY: up down build rebuild logs logs-ui logs-backend logs-db \
       shell-ui shell-backend shell-db \
       db-cli db-reset clean status \
       test test-v test-integration test-all \
       prod-build prod-push prod-up prod-down prod-logs prod-status

# Start all services
up:
	docker compose up -d

# Start all services with build
up-build:
	docker compose up -d --build

# Stop all services
down:
	docker compose down

# Build all images
build:
	docker compose build

# Rebuild all images (no cache)
rebuild:
	docker compose build --no-cache

# Show logs for all services
logs:
	docker compose logs -f

# Show logs per service
logs-ui:
	docker compose logs -f ui

logs-backend:
	docker compose logs -f backend

logs-db:
	docker compose logs -f db

# Open a shell in a service
shell-ui:
	docker compose exec ui sh

shell-backend:
	docker compose exec backend sh

shell-db:
	docker compose exec db bash

# Connect to MariaDB CLI
db-cli:
	docker compose exec db mariadb -u tracker -ptracker_dev ops_ledger

# Reset database (destroy volume and recreate)
db-reset:
	docker compose down -v
	docker compose up -d db
	@echo "Waiting for MariaDB to be ready..."
	@sleep 10
	docker compose up -d

# Remove all containers, volumes, and images
clean:
	docker compose down -v --rmi local

# Show status of all services
status:
	docker compose ps

# Restart a specific service (usage: make restart s=backend)
restart:
	docker compose restart $(s)

# ---------------------------------------------------------------------------
# Backend Tests
# ---------------------------------------------------------------------------

# Run unit tests only
test:
	cd backend && go test ./... -run "^Test[^I]"

# Run unit tests with verbose output
test-v:
	cd backend && go test ./... -run "^Test[^I]" -v

# Run integration tests (requires running MariaDB)
test-integration:
	cd backend && go test ./... -tags=integration -v

# Run all tests (unit + integration)
test-all:
	cd backend && go test ./... -tags=integration -v

# ---------------------------------------------------------------------------
# Production Builds
# ---------------------------------------------------------------------------

IMAGE_REGISTRY ?= ghcr.io/jmartin
IMAGE_TAG      ?= $(shell git rev-parse --short HEAD)

# Build production images (tagged with git SHA and :latest)
prod-build:
	docker build -f frontend/Dockerfile.prod --build-arg VITE_API_URL= \
		-t $(IMAGE_REGISTRY)/opsledger/ui:$(IMAGE_TAG) \
		-t $(IMAGE_REGISTRY)/opsledger/ui:latest ./frontend
	docker build -f backend/Dockerfile.prod \
		-t $(IMAGE_REGISTRY)/opsledger/backend:$(IMAGE_TAG) \
		-t $(IMAGE_REGISTRY)/opsledger/backend:latest ./backend

# Build and push production images to registry
prod-push: prod-build
	docker push $(IMAGE_REGISTRY)/opsledger/ui:$(IMAGE_TAG)
	docker push $(IMAGE_REGISTRY)/opsledger/ui:latest
	docker push $(IMAGE_REGISTRY)/opsledger/backend:$(IMAGE_TAG)
	docker push $(IMAGE_REGISTRY)/opsledger/backend:latest

# Start production stack (requires .env file)
prod-up:
	docker compose -f docker-compose.prod.yml up -d

# Stop production stack
prod-down:
	docker compose -f docker-compose.prod.yml down

# Tail logs for production stack
prod-logs:
	docker compose -f docker-compose.prod.yml logs -f

# Show status of production stack
prod-status:
	docker compose -f docker-compose.prod.yml ps
