package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/app"
)

func main() {
	cfg := app.LoadConfig()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, cfg); err != nil {
		log.Fatal(err)
	}
}
