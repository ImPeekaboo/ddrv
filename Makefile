# Name of the binary to build
BINARY_NAME=ddrv

# Go source files
SRC=$(shell find . -name "*.go" -type f)

# Build the binary for the current platform
build:
	go build -ldflags="-s -w" -o $(BINARY_NAME) ./cmd/ddrv

build-race:
	go build -race -ldflags="-s -w" -o $(BINARY_NAME) ./cmd/ddrv

build-debug:
	go build -tags=debug -o $(BINARY_NAME) ./cmd/ddrv

# Build the binary inside docker image
build-docker:
	CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o $(BINARY_NAME) ./cmd/ddrv

# Build the Docker image
build-image:
	docker build -t ddrv:latest .

# Clean the project
clean:
	go clean
	rm -f $(BINARY_NAME)

# Run the tests
test:
	go test -v ./...

# Format the source code
fmt:
	gofmt -w $(SRC)
