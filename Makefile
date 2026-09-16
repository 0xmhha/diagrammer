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
.PHONY: build test race cover fmt fmt-check vet lint tidy clean install run linux check verify fixtures vendor-check

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
#
# testdata and vendor are excluded on purpose. The Go files under testdata are
# fixture input rather than code this project maintains, and one of them is
# deliberately malformed so the analyzer has a parse failure to report. The ones
# under vendor belong to somebody else and are not ours to reformat.
fmt-check:
	@unformatted=$$(gofmt -l $$(find . -name '*.go' \
		-not -path './testdata/*' -not -path './vendor/*' -not -path './bin/*')); \
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

## tidy: reconcile go.mod, go.sum and vendor/ with the imports
#
# vendor/ is committed so that `make verify` needs no network, which is what
# the release gate asks for. It has to be regenerated whenever a dependency
# changes, and `make verify` fails if it is stale.
tidy:
	$(GO) mod tidy
	$(GO) mod vendor

## check: what must pass before a commit
check: fmt vet test

## vendor-check: fail if vendor/ has drifted from go.mod
#
# The gate depends on vendor/ being complete, so a stale one would turn an
# offline machine's build failure into somebody else's afternoon.
vendor-check:
	@$(GO) mod verify >/dev/null
	@if [ ! -d vendor ]; then \
		echo "vendor/ is missing; run 'make tidy'"; \
		exit 1; \
	fi
	@$(GO) list -mod=vendor ./... >/dev/null 2>&1 || { \
		echo "vendor/ does not satisfy the imports; run 'make tidy'"; \
		exit 1; \
	}

## fixtures: run the shipped binary over every committed fixture
#
# compose is asked for everything a model declares. A family with no composer
# is refused by name rather than skipped in silence, so this line failing means
# a fixture grew a family nobody built rather than that the gate is too strict.
#
# This exercises the binary a second person would get, not the library the
# tests link against, because "the tests pass" and "the program works" are
# different claims.
fixtures: build
	@set -e; \
	work=$$(mktemp -d); \
	trap 'rm -rf "$$work"' EXIT; \
	found=0; \
	for d in testdata/src/*/; do \
		[ -d "$$d" ] || continue; \
		found=$$((found + 1)); \
		name=$$(basename "$$d"); \
		$(BIN_DIR)/$(BINARY) graph "$$d" -o "$$work/$$name.1.json"; \
		$(BIN_DIR)/$(BINARY) graph "$$d" -o "$$work/$$name.2.json"; \
		cmp -s "$$work/$$name.1.json" "$$work/$$name.2.json" \
			|| { echo "$$name: two runs produced different bytes"; exit 1; }; \
	done; \
	if [ "$$found" -eq 0 ]; then \
		echo "no source fixtures under testdata/src; the stage-1 gate would pass by doing nothing"; \
		exit 1; \
	fi; \
	echo "$$found source fixture(s): schema valid and byte-identical across runs"; \
	found=0; \
	for f in testdata/codegraph/*.codegraph.json; do \
		[ -e "$$f" ] || continue; \
		found=$$((found + 1)); \
		$(BIN_DIR)/$(BINARY) validate "$$f"; \
	done; \
	if [ "$$found" -eq 0 ]; then \
		echo "no fixtures under testdata/codegraph; the stage-2 gate would pass by doing nothing"; \
		exit 1; \
	fi; \
	echo "$$found model fixture(s) validated"; \
	found=0; \
	for f in testdata/codegraph/*.codegraph.json; do \
		[ -e "$$f" ] || continue; \
		found=$$((found + 1)); \
		name=$$(basename "$$f" .codegraph.json); \
		$(BIN_DIR)/$(BINARY) compose "$$f" -o "$$work/$$name.1" >/dev/null; \
		$(BIN_DIR)/$(BINARY) compose "$$f" -o "$$work/$$name.2" >/dev/null; \
		for doc in "$$work/$$name.1"/*.diagram.json; do \
			[ -e "$$doc" ] || continue; \
			cmp -s "$$doc" "$$work/$$name.2/$$(basename "$$doc")" \
				|| { echo "$$name: two compose runs produced different bytes"; exit 1; }; \
		done; \
	done; \
	echo "$$found model fixture(s) composed: schema valid, complete, byte-identical across runs"; \
	found=0; \
	for doc in "$$work"/*.1/*.diagram.json; do \
		[ -e "$$doc" ] || continue; \
		found=$$((found + 1)); \
		$(BIN_DIR)/$(BINARY) render "$$doc" -o "$$doc.1.html" >/dev/null; \
		$(BIN_DIR)/$(BINARY) render "$$doc" -o "$$doc.2.html" >/dev/null; \
		cmp -s "$$doc.1.html" "$$doc.2.html" \
			|| { echo "$$doc: two render runs produced different bytes"; exit 1; }; \
	done; \
	if [ "$$found" -eq 0 ]; then \
		echo "nothing was rendered; the stage-4 gate would pass by doing nothing"; \
		exit 1; \
	fi; \
	echo "$$found page(s) rendered: composition rules pass, byte-identical across runs"

## verify: the release gate
#
# Green here is what authorises a tag, and it is the only automated gate. It
# must run offline on a clean macOS machine with only Go and make installed;
# anything it needs that such a machine lacks is a defect, not a prerequisite.
verify: vendor-check fmt-check vet test fixtures
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
