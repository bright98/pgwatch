package reporter

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bright98/pgexplain/advisor"
	"github.com/bright98/pgwatch/internal/source"
)

func TestHTMLFile_WithFindings(t *testing.T) {
	f, err := os.CreateTemp("", "pgwatch-*.html")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	reports := []QueryReport{
		{
			Rank: 1,
			Plan: source.RawPlan{
				User:       "alice",
				Database:   "shop",
				DurationMs: 1243.821,
				Timestamp:  time.Date(2026, 5, 10, 14, 23, 1, 0, time.UTC),
				SourceName: "auto_explain",
				PlanJSON:   []byte(`[{"Plan":{"Node Type":"Seq Scan"}}]`),
			},
			Findings: []advisor.Finding{
				{
					Severity:   advisor.Error,
					NodeType:   "Seq Scan",
					Message:    "large sequential scan detected",
					Detail:     "The planner chose a sequential scan over a large table.",
					Suggestion: "Add an index on the filter column.",
				},
				{
					Severity: advisor.Warn,
					Message:  "row estimate mismatch",
				},
			},
		},
	}

	h := NewHTMLFile(f.Name())
	if err := h.Report(reports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	html := string(raw)

	for _, want := range []string{
		"<!DOCTYPE html>",
		"pgwatch",
		"alice@shop",
		"1243.82 ms",
		"auto_explain",
		"large sequential scan detected",
		"Add an index on the filter column.",
		"row estimate mismatch",
		"badge-error",
		"badge-warn",
		"Raw plan JSON",
		"Seq Scan",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q in HTML output", want)
		}
	}
}

func TestHTMLFile_Empty(t *testing.T) {
	f, err := os.CreateTemp("", "pgwatch-*.html")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	h := NewHTMLFile(f.Name())
	if err := h.Report(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, _ := os.ReadFile(f.Name())
	html := string(raw)
	if !strings.Contains(html, "No plans found") {
		t.Errorf("expected empty-state message in output:\n%s", html)
	}
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("output is not valid HTML")
	}
}

func TestHTMLFile_NoFindings(t *testing.T) {
	f, err := os.CreateTemp("", "pgwatch-*.html")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	reports := []QueryReport{
		{Rank: 1, Plan: source.RawPlan{User: "u", Database: "d", DurationMs: 500, SourceName: "auto_explain"}},
	}
	h := NewHTMLFile(f.Name())
	h.Report(reports)
	raw, _ := os.ReadFile(f.Name())
	if !strings.Contains(string(raw), "No issues found") {
		t.Errorf("expected 'No issues found' label in output")
	}
}

func TestHTMLFile_SeverityCounts(t *testing.T) {
	f, err := os.CreateTemp("", "pgwatch-*.html")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	reports := []QueryReport{
		{
			Rank: 1,
			Plan: source.RawPlan{User: "u", Database: "d", DurationMs: 1000, SourceName: "auto_explain"},
			Findings: []advisor.Finding{
				{Severity: advisor.Error, Message: "e1"},
				{Severity: advisor.Error, Message: "e2"},
				{Severity: advisor.Warn, Message: "w1"},
			},
		},
	}
	h := NewHTMLFile(f.Name())
	h.Report(reports)
	raw, _ := os.ReadFile(f.Name())
	html := string(raw)

	// The summary should show 2 errors and 1 warning
	if !strings.Contains(html, "2 errors") {
		t.Errorf("expected '2 errors' in summary")
	}
	if !strings.Contains(html, "1 warnings") {
		t.Errorf("expected '1 warnings' in summary")
	}
}

func TestSeverityClass(t *testing.T) {
	cases := []struct {
		in   advisor.Severity
		want string
	}{
		{advisor.Error, "error"},
		{advisor.Warn, "warn"},
		{advisor.Info, "info"},
	}
	for _, c := range cases {
		got := severityClass(c.in)
		if got != c.want {
			t.Errorf("severityClass(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPrettyJSON_Valid(t *testing.T) {
	raw := []byte(`{"a":1}`)
	got := prettyJSON(raw)
	if !strings.Contains(got, "\n") {
		t.Errorf("expected indented JSON, got: %s", got)
	}
}

func TestPrettyJSON_Invalid(t *testing.T) {
	raw := []byte(`not json`)
	got := prettyJSON(raw)
	if got != "not json" {
		t.Errorf("expected fallback to raw string, got: %s", got)
	}
}
