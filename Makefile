# diagrammer build rules.
#
# macOS is the supported target today. Linux is listed below but deliberately
# refuses to run, so a cross build fails loudly instead of producing a binary
# nobody has tested.

BINARY      := diagrammer
PKG         := ./cmd/diagrammer
BIN_DIR     := bin
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X main.version=$(VERSION)
GO          ?= go
GOLANGCI    := golangci-lint

.DEFAULT_GOAL := build
.PHONY: build test race cover fmt fmt-check vet lint tidy clean install run linux check verify fixtures

## build: compile the binary for this machine
build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(BINARY) $(PKG)
	@echo "built $(BIN_DIR)/$(BINARY) $(VERSION)"

## test: run the unit tests
test:
	$(GO) test ./...

## race: run the tests under the race detector
race:
	$(GO) test -race ./...

## cover: write a coverage profile and report the total
cover:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -1

## fmt: format every file; formatting is the tool's decision, not a review topic
fmt:
	$(GO) fmt ./...

## fmt-check: fail if anything is unformatted, rather than fixing it
fmt-check:
	@unformatted=$$(gofmt -l . 2>/dev/null); \
	if [ -n "$$unformatted" ]; then \
		echo "these files are not gofmt'd:"; \
		echo "$$unformatted"; \
		echo "run 'make fmt'"; \
		exit 1; \
	fi

## vet: the checks the toolchain ships with
vet:
	$(GO) vet ./...

## lint: the wider rule set, when it is installed
lint:
	@command -v $(GOLANGCI) >/dev/null 2>&1 \
		|| { echo "$(GOLANGCI) not installed: brew install golangci-lint"; exit 1; }
	$(GOLANGCI) run

## tidy: reconcile go.mod and go.sum with the imports
tidy:
	$(GO) mod tidy

## check: what must pass before a commit
check: fmt vet test

## fixtures: run the shipped binary over every committed fixture
#
# This exercises the binary a second person would get, not the library the
# tests link against, because "the tests pass" and "the program works" are
# different claims.
fixtures: build
	@set -e; \
	found=0; \
	for f in testdata/codegraph/*.codegraph.json; do \
		[ -e "$$f" ] || continue; \
		found=$$((found + 1)); \
		$(BIN_DIR)/$(BINARY) validate "$$f"; \
	done; \
	if [ "$$found" -eq 0 ]; then \
		echo "no fixtures found under testdata/codegraph; the gate would pass by doing nothing"; \
		exit 1; \
	fi; \
	echo "$$found fixture(s) validated"

## verify: the release gate
#
# Green here is what authorises a tag, and it is the only automated gate. It
# must run offline on a clean macOS machine with only Go and make installed;
# anything it needs that such a machine lacks is a defect, not a prerequisite.
verify: fmt-check vet test fixtures
	@echo "verify: ok"

## install: put the binary on PATH via GOBIN
install:
	$(GO) install -trimpath -ldflags '$(LDFLAGS)' $(PKG)

## run: build and run, e.g. make run ARGS=version
run: build
	@$(BIN_DIR)/$(BINARY) $(ARGS)

## clean: remove build output
clean:
	rm -rf $(BIN_DIR) coverage.out coverage.html

## linux: not supported yet
linux:
	@echo "linux builds are not supported yet."
	@echo "when they are, they will run on a native linux runner rather than"
	@echo "cross-compiling, so the binary that ships is the binary that was tested."
	@exit 1

## help: list the targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed -e 's/## //'
