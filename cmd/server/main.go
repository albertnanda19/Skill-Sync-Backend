package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"skill-sync/internal/app"
	"skill-sync/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	bootstrap, cleanup, err := app.Bootstrap(cfg)
	if err != nil {
		log.Fatalf("failed to bootstrap app: %v", err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			log.Printf("cleanup error: %v", err)
		}
	}()

	addr, err := app.ListenAddr(cfg.App.HTTPPort)
	if err != nil {
		log.Fatalf("invalid HTTP port: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- bootstrap.Fiber.Listen(addr)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil {
			log.Fatalf("server error: %v", err)
		}
	case <-sigCh:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := bootstrap.Fiber.ShutdownWithContext(ctx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}
}
