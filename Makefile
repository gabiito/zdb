.PHONY: build test test-integration lint fmt run

build:
	CGO_ENABLED=0 go build -o bin/zdb ./cmd/zdb

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

lint:
	go vet ./...

fmt:
	gofmt -w .

run:
	go run ./cmd/zdb
