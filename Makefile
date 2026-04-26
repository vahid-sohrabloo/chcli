VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS  = -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)

.PHONY: build install test test-integration lint clean demo demo-monitor demo-all

build:
	go build -ldflags "$(LDFLAGS)" -o chcli ./cmd/chcli

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/chcli

test:
	go test -race ./...

# Run the chtop integration tests against every ClickHouse version in the
# CI matrix (requires Docker). Pass VERSIONS to narrow the set, e.g.
#   make test-integration VERSIONS="25.3 latest"
test-integration:
	./scripts/test-versions.sh $(VERSIONS)

lint:
	golangci-lint run ./...

# Re-record the README demo. Requires `vhs` (https://github.com/charmbracelet/vhs)
# and a local ClickHouse on 127.0.0.1:9000. Output: demo/demo.gif
demo: build
	vhs demo/demo.tape

# Re-record the \monitor demo. Output: demo/monitor.gif
demo-monitor: build
	vhs demo/monitor.tape

# Regenerate every README GIF in one shot.
demo-all: demo demo-monitor

clean:
	rm -f chcli

generate:
	go run ./cmd/chcli-gen -connstr "clickhouse://default@localhost:9000/default"
