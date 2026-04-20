.PHONY: run lint test

run:
	go run ./cmd/api

lint:
	test -z "$(gofmt -l .)"
	go vet ./...

test:
	go test ./...
