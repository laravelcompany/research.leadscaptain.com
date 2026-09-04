.PHONY: dev build test lint run docker-build docker-up backup
dev:; go run ./cmd/server
build:; go build -o bin/server ./cmd/server
test:; go test ./... -count=1
test-race:; go test -race ./...
lint:; go vet ./...
run:; go run ./cmd/server
docker-build:; docker build -t research-leads .
docker-up:; docker compose up --build
docker-down:; docker compose down
backup:; mkdir -p backups && cp data/leads.db backups/leads-$$(date +%Y-%m-%d-%H%M%S).db || echo "no db"
migrate:; go run ./cmd/server --migrate || true
