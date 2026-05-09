.PHONY: build test test-integration lint fmt run

build:
	CGO_ENABLED=0 go build -o bin/dbviewer ./cmd/dbviewer

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

lint:
	go vet ./...

fmt:
	gofmt -w .

run:
	go run ./cmd/dbviewer
