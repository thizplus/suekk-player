.PHONY: build run dev tidy clean

# Build the worker
build:
	go build -o bin/seo-worker ./cmd/worker

# Run the worker
run: build
	./bin/seo-worker

# Run in development mode
dev:
	go run ./cmd/worker

# Download dependencies
tidy:
	go mod tidy

# Clean build artifacts
clean:
	rm -rf bin/

# Run tests
test:
	go test -v ./...

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Build for Linux (deployment)
build-linux:
	GOOS=linux GOARCH=amd64 go build -o bin/seo-worker-linux ./cmd/worker

# Docker build
docker-build:
	docker build -t seo-worker:latest .

# Docker run
docker-run:
	docker run --env-file .env seo-worker:latest
