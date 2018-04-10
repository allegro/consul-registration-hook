NAME    := consul-registration-hook
VERSION := $(shell git describe --tags || echo "unknown")

ARCH      := $(shell go env GOARCH)
OS        := $(shell go env GOOS)
BUILD_DIR := build
LDFLAGS   := -X main.version=$(VERSION)

.PHONY: all build lint lint-deps test integration-test

all: lint test build

build:
	go build -v -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(NAME) ./cmd/$(NAME)

package: build
	tar -czvf $(BUILD_DIR)/$(NAME)-$(VERSION)-$(OS)-$(ARCH).tar.gz -C $(BUILD_DIR) $(NAME)

clean:
	go clean -v .
	rm -rf $(BUILD_DIR)

lint: lint-deps
	gometalinter.v2 --config=gometalinter.json ./...

lint-deps:
	@which gometalinter.v2 > /dev/null || \
	(go get -u -v gopkg.in/alecthomas/gometalinter.v2 && gometalinter.v2 --install)

test:
	go test -v -coverprofile=coverage.txt -covermode=atomic ./...

integration-test:
	scripts/integration_test.sh