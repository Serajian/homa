# ==================================================================================== #
# VARIABLES
# ==================================================================================== #
APP_NAME  := homa
MODULE    := github.com/Serajian/homa
BUILD_DIR := build
MAIN      := ./cmd/$(APP_NAME)

COLOR_RESET=\033[0m
COLOR_GREEN=\033[32m
COLOR_YELLOW=\033[33m
COLOR_RED=\033[31m
COLOR_BLUE=\033[34m

GO_BIN       := $(shell go env GOPATH)/bin
GOVULNCHECK  := $(GO_BIN)/govulncheck
GOVULNDB    ?= https://vuln.go.dev
GOLANGCI_LINT := $(shell \
    for bin in /opt/homebrew/bin/golangci-lint $$(which -a golangci-lint 2>/dev/null) "$(GO_BIN)/golangci-lint"; do \
       if [ -x "$$bin" ] && "$$bin" version 2>/dev/null | grep -Eq 'version (v)?2\.'; then \
          echo "$$bin"; exit 0; \
       fi; \
    done; \
    echo golangci-lint \
)

TEST_TIMEOUT := 60s

GO_DIRS := $(shell find . -type f -name '*.go' \
    -not -path './build/*' \
    -not -path './.git/*' \
    -exec dirname {} \; | sort -u)

.DEFAULT_GOAL := help

# ==================================================================================== #
# HELP
# ==================================================================================== #
.PHONY: help
help: ## [Help] Show this help
	@echo ""
	@echo "Usage:"
	@echo "  make <target>"
	@echo ""
	@awk 'BEGIN { FS=":.*## "; } \
	/^[a-zA-Z0-9][a-zA-Z0-9_-]*:.*## / { \
	   target=$$1; desc=$$2; \
	   group="Other"; \
	   if (substr(desc,1,1)=="[") { \
	      rb=index(desc,"]"); \
	      if (rb>0) { \
	         group=substr(desc,2,rb-2); \
	         desc=substr(desc,rb+1); \
	         gsub(/^[ \t]+/,"",desc); \
	      } \
	   } \
	   items[group]=items[group] sprintf("  \033[36m%-20s\033[0m %s\n", target, desc); \
	   if (!(group in seen)) { order[++n]=group; seen[group]=1 } \
	} \
	END { \
	   for (i=1; i<=n; i++) { \
	      g=order[i]; \
	      printf "\033[33m%s\033[0m\n", g; \
	      printf "%s\n", items[g]; \
	   } \
	}' $(MAKEFILE_LIST)
	@echo ""

# ==================================================================================== #
# SETUP
# ==================================================================================== #
.PHONY: at-first
at-first: setup-dev deps git-hooks ## [Setup] First-time setup (tools + deps + git hooks)

.PHONY: setup-dev
setup-dev: ## [Setup] Install dev tools (gofumpt, gci, golangci-lint, misspell, ...)
	@echo "$(COLOR_YELLOW)Installing development tools...$(COLOR_RESET)"
	@go install github.com/segmentio/golines@latest
	@go install github.com/daixiang0/gci@latest
	@go install mvdan.cc/gofumpt@latest
	@go install github.com/gordonklaus/ineffassign@latest
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	@go install github.com/client9/misspell/cmd/misspell@latest
	@go install github.com/jondot/goweight@latest
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@echo "$(COLOR_GREEN)Dev tools installed.$(COLOR_RESET)"

.PHONY: deps
deps: ## [Setup] Tidy and download Go modules
	@echo "$(COLOR_YELLOW)Syncing dependencies...$(COLOR_RESET)"
	@go mod tidy
	@go mod download
	@echo "$(COLOR_GREEN)Dependencies are up to date.$(COLOR_RESET)"

# ==================================================================================== #
# BUILD & RUN
# ==================================================================================== #
.PHONY: build
build: ## [Build] Build the binary into ./build
	@echo "$(COLOR_YELLOW)Building...$(COLOR_RESET)"
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(APP_NAME) $(MAIN)
	@echo "$(COLOR_GREEN)Build complete: $(BUILD_DIR)/$(APP_NAME)$(COLOR_RESET)"

.PHONY: build-linux
build-linux: ## [Build] Cross-compile for a linux/amd64 server
	@echo "$(COLOR_YELLOW)Cross-compiling for linux/amd64...$(COLOR_RESET)"
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 $(MAIN)
	@echo "$(COLOR_GREEN)Build complete: $(BUILD_DIR)/$(APP_NAME)-linux-amd64$(COLOR_RESET)"

.PHONY: build-all
build-all: build build-linux ## [Build] Build for this machine and for linux/amd64

.PHONY: install
install: ## [Build] Install into GOPATH/bin so `homa` works anywhere
	@go install $(MAIN)
	@echo "$(COLOR_GREEN)Installed to $(GO_BIN)/$(APP_NAME)$(COLOR_RESET)"

.PHONY: run
run: ## [Run] Run the app locally
	@go run $(MAIN)

.PHONY: brun
brun: clean build ## [Run] Build and run the binary
	@./$(BUILD_DIR)/$(APP_NAME)

# ==================================================================================== #
# FORMAT & BASIC CHECKS
# ==================================================================================== #
.PHONY: format-core
format-core: gofumpt gci misspell govet ineffassign ## [Format] Format without golines + basic checks
	@echo "$(COLOR_GREEN)Core formatting done.$(COLOR_RESET)"

.PHONY: format
format: format-core golines ## [Format] Format everything + basic checks
	@echo "$(COLOR_GREEN)Formatting done.$(COLOR_RESET)"

.PHONY: gofumpt
gofumpt: ## [Format] Format Go files using gofumpt (-extra)
	@echo "$(COLOR_YELLOW)gofumpt...$(COLOR_RESET)"
	@gofumpt -extra -w .

.PHONY: gci
gci: ## [Format] Sort and group imports using gci
	@echo "$(COLOR_YELLOW)gci...$(COLOR_RESET)"
	@gci write --skip-generated \
	   -s standard \
	   -s default \
	   -s "prefix($(MODULE))" \
	   --custom-order \
	   .

.PHONY: golines
golines: ## [Format] Wrap long lines using golines (max 100)
	@echo "$(COLOR_YELLOW)golines...$(COLOR_RESET)"
	@golines --max-len=100 -w .

# base ref the fast golines path diffs against
GOLINES_BASE ?= origin/master

.PHONY: golines-changed
golines-changed: ## [Format] golines only on Go files changed vs $(GOLINES_BASE)
	@echo "$(COLOR_YELLOW)golines (changed vs $(GOLINES_BASE))...$(COLOR_RESET)"
	@base=$$(git merge-base $(GOLINES_BASE) HEAD 2>/dev/null) ; \
	if [ -z "$$base" ]; then \
	   echo "  $(GOLINES_BASE) not found, formatting the whole tree" ; \
	   golines --max-len=100 -w . ; \
	else \
	   files=$$( { git diff --name-only --diff-filter=d "$$base" HEAD -- '*.go' ; \
	               git diff --name-only --diff-filter=d -- '*.go' ; \
	               git diff --name-only --diff-filter=d --cached -- '*.go' ; } | sort -u ) ; \
	   if [ -n "$$files" ]; then \
	      printf '%s\n' "$$files" | tr '\n' '\0' | xargs -0 golines --max-len=100 -w ; \
	   else \
	      echo "  no changed Go files" ; \
	   fi ; \
	fi

.PHONY: misspell
misspell: ## [Format] Fix common misspellings (Go files only)
	@echo "$(COLOR_YELLOW)misspell...$(COLOR_RESET)"
	@find . -type f -name '*.go' \
	   -not -path './build/*' \
	   -not -path './.git/*' \
	   -print0 | xargs -0 misspell -w

.PHONY: govet
govet: ## [Format] Run go vet
	@echo "$(COLOR_YELLOW)go vet...$(COLOR_RESET)"
	@go vet ./...

.PHONY: ineffassign
ineffassign: ## [Format] Detect ineffectual assignments
	@echo "$(COLOR_YELLOW)ineffassign...$(COLOR_RESET)"
	@ineffassign $(GO_DIRS)

# ==================================================================================== #
# LINT
# ==================================================================================== #
.PHONY: lint
lint: ## [Lint] Run golangci-lint
	@echo "$(COLOR_YELLOW)golangci-lint...$(COLOR_RESET)"
	@$(GOLANGCI_LINT) run --timeout 2m ./...
	@echo "$(COLOR_GREEN)Lint passed.$(COLOR_RESET)"

.PHONY: lint-fix
lint-fix: format-core ## [Lint] Format then golangci-lint --fix
	@echo "$(COLOR_YELLOW)golangci-lint --fix...$(COLOR_RESET)"
	@$(GOLANGCI_LINT) run --fix --timeout 2m ./...
	@echo "$(COLOR_GREEN)Lint autofix done.$(COLOR_RESET)"

.PHONY: vulncheck
vulncheck: ## [Lint] Scan for known vulnerabilities reachable from our code
	@echo "$(COLOR_YELLOW)govulncheck (db: $(GOVULNDB))...$(COLOR_RESET)"
	@[ -x "$(GOVULNCHECK)" ] || go install golang.org/x/vuln/cmd/govulncheck@latest
	@$(GOVULNCHECK) -db "$(GOVULNDB)" ./... || { \
	   echo "$(COLOR_RED)govulncheck failed. If it was a 403 or a timeout, vuln.go.dev is likely"; \
	   echo "   geo-blocked here. Retry behind a VPN, or point GOVULNDB at a mirror:$(COLOR_RESET)"; \
	   echo "   HTTPS_PROXY=http://host:port make vulncheck"; \
	   echo "   make vulncheck GOVULNDB=<mirror-or-file://path>"; \
	   exit 1; \
	}
	@echo "$(COLOR_GREEN)No reachable vulnerabilities.$(COLOR_RESET)"

.PHONY: precommit
precommit: deps lint-fix lint build ## [Lint] Pre-commit pipeline (deps + format + lint + build)
	@echo "$(COLOR_GREEN)Pre-commit checks passed.$(COLOR_RESET)"

.PHONY: prepush
prepush: deps golines-changed lint-fix lint test-race clean ## [Lint] Pre-push pipeline (+ race tests)
	@echo "$(COLOR_GREEN)Pre-push checks passed.$(COLOR_RESET)"

# ==================================================================================== #
# TEST & BENCH
# ==================================================================================== #
.PHONY: test
test: ## [Test] Run the tests
	@echo "$(COLOR_BLUE)Running tests...$(COLOR_RESET)"
	@go test -race -timeout=$(TEST_TIMEOUT) ./...
	@echo "$(COLOR_GREEN)Tests passed.$(COLOR_RESET)"

.PHONY: test-race
test-race: ## [Test] Run race-detector tests across all packages (used by prepush)
	@echo "$(COLOR_BLUE)Running race-detector tests...$(COLOR_RESET)"
	@go test -race -timeout=$(TEST_TIMEOUT) ./...
	@echo "$(COLOR_GREEN)Race tests passed.$(COLOR_RESET)"

.PHONY: test-short
test-short: ## [Test] Run only short tests
	@go test -short -race ./...

.PHONY: test-verbose
test-verbose: ## [Test] Run tests with verbose output and no cache
	@go test -v -race -timeout=$(TEST_TIMEOUT) ./... -count=1

.PHONY: test-coverage
test-coverage: ## [Test] Generate a coverage report
	@echo "$(COLOR_BLUE)Running tests with coverage...$(COLOR_RESET)"
	@go test ./... -coverprofile=coverage.out -coverpkg=./internal/...
	@go tool cover -func=coverage.out | grep total | awk '{print "$(COLOR_GREEN)Total coverage: " $$3 "$(COLOR_RESET)"}'

.PHONY: cover-html
cover-html: test-coverage ## [Test] Open the coverage report in a browser
	@go tool cover -html=coverage.out

.PHONY: bench
bench: ## [Test] Run benchmarks
	@go test -bench=. -benchmem ./...

# ==================================================================================== #
# DOC
# ==================================================================================== #
.PHONY: doc
doc: ## [Doc] Print the public API of every internal package
	@for p in $$(go list ./internal/...); do \
	   echo "$(COLOR_YELLOW)=== $$p$(COLOR_RESET)"; \
	   go doc $$p; \
	   echo; \
	done

# ==================================================================================== #
# GIT
# ==================================================================================== #
.PHONY: git-hooks
git-hooks: ## [Git] Point git at .githooks and make the hooks executable
	@echo "$(COLOR_YELLOW)Setting up git hooks...$(COLOR_RESET)"
	@git config --local core.hooksPath .githooks
	@chmod +x .githooks/pre-commit
	@chmod +x .githooks/pre-push
	@echo "$(COLOR_GREEN)Git hooks configured.$(COLOR_RESET)"

.PHONY: master
master: ## [Git] Go to master, sync with the remote, drop local branches already merged
	@if [ -n "$$(git status --porcelain --untracked-files=no)" ]; then \
	   echo "$(COLOR_RED)You have uncommitted changes.$(COLOR_RESET)"; \
	   git status --short --untracked-files=no; \
	   echo "$(COLOR_RED)   Checking out master would carry them onto master.$(COLOR_RESET)"; \
	   echo "$(COLOR_RED)   Commit them on your branch, or stash them, then run make master again.$(COLOR_RESET)"; \
	   exit 1; \
	fi
	@echo "$(COLOR_YELLOW)Switching to master...$(COLOR_RESET)"
	@git checkout master
	@echo "$(COLOR_YELLOW)Fetching every remote, pruning branches deleted upstream...$(COLOR_RESET)"
	@git fetch --all --prune
	@echo "$(COLOR_YELLOW)Pulling master...$(COLOR_RESET)"
	@git pull --ff-only
	@echo "$(COLOR_YELLOW)Deleting local branches already merged into origin/master...$(COLOR_RESET)"
	@merged=$$(git branch --merged origin/master --format='%(refname:short)' \
	   | grep -vE '^(master|main|dev)$$' || true); \
	if [ -z "$$merged" ]; then \
	   echo "   nothing to delete"; \
	else \
	   echo "$$merged" | xargs -n1 git branch -d; \
	fi
	@stale=$$(git branch -vv | grep ': gone\]' | sed -E 's/^\*? *([^ ]+).*/\1/' \
	   | grep -vE '^(master|main|dev)$$' || true); \
	if [ -n "$$stale" ]; then \
	   echo "$(COLOR_YELLOW)Gone upstream but NOT merged into origin/master: squashed, or genuinely unmerged.$(COLOR_RESET)"; \
	   echo "$(COLOR_YELLOW)   Left alone on purpose. Check, then delete by hand:$(COLOR_RESET)"; \
	   echo "$$stale" | sed 's/^/   git branch -D /'; \
	fi
	@echo "$(COLOR_GREEN)master is up to date.$(COLOR_RESET)"

.PHONY: clean-branches
clean-branches: ## [Git] Delete all local branches except master, main and dev
	@echo "$(COLOR_YELLOW)Cleaning local branches...$(COLOR_RESET)"
	@git branch | grep -vE "master|main|dev" | xargs -r git branch -D
	@git fetch --prune
	@echo "$(COLOR_GREEN)Local branches cleaned.$(COLOR_RESET)"

# ==================================================================================== #
# ANALYZE
# ==================================================================================== #
.PHONY: analyze-size
analyze-size: build ## [Analyze] Show total, runtime and own-code binary size
	@echo "$(COLOR_YELLOW)Analyzing binary size...$(COLOR_RESET)"
	@TOTAL_SIZE=$$(stat -f%z $(BUILD_DIR)/$(APP_NAME) 2>/dev/null || stat -c %s $(BUILD_DIR)/$(APP_NAME)); \
	RUNTIME_SIZE=$$(go tool nm -size $(BUILD_DIR)/$(APP_NAME) | grep ' runtime\.' | awk '{sum += $$2} END {print sum}'); \
	if [ -z "$$RUNTIME_SIZE" ]; then RUNTIME_SIZE=0; fi; \
	CUSTOM_SIZE=$$((TOTAL_SIZE - RUNTIME_SIZE)); \
	PCT_RUNTIME=0; PCT_CUSTOM=0; \
	if [ $$TOTAL_SIZE -gt 0 ]; then \
	   PCT_RUNTIME=$$((RUNTIME_SIZE * 100 / TOTAL_SIZE)); \
	   PCT_CUSTOM=$$((CUSTOM_SIZE * 100 / TOTAL_SIZE)); \
	fi; \
	human() { \
	   if [ $$1 -ge 1048576 ]; then printf "%0.1f MB" $$(echo "$$1/1048576" | bc -l); \
	   elif [ $$1 -ge 1024 ]; then printf "%0.1f KB" $$(echo "$$1/1024" | bc -l); \
	   else printf "%d B" $$1; fi; \
	}; \
	echo "Binary: $(BUILD_DIR)/$(APP_NAME)"; \
	echo "  Total size:   $$(human $$TOTAL_SIZE)"; \
	echo "  Runtime size: $$(human $$RUNTIME_SIZE) ($$PCT_RUNTIME%)"; \
	echo "  Your code:    $$(human $$CUSTOM_SIZE) ($$PCT_CUSTOM%)"
	@echo "$(COLOR_GREEN)Analysis complete.$(COLOR_RESET)"

.PHONY: analyze-goweight
analyze-goweight: build ## [Analyze] Analyze binary size using goweight
	@goweight

# ==================================================================================== #
# CLEAN
# ==================================================================================== #
.PHONY: clean
clean: ## [Clean] Remove build output and coverage files
	@rm -rf $(BUILD_DIR) coverage.out
	@go clean
	@echo "$(COLOR_GREEN)Cleaned.$(COLOR_RESET)"
# ==================================================================================== #
# TEST HARNESS
# ==================================================================================== #
# Several homa instances on one machine need separate config directories, and the
# only portable way to move one is to move HOME: os.UserConfigDir reads
# XDG_CONFIG_HOME on Linux but not on macOS, where it always looks under HOME.
#
# A gets its own sandbox too, so a test run never touches the real identity or
# address book. Use `make run` for those.

TEST_A   := /tmp/homa-a
TEST_B   := /tmp/homa-b
TEST_C   := /tmp/homa-c
TEST_FILE := /tmp/homa-test-10m.bin

.PHONY: run-a
run-a: clean build ## [Test] Run instance A in its own sandbox
	@mkdir -p $(TEST_A)
	@echo "$(COLOR_BLUE)A: config in $(TEST_A), log in /tmp/a.log$(COLOR_RESET)"
	@HOME=$(TEST_A) ./$(BUILD_DIR)/$(APP_NAME) -log /tmp/a.log -debug

.PHONY: run-b
run-b: build ## [Test] Run instance B in its own sandbox
	@mkdir -p $(TEST_B)
	@echo "$(COLOR_BLUE)B: config in $(TEST_B), log in /tmp/b.log$(COLOR_RESET)"
	@HOME=$(TEST_B) ./$(BUILD_DIR)/$(APP_NAME) -log /tmp/b.log -debug

.PHONY: run-c
run-c: build ## [Test] Run instance C, for testing a busy line
	@mkdir -p $(TEST_C)
	@echo "$(COLOR_BLUE)C: config in $(TEST_C), log in /tmp/c.log$(COLOR_RESET)"
	@HOME=$(TEST_C) ./$(BUILD_DIR)/$(APP_NAME) -log /tmp/c.log -debug

.PHONY: logs
logs: ## [Test] Follow every instance log at once
	@touch /tmp/a.log /tmp/b.log /tmp/c.log
	@tail -f /tmp/a.log /tmp/b.log /tmp/c.log

.PHONY: logs-quiet
logs-quiet: ## [Test] Follow the logs, hiding the transport's own chatter
	@touch /tmp/a.log /tmp/b.log /tmp/c.log
	@tail -f /tmp/a.log /tmp/b.log /tmp/c.log \
	  | grep -Ev "wg:|magicsock:|netcheck:|derphttp|dns:|wgengine:|netstack:|NetworkMap|fakeRouter"

.PHONY: addr
addr: ## [Test] Print each sandbox's address, so it can be pasted without a menu
	@for d in $(TEST_A) $(TEST_B) $(TEST_C); do \
	   f="$$d/Library/Application Support/homa/contacts.json"; \
	   [ -f "$$f" ] || f="$$d/.config/homa/contacts.json"; \
	   echo "$(COLOR_YELLOW)$$d$(COLOR_RESET)"; \
	   [ -f "$$f" ] && cat "$$f" || echo "  no contacts yet"; \
	done

.PHONY: testfile
testfile: ## [Test] Create a 10 MB file to send with /send
	@dd if=/dev/urandom of=$(TEST_FILE) bs=1m count=10 2>/dev/null
	@echo "$(COLOR_GREEN)$(TEST_FILE)$(COLOR_RESET)"
	@shasum -a 256 $(TEST_FILE) 2>/dev/null || sha256sum $(TEST_FILE)

.PHONY: reset-test
reset-test: ## [Test] Throw away every sandbox: identities, settings, contacts, logs
	@rm -rf $(TEST_A) $(TEST_B) $(TEST_C)
	@rm -f /tmp/a.log /tmp/b.log /tmp/c.log
	@echo "$(COLOR_GREEN)Test sandboxes cleared.$(COLOR_RESET)"
