.PHONY: build test clean dev

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

build:
	@command -v npm >/dev/null 2>&1 || { echo "ERROR: npm is required to build Cloudrift. Install Node.js from https://nodejs.org"; exit 1; }
	npm ci --prefix dashboard
	npm run build --prefix dashboard
	go build -ldflags="-s -w -X main.version=$(VERSION)" -o cloudrift ./cmd/cloudrift

dev:
	go build -ldflags="-X main.version=$(VERSION)" -o cloudrift ./cmd/cloudrift

test:
	go test ./...

clean:
	rm -f cloudrift
	rm -rf dashboard/dist
