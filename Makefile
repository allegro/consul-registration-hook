NAME    := consul-registration-hook
VERSION := $(shell git describe --tags || echo "unknown")

ARCH      := $(shell go env GOARCH)
OS        := $(shell go env GOOS)
BUILD_DIR := build
LDFLAGS   := -X main.version=$(VERSION)

CURRENT_DIR = $(shell pwd)
BIN = $(CURRENT_DIR)/bin
THIS_FILE := $(lastword $(MAKEFILE_LIST))
.PHONY: all build build-linux package clean lint lint-deps test integration-test

all: lint test build

build:
	CGO_ENABLED=0 go build -v -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(NAME) ./cmd/$(NAME)

build-linux:
	GOOS=linux CGO_ENABLED=0 $(MAKE) -f $(THIS_FILE) build

package: build
	tar -czvf $(BUILD_DIR)/$(NAME)-$(VERSION)-$(OS)-$(ARCH).tar.gz -C $(BUILD_DIR) $(NAME)

clean:
	go clean -v ./...
	rm -rf $(BUILD_DIR)

lint: lint-deps
	$(BIN)/golangci-lint --version
	$(BIN)/golangci-lint run --config=golangcilinter.yaml ./...

lint-deps:
	@which golangci-lint > /dev/null || \
		(curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(BIN) v1.30.0)

test:
	go test -v -coverprofile=coverage.txt -covermode=atomic ./...

integration-test:
	scripts/integration_test.sh