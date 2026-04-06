.PHONY: dev dev-down run build test sqlc tidy

# Start dev infrastructure (PostgreSQL, Redis, MinIO)
dev:
	docker compose -f infra/docker/docker-compose.dev.yml up -d

# Stop dev infrastructure
dev-down:
	docker compose -f infra/docker/docker-compose.dev.yml down

# Run the API server locally
run:
	cd backend && go run ./cmd/api

# Build the API binary
build:
	cd backend && go build -o bin/api ./cmd/api

# Run tests
test:
	cd backend && go test ./... -v -count=1 -p 1

# Generate sqlc code
sqlc:
	cd backend && $(shell which sqlc || echo $$HOME/go/bin/sqlc) generate

# Tidy Go modules
tidy:
	cd backend && go mod tidy
