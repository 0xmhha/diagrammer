# diagrammer build rules.
#
# macOS is the supported target today, and `dist` packages both of its
# architectures. `linux` refuses, and not because the result would be a cross
# build: `dist` cross-compiles. It refuses because nothing here can run a linux
# binary, and the rule is that nothing ships until it has been run.

BINARY      := diagrammer
PKG         := ./cmd/diagrammer
BIN_DIR     := bin
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X main.version=$(VERSION)
GO          ?= go
GOLANGCI    := golangci-lint

# Packaging. See the dist target for why each of these is pinned rather than
# left to whatever the machine doing the building happens to be.
DIST_DIR    := dist
DIST_ARCHES := arm64 amd64
# The oldest macOS the packaged binaries declare they will run on. 12.0 is not
# a preference: it is the floor this Go toolchain already puts on a build of its
# own, and the number is here so that the cgo build is held to the same one
# instead of inheriting the builder's macOS. It is a declaration and not a
# measurement, and docs/install.md says so; the oldest macOS anything here has
# actually been run on is the one that built it.
MACOS_FLOOR := 12.0
# Ad-hoc signing, which is not notarisation and does not satisfy Gatekeeper. It
# gives the binary an identity that is its own rather than the linker's a.out,
# and lets a recipient ask codesign whether the file still matches itself.
SIGN_ID     := com.github.0xmhha.diagrammer
# Every timestamp in an archive is set to this, so that packing the same
# directory twice produces the same bytes and a checksum means something.
DIST_MTIME  := 202001010000

# Drawing a project. See the diagram target for what each of these does; they
# are here because make needs them resolved before any recipe runs.
OUT         ?= out/diagram
# AI=1 performs stage 2 by calling a model, which is the only way this draws a
# tree in one command. It is opt-in and will never be a default: it spends money
# and it sends the graph, which carries every doc comment in the tree, to
# whoever runs that model. AI_CMD is what gets run, so pointing this at a
# different tool needs no change here.
AI_CMD      ?= claude -p --output-format text
# The same tool, allowed to read. A graph too big to send is read by the model
# out of the directory the run is working in, so the command needs file access
# and nothing else does. It is a separate variable because granting a model a
# shell is not something to do on every run for the sake of one.
AI_TOOL_CMD ?= claude -p --output-format text --allowed-tools Read Grep Bash Write
# How many times to ask before giving up. A model is not reliable run to run:
# asked for one small fixture six times it returned a fenced document, a null
# where the schema wants a string, and a note over its length limit, each once.
# None of those is a reason to stop, and all of them are caught by `validate`
# rather than guessed at, so asking again is both safe and usually enough.
AI_TRIES    ?= 3
# A graph larger than this is not sent; the model is pointed at the file and
# reads it instead. Measured: a 1.13 MB graph is answered and a 2.82 MB one is
# refused outright, so the cut is well below the refusal and well above the
# 349 KB this repository produces. See the diagram target.
AI_INLINE_KB ?= 1200
# What is asked for past the instruction itself. Every sentence was put here by
# a run that failed without it: the answer came fenced, then with a null where
# the schema wants a string, then with a note over the length limit the schema
# states and the instruction already carried.
AI_TAIL     := Return the codegraph.json document and nothing else. Leave an optional field out rather than setting it to null. Keep provenance.note under 1000 characters, which is the limit the schema states and the one most often broken.
ifeq ($(AI),1)
MODEL_CMD      := $(AI_CMD)
MODEL_TOOL_CMD := $(AI_TOOL_CMD)
endif
# Somebody who named their own MODEL_CMD gets it in both modes: they chose a
# tool, and this is not the place to substitute a different one behind them.
MODEL_TOOL_CMD ?= $(MODEL_CMD)

# Whether MODEL_CMD was given is decided here rather than in the recipe. A
# command line holds quotes, and `[ -n "$(MODEL_CMD)" ]` hands those quotes to
# the shell a second time, which turns a perfectly good command into a test with
# the wrong number of arguments and answers no. Running it is still make's own
# expansion, which is what makes the quotes work where they are meant to.
HAS_MODEL_CMD := $(if $(strip $(MODEL_CMD)),1,)
ifeq ($(POLYGLOT),1)
DIAGRAM_BIN := $(BIN_DIR)/$(BINARY)-polyglot
DIAGRAM_DEP := build-polyglot
else
DIAGRAM_BIN := $(BIN_DIR)/$(BINARY)
DIAGRAM_DEP := build
endif

