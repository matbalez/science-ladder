package main

import (
	"context"
	"github.com/matbalez/science-ladder/internal/platform"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	s, err := platform.New(ctx, platform.LoadConfig())
	if err != nil {
		slog.Error("worker initialization failed", "error", err)
		os.Exit(1)
	}
	defer s.DB.Close()
	if err = s.RunWorker(ctx); err != nil {
		slog.Error("worker failed", "error", err)
		os.Exit(1)
	}
}
