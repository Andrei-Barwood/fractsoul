.PHONY: run lint test integration e2e perf simulate

run:
	go run ./cmd/api

lint:
	test -z "$(gofmt -l .)"
	go vet ./...

test:
	go test ./...

integration:
	go test -count=1 -tags=integration ./tests/integration -v

e2e:
	go test -count=1 -tags=e2e ./tests/e2e -v

perf:
	go test -count=1 -tags=perf ./tests/perf -v

simulate:
	go run ./cmd/simulator -miners=100 -duration=2m -tick=5s
