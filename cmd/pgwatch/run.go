package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bright98/pgwatch/config"
	"github.com/bright98/pgwatch/internal/watcher"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Start the polling daemon (tails the log, reports at each interval)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDaemon(cmd.Context())
		},
	}
}

func runDaemon(ctx context.Context) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	src, err := buildSource(cfg, true) // tail=true: follow new log lines
	if err != nil {
		return err
	}
	defer src.Close()

	rep, err := buildReporter(cfg)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("pgwatch: tailing %s, reporting every %s (Ctrl-C to stop)\n", cfg.LogFile, cfg.Interval)
	return watcher.New(cfg, src, rep).Run(ctx)
}
