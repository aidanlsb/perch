.PHONY: all build test vet lint vuln check fmt clean

BINARY := perch
GOLANGCI_LINT_VERSION ?= v2.15.0

all: check build

build:
	go build -o $(BINARY) ./cmd/perch

test:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

lint:
	@version="$$(golangci-lint --version 2>/dev/null || true)"; \
	case "$$version" in \
		*"$(GOLANGCI_LINT_VERSION)"*|*"2.15."*) golangci-lint run ;; \
		"") echo "golangci-lint not found. Install $(GOLANGCI_LINT_VERSION)."; exit 1 ;; \
		*) echo "expected golangci-lint $(GOLANGCI_LINT_VERSION), found: $$version"; golangci-lint run ;; \
	esac

vuln:
	govulncheck ./...

check: test vet

clean:
	rm -f $(BINARY)
