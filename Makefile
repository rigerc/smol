GO ?= go
export GOTOOLCHAIN ?= go1.26.10
BIN := dist/smol
LINTER_BIN := ./custom-gcl
COVERAGE_FILE := coverage.out
COVERAGE_MIN := 80

.PHONY: check fix format format-check lint test test-coverage build run demos audit hooks-install clean

check: format-check lint test-coverage build

fix: format

format:
	@find cmd internal -type f -name '*.go' -exec gofmt -w {} +

format-check:
	@unformatted="$$(find cmd internal -type f -name '*.go' -exec gofmt -l {} +)"; \
	if [ -n "$$unformatted" ]; then \
	  printf 'Files need gofmt:\n%s\n' "$$unformatted"; exit 1; \
	fi

# The plugin cannot be loaded by stock golangci-lint: build the pinned binary.
# A change to .custom-gcl.yml rebuilds the tool on the next check.
$(LINTER_BIN): .custom-gcl.yml
	@command -v golangci-lint >/dev/null 2>&1 || { \
	  echo 'golangci-lint is required to build the pinned custom linter (v2.10.1)'; exit 1; \
	}
	golangci-lint custom

lint: $(LINTER_BIN)
	$(GO) vet ./...
	$(LINTER_BIN) run ./...

test:
	$(GO) test -race ./...

test-coverage:
	$(GO) test -race ./...
	$(GO) test -race -covermode=atomic -coverprofile=$(COVERAGE_FILE) ./internal/...
	@$(GO) tool cover -func=$(COVERAGE_FILE) | awk -v min=$(COVERAGE_MIN) \
	  '/^total:/ { gsub(/%/, "", $$3); printf "Coverage: %.1f%% (minimum: %s%%)\n", $$3, min; found = 1; if ($$3 + 0 < min) exit 1 } \
	   END { if (!found) { print "No coverage total found" > "/dev/stderr"; exit 1 } }'

build:
	@mkdir -p dist
	$(GO) build -trimpath -o $(BIN) ./cmd/smol

run:
	$(GO) run ./cmd/smol

# Refresh the committed README gifs; requires VHS (https://github.com/charmbracelet/vhs).
demos:
	cd demos/vhs && vhs demo.tape && vhs wizard.tape
	cp demos/vhs/outputs/demo.gif demos/vhs/demo.gif
	cp demos/vhs/outputs/wizard.gif demos/vhs/wizard.gif

audit:
	@command -v govulncheck >/dev/null 2>&1 || { \
	  echo 'govulncheck is required: https://go.dev/doc/tutorial/govulncheck'; exit 1; \
	}
	govulncheck ./...

hooks-install:
	git config --local core.hooksPath .githooks

clean:
	rm -rf dist $(COVERAGE_FILE) $(LINTER_BIN)
