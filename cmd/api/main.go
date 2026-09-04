package main

import (
	"context"
	"github.com/matbalez/science-ladder/internal/platform"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	s, err := platform.New(ctx, platform.LoadConfig())
	if err != nil {
		slog.Error("API initialization failed", "error", err)
		os.Exit(1)
	}
	defer s.DB.Close()
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		if err = s.Migrate(ctx); err != nil {
			slog.Error("migration failed", "error", err)
			os.Exit(1)
		}
		slog.Info("database migrations complete")
		return
	}
	server := &http.Server{Addr: s.Config.ListenAddr, Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	go func() {
		<-ctx.Done()
		shutdown, c := context.WithTimeout(context.Background(), 15*time.Second)
		defer c()
		server.Shutdown(shutdown)
	}()
	if s.Config.RunnerListenAddr != "" {
		go func() {
			if e := s.RunRunnerListener(ctx); e != nil && e != http.ErrServerClosed {
				slog.Error("runner listener failed", "error", e)
				cancel()
			}
		}()
	}
	slog.Info("API listening", "address", s.Config.ListenAddr)
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("API server failed", "error", err)
		os.Exit(1)
	}
}
