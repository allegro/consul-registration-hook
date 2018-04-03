.PHONY: all build lint lint-deps test integration-test

all: lint test build

build:
	go build -v -o ./build/consul-registration-hook ./cmd/consul-registration-hook

lint: lint-deps
	gometalinter.v2 --config=gometalinter.json ./...

lint-deps:
	@which gometalinter.v2 > /dev/null || \
	(go get -u -v gopkg.in/alecthomas/gometalinter.v2 && gometalinter.v2 --install)

test:
	go test -v -coverprofile=coverage.txt -covermode=atomic ./...

integration-test:
	scripts/integration_test.sh