package main

import (
	"fmt"

	"github.com/bright98/pgwatch/config"
	"github.com/bright98/pgwatch/internal/reporter"
	"github.com/bright98/pgwatch/internal/source"
	"github.com/bright98/pgwatch/internal/source/autoexplain"
)

// buildSource constructs the PlanSource selected by cfg.Source.
// tail=true is used by the run command (daemon); tail=false by report (one-shot).
func buildSource(cfg *config.Config, tail bool) (source.PlanSource, error) {
	switch cfg.Source {
	case config.SourceAutoExplain:
		return autoexplain.New(autoexplain.Config{
			LogFile: cfg.LogFile,
			Tail:    tail,
		}), nil
	default:
		return nil, fmt.Errorf("unknown source %q", cfg.Source)
	}
}

// buildReporter constructs the Reporter selected by cfg.Output.
func buildReporter(cfg *config.Config) (reporter.Reporter, error) {
	switch cfg.Output {
	case config.OutputTerminal:
		return reporter.NewTerminal(), nil
	case config.OutputJSON:
		return reporter.NewJSONFile(cfg.OutFile), nil
	case config.OutputHTML:
		return reporter.NewHTMLFile(cfg.OutFile), nil
	default:
		return nil, fmt.Errorf("unknown output %q", cfg.Output)
	}
}

// dryRunReporter satisfies the Reporter interface but only prints a summary,
// without writing to any file or external system. Used with --dry-run.
type dryRunReporter struct{}

func (d *dryRunReporter) Report(reports []reporter.QueryReport) error {
	fmt.Printf("dry-run: would report on %d plans\n", len(reports))
	for _, r := range reports {
		fmt.Printf("  #%d  %s@%s  duration=%.2fms  findings=%d\n",
			r.Rank, r.Plan.User, r.Plan.Database, r.Plan.DurationMs, len(r.Findings))
	}
	return nil
}
