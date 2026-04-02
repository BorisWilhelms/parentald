VERSION ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
LDFLAGS := -X github.com/BorisWilhelms/parentald/internal/version.Version=$(VERSION)

.PHONY: build build-server build-daemon build-all clean test

# Build both for current platform
build: build-server build-daemon

build-server:
	go build -ldflags "$(LDFLAGS)" -o parentald-server ./cmd/server/

build-daemon:
	go build -ldflags "$(LDFLAGS)" -o dist/parentald-$(shell go env GOOS)-$(shell go env GOARCH) ./cmd/daemon/

# Cross-compile daemon for all target platforms
build-all: build-server
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/parentald-linux-amd64 ./cmd/daemon/
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/parentald-linux-arm64 ./cmd/daemon/

test:
	go test ./internal/...

clean:
	rm -rf parentald-server dist/
