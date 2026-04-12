VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS  = -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)

.PHONY: build install test lint clean

build:
	go build -ldflags "$(LDFLAGS)" -o chcli ./cmd/chcli

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/chcli

test:
	go test -race ./...

lint:
	golangci-lint run ./...

clean:
	rm -f chcli

generate:
	go run ./cmd/chcli-gen -connstr "clickhouse://default@localhost:9000/default"
