# Single entry point for common tasks. Run `make` (or `make help`) to list targets.
# Works with the GNU Make 3.81 that ships with macOS.

IMAGE   ?= <dockerhub-user>/mapup-geofencing-api
TAG     ?= latest
VEHICLE ?=

# Load .env if present (a missing file is fine) and pass its variables to every recipe.
-include .env
export

.DEFAULT_GOAL := help

.PHONY: help up down logs db run test vet fmt fmt-check build check \
        docker-build docker-push psql reset-db simulate fe-install fe-dev fe-build

help: ## List all targets
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk -F ':.*## ' '{printf "  \033[36m%-13s\033[0m %s\n", $$1, $$2}'

up: ## Build and start db + api in the background
	docker compose up --build -d

down: ## Stop all containers (keeps data)
	docker compose down

logs: ## Follow API logs
	docker compose logs -f api

db: ## Start only the database (for `make run`)
	docker compose up -d db

run: ## Run the API locally with variables from .env
	cd backend && go run ./cmd/api

test: ## Run Go tests with the race detector
	cd backend && go test ./... -race

vet: ## Run go vet
	cd backend && go vet ./...

fmt: ## Format Go code
	cd backend && gofmt -w .

fmt-check: ## Fail if any Go file needs formatting
	@cd backend && out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed on:"; echo "$$out"; exit 1; fi

build: ## Build the API binary into backend/bin/
	cd backend && CGO_ENABLED=0 go build -o bin/api ./cmd/api

check: fmt-check vet test fe-build ## Everything that must pass before a commit

docker-build: ## Build the backend image as $(IMAGE):$(TAG)
	docker build -t $(IMAGE):$(TAG) backend

docker-push: docker-build ## Push $(IMAGE):$(TAG) to Docker Hub
	docker push $(IMAGE):$(TAG)

psql: ## Open psql inside the db container
	docker compose exec db psql -U geofence -d geofence

reset-db: ## Delete all data (docker compose down -v); needs CONFIRM=1
	@if [ "$(CONFIRM)" != "1" ]; then echo "WARNING: this deletes the database volume. Run: make reset-db CONFIRM=1"; exit 1; fi
	docker compose down -v

simulate: ## Drive VEHICLE in and out of a geofence: make simulate VEHICLE=veh_...
	@if [ -z "$(VEHICLE)" ]; then echo "usage: make simulate VEHICLE=<vehicle_id>"; exit 1; fi
	./backend/scripts/simulate.sh $(VEHICLE)

fe-install: ## Install frontend dependencies (npm ci)
	@if [ -d frontend ]; then cd frontend && npm ci; else echo "frontend/ does not exist yet, skipping"; fi

fe-dev: ## Start the frontend dev server
	@if [ -d frontend ]; then cd frontend && npm run dev; else echo "frontend/ does not exist yet, skipping"; fi

fe-build: ## Build the frontend
	@if [ -d frontend ]; then cd frontend && npm run build; else echo "frontend/ does not exist yet, skipping"; fi
