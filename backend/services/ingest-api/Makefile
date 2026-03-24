.PHONY: run lint test e2e simulate

run:
	go run ./cmd/api

lint:
	test -z "$(gofmt -l .)"
	go vet ./...

test:
	go test ./...

e2e:
	go test -count=1 -tags=e2e ./tests/e2e -v

simulate:
	go run ./cmd/simulator -miners=100 -duration=2m -tick=5s