# tarball packs one staged directory into one archive. The members are named in
# a fixed order rather than walked, and the ownership and names are pinned, for
# the same reason as the timestamp above: what comes out has to be a function of
# what went in.
define tarball
tar -cf - -C $(DIST_DIR) --uid 0 --gid 0 --uname '' --gname '' \
	"$(1)/install.md" "$(1)/LICENSE" "$(1)/THIRD_PARTY_NOTICES.md" \
	"$(1)/$(BINARY)" "$(1)/$(BINARY)-polyglot" | gzip -n -9 > "$(2)"
endef

.DEFAULT_GOAL := build
.PHONY: build build-polyglot test test-cgo race cover fmt fmt-check vet lint tidy clean install run linux check verify verify-cgo fixtures vendor-check dist dist-check diagram

## build: compile the binary for this machine
#
# CGO_ENABLED=0, which reads Go and nothing else. That is the binary the release
# gate covers and the one that needs no C toolchain, so it is what `make build`
# means without being asked otherwise.
build:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(BINARY) $(PKG)
	@echo "built $(BIN_DIR)/$(BINARY) $(VERSION), reading Go"

## build-polyglot: compile the binary that also reads Python, Solidity and JS/TS
#
# Needs a C toolchain, because tree-sitter is a C library and Go links C through
# cgo and nothing else. Which languages a binary reads is printed by `graph` on
# every run, so nobody has to remember which one they built.
build-polyglot:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=1 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(BINARY)-polyglot $(PKG)
	@echo "built $(BIN_DIR)/$(BINARY)-polyglot $(VERSION), reading Go, Python, Solidity and JS/TS"

## test: run the unit tests
#
# CGO_ENABLED=0 on purpose. This is the build the release gate covers, and it
# must not quietly start depending on a C toolchain being present.
test:
	CGO_ENABLED=0 $(GO) test ./...

## test-cgo: run the tests for the build that reads four languages
test-cgo:
	CGO_ENABLED=1 $(GO) test ./...

## size: what the second build costs in bytes, both binaries built fresh
#
# The figure docs/decisions.md carried for this was the publishing project's
# own and had never been taken here. This is how it is taken, so that it is
# regenerated rather than remembered.
size: build build-polyglot
	@go_only=$$(wc -c < $(BIN_DIR)/$(BINARY)); 	polyglot=$$(wc -c < $(BIN_DIR)/$(BINARY)-polyglot); 	printf '  %-30s %10d bytes  %6.2f MB\n' 'reading Go' $$go_only $$(echo "$$go_only/1048576" | bc -l); 	printf '  %-30s %10d bytes  %6.2f MB\n' 'reading four languages' $$polyglot $$(echo "$$polyglot/1048576" | bc -l); 	printf '  %-30s %10d bytes  %6.2f MB\n' 'the runtime and four grammars' $$((polyglot - go_only)) $$(echo "($$polyglot - $$go_only)/1048576" | bc -l)

## bench: what the second build costs in time, per megabyte it reads
bench:
	CGO_ENABLED=1 $(GO) test ./internal/analyze/treesitter/ -run '^$$' -bench BenchmarkAnalyze -benchtime 5x

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
#
# Both builds, because a linter run with CGO_ENABLED=0 never sees the four
# grammars: every file in that package but its doc is behind a build tag, so
# the checks would pass by not looking.
#
# Deliberately not part of `make verify`. The release gate is defined as a clean
# machine with only Go and make, and requiring a linter would change that
# definition for a check that finds style rather than defects. It is part of
# `make check`, which is what to run before a commit on a machine that has it.
lint:
	@command -v $(GOLANGCI) >/dev/null 2>&1 \
		|| { echo "$(GOLANGCI) not installed: brew install golangci-lint"; exit 1; }
	CGO_ENABLED=0 $(GOLANGCI) run ./...
	@command -v cc >/dev/null 2>&1 && CGO_ENABLED=1 $(GOLANGCI) run ./... \
		|| echo "no C compiler: the four-language build was not linted"

## tidy: reconcile go.mod, go.sum and vendor/ with the imports
#
# vendor/ is committed so that `make verify` needs no network, which is what
# the release gate asks for. It has to be regenerated whenever a dependency
# changes, and `make verify` fails if it is stale.
tidy:
	$(GO) mod tidy
	$(GO) mod vendor

