SHELL := bash
.ONESHELL:
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := all
.DELETE_ON_ERROR:
MAKEFLAGS += --warn-undefined-variables --no-builtin-rules
GO ?= go
TESTARGS ?=

# Build tags. Use `make build TAGS=dev` for a development build, which adds the
# `custom_endpoint` setting for pointing the provider at another API.
TAGS ?=
BINARY_NAME := statuscake-mcp

# Find all Go source files
GO_SOURCES := $(shell find . -type f -name '*.go')

ifeq ($(origin .RECIPEPREFIX), undefined)
  $(error This Make does not support .RECIPEPREFIX. Please use GNU Make 4.0 or later)
endif
.RECIPEPREFIX =

.PHONY: all
all: build test

.PHONY: help
help:
	@echo "Usage: make <target>"
	@echo
	@echo "Targets:"
	@echo "  all     Build and test the project (default)"
	@echo "  build   Build the project"
	@echo "  test    Run tests"
	@echo "  clean   Clean the project"

.PHONY: build
build: $(BINARY_NAME)
	@echo "done"

$(BINARY_NAME): $(GO_SOURCES) go.mod go.sum
	@echo "building $(BINARY_NAME)..."
	@go build -tags "$(TAGS)" -o $@ main.go

.PHONY: test
test:
	$(GO) vet ./...
	$(GO) test -race ./... -v $(TESTARGS)
	$(GO) vet -tags dev ./...
	$(GO) test -race -tags dev ./... -v $(TESTARGS)

.PHONY: clean
clean:
	@rm -f $(BINARY_NAME)
