FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/ingest-api ./cmd/api

FROM alpine:3.20

RUN adduser -D -u 10001 appuser
USER appuser

COPY --from=builder /out/ingest-api /usr/local/bin/ingest-api

EXPOSE 8080

ENTRYPOINT ["ingest-api"]
