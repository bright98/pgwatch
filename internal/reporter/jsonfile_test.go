package reporter

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/bright98/pgexplain/advisor"
	"github.com/bright98/pgwatch/internal/source"
)

func TestJSONFile_Write(t *testing.T) {
	f, err := os.CreateTemp("", "pgwatch-json-*.json")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	jr := NewJSONFile(f.Name())
	reports := []QueryReport{
		{
			Rank: 1,
			Plan: source.RawPlan{
				User:       "alice",
				Database:   "shop",
				DurationMs: 1243.821,
				Timestamp:  time.Date(2026, 5, 10, 14, 23, 1, 0, time.UTC),
				SourceName: "auto_explain",
			},
			Findings: []advisor.Finding{
				{Severity: advisor.Error, Message: "seq scan", Suggestion: "add index"},
			},
		},
	}
	if err := jr.Report(reports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	var out jsonOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("invalid json: %v\nraw: %s", err, raw)
	}
	if len(out.Reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(out.Reports))
	}
	r := out.Reports[0]
	if r.User != "alice" || r.Database != "shop" {
		t.Errorf("unexpected user/db: %s@%s", r.User, r.Database)
	}
	if r.DurationMs != 1243.821 {
		t.Errorf("unexpected duration: %v", r.DurationMs)
	}
	if len(r.Findings) != 1 || r.Findings[0].Severity != "ERROR" {
		t.Errorf("expected severity ERROR, got %+v", r.Findings)
	}
	if out.GeneratedAt.IsZero() {
		t.Error("expected generated_at to be set")
	}
}

func TestJSONFile_Empty(t *testing.T) {
	f, err := os.CreateTemp("", "pgwatch-json-*.json")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	if err := NewJSONFile(f.Name()).Report(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	raw, _ := os.ReadFile(f.Name())
	var out jsonOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(out.Reports) != 0 {
		t.Errorf("expected empty reports, got %d", len(out.Reports))
	}
}
