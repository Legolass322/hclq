VERSION ?= dev

.PHONY: test build

test:
	go test ./...

build:
	go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o hclq ./cmd/hclq
