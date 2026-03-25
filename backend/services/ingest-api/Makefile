.PHONY: run lint test integration e2e alerts-e2e perf simulate report-once report-daemon

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

alerts-e2e:
	go test -count=1 -tags=e2e ./tests/e2e -run TestIngestPipelinePersistsAlerts -v

perf:
	go test -count=1 -tags=perf ./tests/perf -v

simulate:
	go run ./cmd/simulator -miners=100 -duration=2m -tick=5s

report-once:
	go run ./cmd/reporter -mode once

report-daemon:
	go run ./cmd/reporter -mode daemon
