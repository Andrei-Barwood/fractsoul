.PHONY: run lint test e2e

run:
	go run ./cmd/api

lint:
	test -z "$(gofmt -l .)"
	go vet ./...

test:
	go test ./...

e2e:
	go test -tags=e2e ./tests/e2e -v
