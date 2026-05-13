package reporter

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/bright98/pgexplain/advisor"
	"github.com/bright98/pgwatch/internal/source"
)

func makePlan(user, db string, durMs float64) source.RawPlan {
	return source.RawPlan{
		User:       user,
		Database:   db,
		DurationMs: durMs,
		Timestamp:  time.Date(2026, 5, 10, 14, 23, 1, 0, time.UTC),
		SourceName: "auto_explain",
	}
}

func TestTerminal_Empty(t *testing.T) {
	var buf bytes.Buffer
	tr := &Terminal{w: &buf}
	if err := tr.Report(nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "no plans found") {
		t.Errorf("expected 'no plans found' message, got: %s", buf.String())
	}
}

func TestTerminal_WithFindings(t *testing.T) {
	var buf bytes.Buffer
	tr := &Terminal{w: &buf}
	reports := []QueryReport{
		{
			Rank: 1,
			Plan: makePlan("myuser", "mydb", 1243.821),
			Findings: []advisor.Finding{
				{
					Severity:   advisor.Warn,
					NodeID:     2,
					NodeType:   "Seq Scan",
					Message:    "large sequential scan",
					Suggestion: "add an index on the filter column",
				},
			},
		},
	}
	if err := tr.Report(reports); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"myuser@mydb",
		"duration=1243.82ms",
		"auto_explain",
		"Seq Scan",
		"large sequential scan",
		"add an index",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in terminal output:\n%s", want, out)
		}
	}
}

func TestTerminal_NoFindings(t *testing.T) {
	var buf bytes.Buffer
	tr := &Terminal{w: &buf}
	reports := []QueryReport{
		{Rank: 1, Plan: makePlan("u", "d", 500)},
	}
	tr.Report(reports)
	if !strings.Contains(buf.String(), "(no findings)") {
		t.Errorf("expected '(no findings)' label:\n%s", buf.String())
	}
}
