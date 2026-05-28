BINARY     := health
MODULE     := comp-health
MAIN       := ./cmd/health
BUILD_DIR  := dist

VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -s -w \
              -X main.Version=$(VERSION) \
              -X main.BuildTime=$(BUILD_TIME)

.PHONY: all build build-linux build-darwin build-windows \
        tidy test run-server run-agent \
        init-server-config init-agent-config clean

## Default target
all: build

## Build for the current platform
build:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(MAIN)
	@echo "Built: $(BUILD_DIR)/$(BINARY)"

## Cross-compile targets
build-linux:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-linux-amd64 $(MAIN)

build-darwin:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
		go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-darwin-arm64 $(MAIN)

build-windows:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
		go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-windows-amd64.exe $(MAIN)

## Build all platforms
build-all: build-linux build-darwin build-windows

## Dependency management
tidy:
	go mod tidy

## Run tests
test:
	go test ./...

## Run server in development
run-server: configs/server.yaml
	go run $(MAIN) server -c configs/server.yaml

## Run agent in development
run-agent: configs/agent.yaml
	go run $(MAIN) agent -c configs/agent.yaml

## Generate example configs
configs/server.yaml:
	@mkdir -p configs
	go run $(MAIN) init-config --mode server --out configs/server.yaml

configs/agent.yaml:
	@mkdir -p configs
	go run $(MAIN) init-config --mode agent --out configs/agent.yaml

init-server-config:
	@mkdir -p configs
	go run $(MAIN) init-config --mode server --out configs/server.yaml

init-agent-config:
	@mkdir -p configs
	go run $(MAIN) init-config --mode agent --out configs/agent.yaml

## Clean build artefacts
clean:
	rm -rf $(BUILD_DIR)
