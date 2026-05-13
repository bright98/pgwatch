package main

import (
	"context"
	"fmt"

	"github.com/bright98/pgwatch/config"
	"github.com/bright98/pgwatch/internal/reporter"
	"github.com/bright98/pgwatch/internal/watcher"
	"github.com/spf13/cobra"
)

func newReportCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Read the log file once and exit (good for CI/cron)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runReport(cmd.Context(), dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "parse and analyze but do not output findings")
	return cmd
}

func runReport(ctx context.Context, dryRun bool) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	src, err := buildSource(cfg, false) // tail=false: read to EOF then stop
	if err != nil {
		return err
	}
	defer src.Close()

	var rep reporter.Reporter
	if dryRun {
		rep = &dryRunReporter{}
	} else {
		rep, err = buildReporter(cfg)
		if err != nil {
			return err
		}
	}

	fmt.Printf("pgwatch: reading %s\n", cfg.LogFile)
	return watcher.New(cfg, src, rep).RunOnce(ctx)
}
