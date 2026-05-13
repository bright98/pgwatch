package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bright98/pgexplain/advisor"
	"github.com/bright98/pgexplain/parser"
	"github.com/bright98/pgexplain/rules"
	"github.com/bright98/pgwatch/config"
	"github.com/bright98/pgwatch/internal/reporter"
	"github.com/bright98/pgwatch/internal/source"
)

type Watcher struct {
	cfg      *config.Config
	src      source.PlanSource
	adv      *advisor.Advisor
	reporter reporter.Reporter
}

func New(cfg *config.Config, src source.PlanSource, rep reporter.Reporter) *Watcher {
	adv := advisor.New(
		rules.SeqScan(),
		rules.RowEstimateMismatch(),
		rules.HashJoinSpill(),
		rules.NestedLoopLarge(),
		rules.MissingIndexOnlyScan(),
		rules.SortSpill(),
		rules.TopNHeapsort(),
		rules.ParallelNotLaunched(),
	)
	return &Watcher{cfg: cfg, src: src, adv: adv, reporter: rep}
}

// RunOnce reads all plans from src until the channel closes, then reports.
// Designed for one-shot mode where the source stops at EOF.
func (w *Watcher) RunOnce(ctx context.Context) error {
	plans, err := w.src.Plans(ctx)
	if err != nil {
		return fmt.Errorf("watcher: open source: %w", err)
	}

	var buf []source.RawPlan
	for {
		select {
		case p, ok := <-plans:
			if !ok {
				return w.flush(buf)
			}
			buf = append(buf, p)
		case <-ctx.Done():
			return nil
		}
	}
}

// Run reads plans continuously and flushes findings to the reporter every cfg.Interval.
// Blocks until ctx is cancelled.
func (w *Watcher) Run(ctx context.Context) error {
	plans, err := w.src.Plans(ctx)
	if err != nil {
		return fmt.Errorf("watcher: open source: %w", err)
	}

	var buf []source.RawPlan
	ticker := time.NewTicker(w.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case p, ok := <-plans:
			if !ok {
				return w.flush(buf)
			}
			buf = append(buf, p)
			if len(buf) >= w.cfg.MaxBufferedPlans {
				slog.Warn("buffer limit reached, flushing early",
					"limit", w.cfg.MaxBufferedPlans)
				if err := w.flush(buf); err != nil {
					slog.Error("report failed", "err", err)
				}
				buf = buf[:0]
			}
		case <-ticker.C:
			if err := w.flush(buf); err != nil {
				slog.Error("report failed", "err", err)
			}
			buf = buf[:0]
		case <-ctx.Done():
			if len(buf) > 0 {
				_ = w.flush(buf)
			}
			return nil
		}
	}
}

// flush parses and analyzes each buffered plan then calls the reporter.
func (w *Watcher) flush(plans []source.RawPlan) error {
	var reports []reporter.QueryReport
	for i, p := range plans {
		plan, err := parser.Parse(p.PlanJSON)
		if err != nil {
			slog.Warn("plan parse failed, skipping", "source", p.SourceName, "err", err)
			continue
		}
		reports = append(reports, reporter.QueryReport{
			Rank:     i + 1,
			Plan:     p,
			Findings: w.adv.Analyze(plan),
		})
	}
	if err := w.reporter.Report(reports); err != nil {
		return fmt.Errorf("watcher: report: %w", err)
	}
	return nil
}
