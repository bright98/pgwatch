package watcher

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bright98/pgwatch/config"
	"github.com/bright98/pgwatch/internal/reporter"
	"github.com/bright98/pgwatch/internal/source"
)

// minimalPlan is a valid EXPLAIN (ANALYZE, FORMAT JSON) output with actuals.
const minimalPlan = `[{
  "Plan": {
    "Node Type": "Seq Scan",
    "Relation Name": "orders",
    "Startup Cost": 0.00,
    "Total Cost": 100.00,
    "Plan Rows": 100,
    "Plan Width": 8,
    "Actual Rows": 50000,
    "Actual Total Time": 99.0,
    "Actual Loops": 1
  },
  "Planning Time": 0.5,
  "Execution Time": 99.5
}]`

// --- mocks ---

type mockSource struct {
	plans []source.RawPlan
	err   error
}

func (m *mockSource) Name() string { return "mock" }
func (m *mockSource) Close() error { return nil }
func (m *mockSource) Plans(_ context.Context) (<-chan source.RawPlan, error) {
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan source.RawPlan, len(m.plans))
	for _, p := range m.plans {
		ch <- p
	}
	close(ch)
	return ch, nil
}

type mockReporter struct {
	got       []reporter.QueryReport
	callCount int
	err       error
}

func (m *mockReporter) Report(r []reporter.QueryReport) error {
	m.got = r
	m.callCount++
	return m.err
}

func testConfig() *config.Config {
	return &config.Config{
		Source:        "auto_explain",
		LogFile:       "/tmp/pg.log",
		Interval: time.Hour,
		Output:        "terminal",
	}
}

func rawPlan(durMs float64) source.RawPlan {
	return source.RawPlan{
		PlanJSON:   []byte(minimalPlan),
		DurationMs: durMs,
		Timestamp:  time.Now(),
		User:       "testuser",
		Database:   "testdb",
		SourceName: "mock",
	}
}

// --- tests ---

func TestRunOnce_HappyPath(t *testing.T) {
	src := &mockSource{plans: []source.RawPlan{rawPlan(1500), rawPlan(2000)}}
	rep := &mockReporter{}

	w := New(testConfig(), src, rep)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rep.got) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(rep.got))
	}
	if rep.got[0].Rank != 1 || rep.got[1].Rank != 2 {
		t.Errorf("unexpected ranks: %d, %d", rep.got[0].Rank, rep.got[1].Rank)
	}
}

func TestRunOnce_SourceError(t *testing.T) {
	src := &mockSource{err: errors.New("log file missing")}
	w := New(testConfig(), src, &mockReporter{})
	err := w.RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected error from source failure")
	}
}

func TestRunOnce_InvalidPlanJSON(t *testing.T) {
	bad := source.RawPlan{PlanJSON: []byte("not json"), DurationMs: 1500, SourceName: "mock"}
	src := &mockSource{plans: []source.RawPlan{bad, rawPlan(2000)}}
	rep := &mockReporter{}

	w := New(testConfig(), src, rep)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Bad plan is skipped; only the valid one is reported.
	if len(rep.got) != 1 {
		t.Errorf("expected 1 report (bad plan skipped), got %d", len(rep.got))
	}
}

func TestRunOnce_Empty(t *testing.T) {
	src := &mockSource{}
	rep := &mockReporter{}
	w := New(testConfig(), src, rep)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(rep.got) != 0 {
		t.Errorf("expected empty report, got %d items", len(rep.got))
	}
}

func TestRun_EarlyFlushOnBufferLimit(t *testing.T) {
	// Set limit to 2 so the buffer flushes after 2 plans, before the ticker fires.
	cfg := testConfig()
	cfg.MaxBufferedPlans = 2
	cfg.Interval = time.Hour // ticker should never fire in this test

	src := &mockSource{plans: []source.RawPlan{
		rawPlan(1000), rawPlan(1100), rawPlan(1200),
	}}
	rep := &mockReporter{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	w := New(cfg, src, rep)
	if err := w.Run(ctx); err != nil {
		t.Fatal(err)
	}

	// Buffer hit limit at 2 → early flush of 2 plans.
	// Source then closed → final flush of remaining 1 plan.
	// reporter.Report was called twice; rep.got holds the last call (1 plan).
	// Total plans across both calls = 3.
	if rep.callCount < 2 {
		t.Errorf("expected at least 2 Report calls (early flush + final), got %d", rep.callCount)
	}
}

func TestRun_FinalFlushOnShutdown(t *testing.T) {
	cfg := testConfig()
	cfg.Interval = time.Hour // ticker never fires

	// Use a non-closing source to simulate a live tail; cancel ctx to trigger shutdown.
	live := make(chan source.RawPlan, 10)
	live <- rawPlan(1500)
	live <- rawPlan(2000)

	src := &chanSource{ch: live}
	rep := &mockReporter{}

	ctx, cancel := context.WithCancel(context.Background())
	w := New(cfg, src, rep)

	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	// Let the plans arrive in the buffer, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()

	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if rep.callCount == 0 {
		t.Error("expected final flush on shutdown, but Report was never called")
	}
}

// chanSource is a PlanSource backed by a raw channel (never closed automatically).
type chanSource struct{ ch chan source.RawPlan }

func (c *chanSource) Name() string { return "chan" }
func (c *chanSource) Close() error { return nil }
func (c *chanSource) Plans(_ context.Context) (<-chan source.RawPlan, error) {
	return c.ch, nil
}
