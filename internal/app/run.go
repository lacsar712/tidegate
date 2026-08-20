package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/lacsar712/tidegate/internal/config"
)

// RunFromEnv loads configuration and blocks until shutdown.
func RunFromEnv() error {
	cfg := config.Default()
	app, err := New(cfg)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fmt.Printf("tidegate listening on %s\n", cfg.ListenAddr)
	return app.Run(ctx)
}

// MustRunFromEnv exits the process on startup failure.
func MustRunFromEnv() {
	if err := RunFromEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "tidegate fatal: %v\n", err)
		os.Exit(1)
	}
}
