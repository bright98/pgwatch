package autoexplain

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bright98/pgwatch/internal/source"
)

// sampleLog contains two valid auto_explain entries and one unrelated log line.
const sampleLog = `2026-05-10 14:23:01.887 UTC [8821] myuser@mydb LOG:  duration: 1243.821 ms  plan:
[
  {
    "Plan": {
      "Node Type": "Hash Join",
      "Startup Cost": 0.00,
      "Total Cost": 5000.00,
      "Plan Rows": 10000,
      "Plan Width": 100,
      "Actual Rows": 923847,
      "Actual Total Time": 1241.008,
      "Actual Loops": 1
    },
    "Planning Time": 2.341,
    "Execution Time": 1243.821
  }
]
2026-05-10 14:23:02.000 UTC [8822] admin@postgres LOG:  duration: 50.000 ms  plan:
[
  {
    "Plan": {
      "Node Type": "Seq Scan",
      "Startup Cost": 0.00,
      "Total Cost": 100.00,
      "Plan Rows": 1000,
      "Plan Width": 8,
      "Actual Rows": 1000,
      "Actual Total Time": 49.500,
      "Actual Loops": 1
    },
    "Planning Time": 0.100,
    "Execution Time": 50.000
  }
]
2026-05-10 14:23:03.000 UTC [8823] user2@db2 LOG:  checkpoint complete: wrote 42 buffers
`

func tempLog(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "pgwatch-log-*.log")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	f.WriteString(content)
	f.Close()
	return f.Name()
}

func collect(t *testing.T, path string) []source.RawPlan {
	t.Helper()
	src := New(Config{LogFile: path, Tail: false})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ch, err := src.Plans(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var out []source.RawPlan
	for p := range ch {
		out = append(out, p)
	}
	return out
}

func TestPlans_ParsesBothEntries(t *testing.T) {
	// pgwatch reads everything Postgres logged — filtering is Postgres's job.
	path := tempLog(t, sampleLog)
	plans := collect(t, path)
	if len(plans) != 2 {
		t.Fatalf("expected 2 plans, got %d", len(plans))
	}
}

func TestPlans_Fields(t *testing.T) {
	path := tempLog(t, sampleLog)
	plans := collect(t, path)

	p := plans[0] // first entry: myuser@mydb, 1243.821 ms
	if p.DurationMs != 1243.821 {
		t.Errorf("duration: got %.3f, want 1243.821", p.DurationMs)
	}
	if p.User != "myuser" {
		t.Errorf("user: got %q, want \"myuser\"", p.User)
	}
	if p.Database != "mydb" {
		t.Errorf("database: got %q, want \"mydb\"", p.Database)
	}
	if p.SourceName != Name {
		t.Errorf("source name: got %q, want %q", p.SourceName, Name)
	}
	if len(p.PlanJSON) == 0 {
		t.Error("expected non-empty PlanJSON")
	}
}

func TestPlans_EmptyFile(t *testing.T) {
	path := tempLog(t, "")
	plans := collect(t, path)
	if len(plans) != 0 {
		t.Errorf("expected no plans from empty file, got %d", len(plans))
	}
}

func TestPlans_FileNotFound(t *testing.T) {
	src := New(Config{LogFile: "/nonexistent/pgwatch-test.log", Tail: false})
	_, err := src.Plans(context.Background())
	if err == nil {
		t.Fatal("expected error for missing log file")
	}
}

func TestParseTimestamp(t *testing.T) {
	ts := parseTimestamp("2026-05-10 14:23:01.887 UTC")
	if ts.Year() != 2026 || ts.Month() != 5 || ts.Day() != 10 {
		t.Errorf("unexpected timestamp: %v", ts)
	}
}

// sampleLogBareObject uses the bare-object format that auto_explain emits
// (no outer array), which must be normalized before parser.Parse can handle it.
const sampleLogBareObject = `2026-05-10 15:00:00.000 UTC [9000] myuser@mydb LOG:  duration: 500.000 ms  plan:
{
  "Plan": {
    "Node Type": "Seq Scan",
    "Startup Cost": 0.00,
    "Total Cost": 200.00,
    "Plan Rows": 500,
    "Plan Width": 8,
    "Actual Rows": 500,
    "Actual Total Time": 499.000,
    "Actual Loops": 1
  },
  "Planning Time": 0.500,
  "Execution Time": 500.000
}
`

func TestPlans_BareObjectFormat(t *testing.T) {
	path := tempLog(t, sampleLogBareObject)
	plans := collect(t, path)
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan from bare-object format, got %d", len(plans))
	}
	if plans[0].DurationMs != 500.000 {
		t.Errorf("duration: got %.3f, want 500.000", plans[0].DurationMs)
	}
}

func TestPlans_NonPlanLinesIgnored(t *testing.T) {
	log := "2026-05-10 14:23:03.000 UTC [8823] user2@db2 LOG:  checkpoint complete: wrote 42 buffers\n"
	path := tempLog(t, log)
	plans := collect(t, path)
	if len(plans) != 0 {
		t.Errorf("expected no plans from non-plan log line, got %d", len(plans))
	}
}