## check: what must pass before a commit
#
# Wider than `verify` rather than narrower: this runs on a development machine
# that has the linter, and `verify` runs on one that has only Go and make.
check: fmt vet lint test

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

## verify-cgo: the second gate, for the build that reads four languages
#
# Separate because the first one must stay runnable on a machine with only Go
# and make. This one needs a C toolchain, and a release claiming those languages
# has to pass it: an analyzer no gate covers is worse than one that does not
# exist, because its output looks the same as a tree with none of that language
# in it.
verify-cgo: build-polyglot
	@command -v cc >/dev/null 2>&1 \
		|| { echo "no C compiler; this gate needs one and 'make verify' does not"; exit 1; }
	CGO_ENABLED=1 $(GO) vet ./...
	$(MAKE) test-cgo
	@set -e; \
	work=$$(mktemp -d); \
	trap 'rm -rf "$$work"' EXIT; \
	found=0; \
	for d in testdata/src/*/; do \
		[ -d "$$d" ] || continue; \
		found=$$((found + 1)); \
		name=$$(basename "$$d"); \
		$(BIN_DIR)/$(BINARY)-polyglot graph "$$d" -o "$$work/$$name.1.json"; \
		$(BIN_DIR)/$(BINARY)-polyglot graph "$$d" -o "$$work/$$name.2.json"; \
		cmp -s "$$work/$$name.1.json" "$$work/$$name.2.json" \
			|| { echo "$$name: two runs produced different bytes"; exit 1; }; \
	done; \
	if [ "$$found" -eq 0 ]; then \
		echo "no source fixtures; this gate would pass by doing nothing"; \
		exit 1; \
	fi; \
	echo "$$found source fixture(s) read by every grammar: schema valid, byte-identical"
	@echo "verify-cgo: ok"

## dist: package the binaries for a mac that did not build them
#
# `make install` needs a Go toolchain on the machine that will run the program.
# This is for a machine that has none: one archive per macOS architecture, each
# holding both builds, the licences, and the notes a recipient needs.
#
# Cross compiling here and refusing it for linux is not a contradiction. The
# objection to a cross build is that nobody has run it, and an x86_64 mac binary
# runs on an arm64 one through Rosetta, so `dist-check` runs the whole pipeline
# out of every archive it makes. Nothing here can run a linux binary.
#
# This is the one target that asks for more than Go and make. It needs the Xcode
# command line tools, for clang to build the cgo half and for codesign and otool
# to check what came out. `make verify` is the gate that must run on a machine
# with neither, and it still does.
#
# MACOSX_DEPLOYMENT_TARGET is the reason this target exists rather than a tar
# command in a shell history. clang defaults it to the version of macOS doing
# the building, so the cgo build was quietly refusing to launch on anything
# older than the builder's own machine while the Go-only build beside it ran
# back to 12.0. A floor that depends on who built it is the packaging defect
# this target is for.
dist: verify
	@rm -rf $(DIST_DIR)
	@mkdir -p $(DIST_DIR)
	@set -e; \
	for arch in $(DIST_ARCHES); do \
		case $$arch in \
			arm64) cc="clang -arch arm64";; \
			amd64) cc="clang -arch x86_64";; \
			*) echo "no C compiler flags for darwin/$$arch"; exit 1;; \
		esac; \
		name=$(BINARY)_$(VERSION)_darwin_$$arch; \
		stage=$(DIST_DIR)/$$name; \
		mkdir -p "$$stage"; \
		CGO_ENABLED=0 GOOS=darwin GOARCH=$$arch \
			MACOSX_DEPLOYMENT_TARGET=$(MACOS_FLOOR) \
			$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o "$$stage/$(BINARY)" $(PKG); \
		CGO_ENABLED=1 GOOS=darwin GOARCH=$$arch CC="$$cc" \
			MACOSX_DEPLOYMENT_TARGET=$(MACOS_FLOOR) \
			$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o "$$stage/$(BINARY)-polyglot" $(PKG); \
		for bin in $(BINARY) $(BINARY)-polyglot; do \
			codesign --sign - --identifier $(SIGN_ID) --force "$$stage/$$bin" >/dev/null 2>&1 \
				|| { echo "$$name: could not sign $$bin"; exit 1; }; \
		done; \
		cp LICENSE THIRD_PARTY_NOTICES.md docs/install.md "$$stage/"; \
		find "$$stage" -exec touch -t $(DIST_MTIME) {} +; \
		$(call tarball,$$name,$(DIST_DIR)/$$name.tar.gz); \
		$(call tarball,$$name,$(DIST_DIR)/.$$name.again); \
		cmp -s "$(DIST_DIR)/$$name.tar.gz" "$(DIST_DIR)/.$$name.again" \
			|| { echo "$$name: two archivings of one directory produced different bytes"; exit 1; }; \
		rm -f "$(DIST_DIR)/.$$name.again"; \
		rm -rf "$$stage"; \
		echo "packed $(DIST_DIR)/$$name.tar.gz"; \
	done
	@cd $(DIST_DIR) && shasum -a 256 *.tar.gz > SHA256SUMS
	@$(MAKE) --no-print-directory dist-check
	@echo "dist: ok"

## dist-check: prove an archive works away from the tree that built it
#
# Run by `dist` rather than instead of it. Packaging that nothing opens again is
# the same class of claim as an analyzer no gate covers: it looks identical to
# one that works until somebody else needs it to.
#
# Everything here is done to the unpacked copy, from a directory that is not
# this one, with an environment that carries nothing but a PATH. A binary that
# reached back into the build tree would pass every test in `make verify` and
# fail on the first machine it was sent to.
#
# The quarantine step is the one that reads oddly. Gatekeeper kills the process
# rather than refusing to start it, and a shell announces a child that died by a
# signal, so the check would print `Killed: 9` every time it passed. The inner
# `sh -c` is there to receive that announcement, and the `exit $$?` after the
# command is there to stop `sh` optimising itself away and leaving the
# announcement to this shell after all.
dist-check:
	@set -e; \
	test -d $(DIST_DIR) || { echo "no $(DIST_DIR)/; run 'make dist'"; exit 1; }; \
	root=$$(pwd); \
	( cd $(DIST_DIR) && shasum -a 256 -c SHA256SUMS >/dev/null ); \
	found=0; \
	for archive in $(DIST_DIR)/*.tar.gz; do \
		[ -e "$$archive" ] || continue; \
		found=$$((found + 1)); \
		name=$$(basename "$$archive" .tar.gz); \
		work=$$(mktemp -d); \
		trap 'rm -rf "$$work"' EXIT; \
		tar xzf "$$archive" -C "$$work"; \
		unpacked="$$work/$$name"; \
		for bin in $(BINARY) $(BINARY)-polyglot; do \
			codesign --verify "$$unpacked/$$bin" \
				|| { echo "$$name: $$bin lost its signature in the archive"; exit 1; }; \
			floor=$$(otool -l "$$unpacked/$$bin" \
				| awk '/LC_BUILD_VERSION/{seen=1} seen && /minos/{print $$2; exit}'); \
			[ "$$floor" = "$(MACOS_FLOOR)" ] \
				|| { echo "$$name: $$bin declares macOS $$floor, the package says $(MACOS_FLOOR)"; exit 1; }; \
		done; \
		grep -q 'macOS $(MACOS_FLOOR)' "$$unpacked/install.md" \
			|| { echo "$$name: install.md does not tell a reader the floor is $(MACOS_FLOOR)"; exit 1; }; \
		cp -R "$$root/testdata/src/go-basic" "$$work/src"; \
		cp -R "$$root/testdata/src/polyglot" "$$work/polyglot-src"; \
		cp "$$root/testdata/codegraph/diagrammer.codegraph.json" "$$work/model.json"; \
		( cd "$$work" && env -i PATH=/usr/bin:/bin "$$unpacked/$(BINARY)" graph src -o graph.json ) >/dev/null; \
		( cd "$$work" && env -i PATH=/usr/bin:/bin "$$unpacked/$(BINARY)" validate model.json ) >/dev/null; \
		( cd "$$work" && env -i PATH=/usr/bin:/bin "$$unpacked/$(BINARY)" compose model.json -o out ) >/dev/null; \
		( cd "$$work" && env -i PATH=/usr/bin:/bin "$$unpacked/$(BINARY)" render out/component.diagram.json -o page.html ) >/dev/null; \
		grep -q '<svg' "$$work/page.html" \
			|| { echo "$$name: the packaged binary wrote a page with no drawing in it"; exit 1; }; \
		( cd "$$work" && env -i PATH=/usr/bin:/bin "$$unpacked/$(BINARY)" graph polyglot-src -o go-only.json ) >/dev/null; \
		( cd "$$work" && env -i PATH=/usr/bin:/bin "$$unpacked/$(BINARY)-polyglot" graph polyglot-src -o four.json ) >/dev/null; \
		go_only=$$(wc -c < "$$work/go-only.json"); four=$$(wc -c < "$$work/four.json"); \
		[ "$$four" -gt "$$go_only" ] \
			|| { echo "$$name: the four-language build read no more than the Go-only one"; exit 1; }; \
		cp "$$unpacked/$(BINARY)" "$$work/quarantined"; \
		xattr -w com.apple.quarantine '0081;00000000;dist-check;' "$$work/quarantined"; \
		if sh -c "'$$work/quarantined' version; exit \$$?" >/dev/null 2>&1; then \
			echo "  note: a quarantined binary ran here, so Gatekeeper is not enforcing on this machine"; \
		fi; \
		xattr -d com.apple.quarantine "$$work/quarantined"; \
		"$$work/quarantined" version >/dev/null \
			|| { echo "$$name: install.md's remedy for a quarantined download does not work"; exit 1; }; \
		rm -rf "$$work"; trap - EXIT; \
		echo "  $$name: signed, floors at $(MACOS_FLOOR), both builds run the pipeline from outside the tree"; \
	done; \
	if [ "$$found" -eq 0 ]; then \
		echo "no archives in $(DIST_DIR)/; this gate would pass by doing nothing"; \
		exit 1; \
	fi; \
	echo "$$found archive(s) checked"

## diagram: read a project and draw it, in one command
#
# Stage 2 is not in this program, so there is a gap in the middle of this. The
# gap is the reason this is a make target and not a subcommand: a subcommand
# that drove a model would put stage 2 back inside the binary, and keeping it
# out is the decision the whole pipeline is shaped by. make is glue, and glue is
# allowed to know about a model.
#
#   make diagram SRC=../some/project
#     reads the tree, writes the graph and the instruction stage 2 is performed
#     from, and stops. No model, no diagram.
#
#   make diagram SRC=../some/project MODEL=out/diagram/model.codegraph.json
#     picks up from a model, whoever wrote it, and draws every family in it.
#
#   make diagram SRC=../some/project AI=1
#     performs stage 2 by calling a model, and is the only one of these that
#     draws a tree in one command. Opt-in, because it spends money and hands the
#     graph, doc comments and all, to whoever runs that model. A graph small
#     enough is sent; one too big is read by the model instead, which is slower
#     and has no size limit.
#
#   make diagram SRC=../some/project MODEL_CMD='your-own-runner'
#     the same path with something else on the other end. The command is handed
#     the whole stage-2 prompt on its standard input, which is the instruction
#     followed by the graph, and must write a model to its standard output.
#     AI=1 is this with AI_CMD filled in.
#
# Whatever answers goes through `validate` before anything is drawn, so a model
# that came back wrong is refused here rather than three stages later, and then
# through it a second time against the graph, which reports how much of the tree
# any component actually stood for. The second pass does not refuse anything: a
# model is a map rather than a census and may leave things out. It may not leave
# them out without saying so. What was
# asked is left in OUT/stage-2.prompt, so a bad answer can be read next to the
# question that produced it.
#
# The answer has its code fences stripped before it is validated. Asked for bare
# JSON, the model fenced it anyway, which was measured rather than supposed.
# Anything the stripping leaves behind is validate's problem, which is the right
# place for it.
#
# The prompt ends with two sentences this adds, and both were put there by a run
# that failed. One asks for the document alone, because the answer came fenced.
# The other asks for an absent field to be left out rather than set to null,
# because a run produced `"parent": null` where the schema wants a string.
#
# The third is the length of `provenance.note`, and it is there because asking
# again did not help. Two kinds of wrong answer turned up while this was built
# and they want different things. A model that returns a fenced document once
# and a clean one the next time is flaky, and asking again is enough. A model
# that goes over the same length limit three times in a row has misread the
# instruction, and asking a fourth time is just spending money: the limit was in
# the prompt, because the instruction carries the schema verbatim, and it was
# read past anyway. The sentence above says it where it cannot be missed.
#
# What holds all of this up is that `validate` runs before anything is drawn.
# Each attempt says what was wrong with the last, and the answer that finally
# fails is kept beside the prompt that produced it. A model is not a function,
# and the gate is what makes that survivable rather than silent.
#
# A model command that fails has whatever it wrote reported before it is thrown
# away, out of both streams. The first thing this was pointed at that did not fit
# answered "Prompt is too long" on its standard output rather than its standard
# error, so the message went into the file holding the model and was deleted with
# it, and the failure arrived with no reason attached.
#
# A graph past AI_INLINE_KB is not sent at all. The model is pointed at the file
# and reads it with its own tools, out of the directory the run is working in, so
# nothing large ever enters a prompt and the limit stops applying. Measured
# against the default: a 1.13 MB graph is answered when sent, a 2.82 MB one is
# refused outright, and the whole of a 9.2 MB tree is drawn in about three
# minutes when read rather than sent. The instruction is 33 KB of any prompt, so
# the graph is the part that varies.
#
# Sending is still what happens when it fits, because it is far faster: seconds
# against minutes on a small tree. Reading is the way to do the one thing that
# was impossible, not a better way to do the thing that already worked.
#
# OUT= puts the work somewhere else. POLYGLOT=1 uses the four-language build,
# which is usually what you want for a tree that is not all Go.
#
# It is here rather than shipped because a packaged binary comes with no
# Makefile. What a plugin drives is `serve`, which needs no glue at all.
diagram: $(DIAGRAM_DEP)
	@set -e; \
	test -n "$(SRC)" || { \
		echo "make diagram needs SRC=<a source directory>"; \
		echo "  make diagram SRC=../some/project"; \
		exit 1; \
	}; \
	out="$(OUT)"; \
	mkdir -p "$$out/documents"; \
	$(DIAGRAM_BIN) graph "$(SRC)" -o "$$out/graph.json"; \
	$(DIAGRAM_BIN) instruct -o "$$out/stage-2.md"; \
	model="$(MODEL)"; \
	if [ -z "$$model" ] && [ -n "$(HAS_MODEL_CMD)" ]; then \
		model="$$out/model.codegraph.json"; \
		graphkb=$$(($$(wc -c < "$$out/graph.json") / 1024)); \
		if [ "$$graphkb" -le "$(AI_INLINE_KB)" ]; then \
			reading=""; \
			{ cat "$$out/stage-2.md"; \
			  printf '\n\n## The code graph\n\n'; \
			  cat "$$out/graph.json"; \
			  printf '\n\n%s\n' '$(AI_TAIL)'; \
			} > "$$out/stage-2.prompt"; \
		else \
			reading=" by reading it"; \
			{ cat "$$out/stage-2.md"; \
			  printf '\n\n## Where the code graph is\n\n'; \
			  printf 'Not in this prompt. It is graph.json in this directory, %s KB of it, ' "$$graphkb"; \
			  printf 'which is more than can be handed to you whole.\n\n'; \
			  printf 'Read it with your tools. It is one JSON document with a nodes array '; \
			  printf 'and an edges array; every node carries an id, a kind of group, '; \
			  printf 'package, file, type or func, a parent, and often a doc comment. '; \
			  printf 'Sample it however you like, but what you return describes the whole '; \
			  printf 'tree, so find out what is in all of it before deciding what the '; \
			  printf 'parts are.\n\n'; \
			  printf 'Write the document to model.codegraph.json in this directory. Put '; \
			  printf 'nothing on your standard output but the path you wrote.\n\n'; \
			  printf '%s\n' '$(AI_TAIL)'; \
			} > "$$out/stage-2.prompt"; \
		fi; \
		size=$$(($$(wc -c < "$$out/stage-2.prompt") / 1024)); \
		try=1; \
		while :; do \
			echo "stage 2: asking $(firstword $(MODEL_CMD)) about a $$graphkb KB graph$$reading, attempt $$try of $(AI_TRIES)"; \
			rm -f "$$model"; \
			fail=0; \
			if [ -z "$$reading" ]; then \
				( cd "$$out" && $(MODEL_CMD) < stage-2.prompt ) \
					> "$$model.part" 2> "$$out/stage-2.err" || fail=$$?; \
			else \
				( cd "$$out" && $(MODEL_TOOL_CMD) < stage-2.prompt ) \
					> "$$model.part" 2> "$$out/stage-2.err" || fail=$$?; \
			fi; \
			if [ "$$fail" -ne 0 ]; then \
				echo "stage 2: the model command failed. What it said:"; \
				cat "$$out/stage-2.err" "$$model.part" 2>/dev/null \
					| grep -v '^[[:space:]]*$$' | head -5 | sed 's/^/  /'; \
				rm -f "$$model.part" "$$out/stage-2.err"; \
				exit 1; \
			fi; \
			rm -f "$$out/stage-2.err"; \
			if [ ! -s "$$model" ]; then \
				sed '/^```/d' "$$model.part" | sed -n '/^{/,$$p' > "$$model"; \
			fi; \
			rm -f "$$model.part"; \
			if [ -s "$$model" ] && $(DIAGRAM_BIN) validate "$$model" >/dev/null 2>&1; then \
				echo "stage 2: wrote $$model"; \
				$(DIAGRAM_BIN) validate "$$model" -graph "$$out/graph.json" \
					2>&1 | grep -v ': valid,' || true; \
				break; \
			fi; \
			if [ "$$try" -ge "$(AI_TRIES)" ]; then \
				echo "stage 2: $(AI_TRIES) answers, none of which satisfies the contract. The last one:"; \
				$(DIAGRAM_BIN) validate "$$model" 2>&1 | sed 's/^/  /'; \
				echo "  it is kept at $$model, next to the prompt that produced it"; \
				exit 1; \
			fi; \
			echo "stage 2: that answer was refused, asking again"; \
			$(DIAGRAM_BIN) validate "$$model" 2>&1 | head -3 | sed 's/^/  /'; \
			try=$$((try + 1)); \
		done; \
	fi; \
	if [ -z "$$model" ]; then \
		echo; \
		echo "no diagram was drawn, because stage 2 has not been performed."; \
		echo "  the graph to read:   $$out/graph.json"; \
		echo "  what to do with it:  $$out/stage-2.md"; \
		echo; \
		echo "hand both to a skill, keep what it returns, then run:"; \
		echo "  make diagram SRC=$(SRC) MODEL=<the file it returned>"; \
		echo; \
		echo "or have this ask a model for you, which spends money and hands the"; \
		echo "graph to whoever runs it:"; \
		echo "  make diagram SRC=$(SRC) AI=1"; \
		echo; \
		echo "(make reports this as an error because nothing was drawn, which is"; \
		echo " what an exit code means here. The run itself did what it could.)"; \
		exit 1; \
	fi; \
	$(DIAGRAM_BIN) validate "$$model"; \
	$(DIAGRAM_BIN) compose "$$model" -o "$$out/documents"; \
	drawn=0; \
	for doc in "$$out/documents"/*.diagram.json; do \
		[ -e "$$doc" ] || continue; \
		family=$$(basename "$$doc" .diagram.json); \
		$(DIAGRAM_BIN) render "$$doc" -o "$$out/$$family.html"; \
		drawn=$$((drawn + 1)); \
	done; \
	if [ "$$drawn" -eq 0 ]; then \
		echo "$$model declares no family anything here can draw"; \
		exit 1; \
	fi; \
	echo; \
	echo "$$drawn page(s):"; \
	for page in "$$out"/*.html; do echo "  $$page"; done

## install: put the binary on PATH via GOBIN
install:
	$(GO) install -trimpath -ldflags '$(LDFLAGS)' $(PKG)

## run: build and run, e.g. make run ARGS=version
run: build
	@$(BIN_DIR)/$(BINARY) $(ARGS)

## clean: remove build output
clean:
	rm -rf $(BIN_DIR) $(DIST_DIR) coverage.out coverage.html

## linux: not supported yet
#
# `dist` cross-compiles for the other mac architecture, so the rule is not that
# cross-compiling is forbidden. The rule is that nothing ships without having
# been run: Rosetta runs an x86_64 mac binary on this machine and `dist-check`
# does run it, and there is nothing here that will run a linux one. When there
# is a linux runner, this becomes the same target pointed at it.
linux:
	@echo "linux builds are not supported yet."
	@echo "when they are, they will be run on linux before they ship, the way"
	@echo "'make dist' runs the x86_64 mac binary it cross-compiles."
	@exit 1

## help: list the targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed -e 's/## //'
