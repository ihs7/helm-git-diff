.PHONY: build install clean test lint lint-yaml lint-md

BINARY_NAME=helm-git-diff
INSTALL_DIR=$(HELM_PLUGIN_DIR)/bin

build:
	@mkdir -p bin
	go build -o bin/$(BINARY_NAME) main.go

install: build
	@echo "Plugin installed successfully"

clean:
	rm -rf bin/

test:
	go test -v ./...

lint:
	$(shell go env GOPATH)/bin/golangci-lint run .

lint-yaml:
	$(shell go env GOPATH)/bin/yamllint .

lint-md:
	npx markdownlint-cli '**/*.md'

.DEFAULT_GOAL := build
