package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"strings"
	"time"

	"github.com/bright98/pgexplain/advisor"
)

// reportTmpl is parsed once at startup. template.Must panics if the template
// syntax is invalid, catching authoring errors at development time.
var reportTmpl = template.Must(template.New("report").Parse(htmlTemplate))

// htmlData is the top-level context passed to the template.
type htmlData struct {
	GeneratedAt string
	TotalPlans  int
	ErrorCount  int
	WarnCount   int
	InfoCount   int
	Reports     []htmlReport
}

type htmlReport struct {
	Rank       int
	Timestamp  string
	Who        string // "user@database"
	Duration   string // "1243.82 ms"
	SourceName string
	PlanJSON   string // pretty-printed JSON for the collapsible block
	Findings   []htmlFinding
}

type htmlFinding struct {
	SeverityClass string // "error" | "warn" | "info"  — used as CSS modifier
	SeverityLabel string // "ERROR" | "WARN" | "INFO"  — displayed in the badge
	NodeID        int
	NodeType      string
	Message       string
	Detail        string
	Suggestion    string
}

// HTMLFile writes a self-contained HTML report to a file.
// All CSS is inlined — no external dependencies, works offline.
type HTMLFile struct {
	path string
}

// NewHTMLFile returns an HTMLFile that writes reports to path.
func NewHTMLFile(path string) *HTMLFile {
	return &HTMLFile{path: path}
}

func (h *HTMLFile) Report(reports []QueryReport) error {
	data := buildHTMLData(reports)

	var buf bytes.Buffer
	if err := reportTmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("reporter: render html: %w", err)
	}
	if err := os.WriteFile(h.path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("reporter: write %q: %w", h.path, err)
	}
	return nil
}

// buildHTMLData converts []QueryReport into the template context, computing
// severity counts and formatting all values as plain strings so the template
// stays logic-free.
func buildHTMLData(reports []QueryReport) htmlData {
	data := htmlData{
		GeneratedAt: time.Now().UTC().Format("2 Jan 2006, 15:04:05 UTC"),
		TotalPlans:  len(reports),
	}
	for _, r := range reports {
		hr := htmlReport{
			Rank:       r.Rank,
			Timestamp:  r.Plan.Timestamp.UTC().Format("2006-01-02 15:04:05 UTC"),
			Who:        r.Plan.User + "@" + r.Plan.Database,
			Duration:   fmt.Sprintf("%.2f ms", r.Plan.DurationMs),
			SourceName: r.Plan.SourceName,
			PlanJSON:   prettyJSON(r.Plan.PlanJSON),
		}
		for _, f := range r.Findings {
			cls := severityClass(f.Severity)
			switch f.Severity {
			case advisor.Error:
				data.ErrorCount++
			case advisor.Warn:
				data.WarnCount++
			default:
				data.InfoCount++
			}
			hr.Findings = append(hr.Findings, htmlFinding{
				SeverityClass: cls,
				SeverityLabel: strings.ToUpper(cls),
				NodeID:        f.NodeID,
				NodeType:      f.NodeType,
				Message:       f.Message,
				Detail:        f.Detail,
				Suggestion:    f.Suggestion,
			})
		}
		data.Reports = append(data.Reports, hr)
	}
	return data
}

// severityClass maps an advisor.Severity to a CSS class modifier.
func severityClass(s advisor.Severity) string {
	switch s {
	case advisor.Error:
		return "error"
	case advisor.Warn:
		return "warn"
	case advisor.Info:
		return "info"
	default:
		return "info"
	}
}

// prettyJSON attempts to re-indent raw JSON bytes. Falls back to the raw string
// if the bytes are not valid JSON (e.g. an empty or malformed plan).
func prettyJSON(raw []byte) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return string(raw)
	}
	return buf.String()
}

// htmlTemplate is the single-file self-contained report template.
// html/template auto-escapes all interpolated values, so user-supplied strings
// (query text, pg usernames, etc.) cannot inject HTML or JS.
const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>pgwatch report</title>
<style>
*, *::before, *::after { box-sizing: border-box; }

body {
  margin: 0;
  padding: 2rem 1rem;
  background: #f1f5f9;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, sans-serif;
  color: #1e293b;
  font-size: 15px;
  line-height: 1.6;
}

.container { max-width: 860px; margin: 0 auto; }

/* ── Header ── */
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 1.5rem;
  flex-wrap: wrap;
  gap: 0.5rem;
}
.brand { display: flex; align-items: center; gap: 0.625rem; }
.brand-icon {
  width: 2.125rem; height: 2.125rem;
  background: #4f46e5;
  border-radius: 0.5rem;
  display: flex; align-items: center; justify-content: center;
  color: #fff; font-size: 1.1rem;
}
.brand h1 { margin: 0; font-size: 1.375rem; font-weight: 700; color: #0f172a; }
.generated { font-size: 0.78rem; color: #94a3b8; }

/* ── Summary chips ── */
.summary { display: flex; gap: 0.5rem; flex-wrap: wrap; margin-bottom: 1.75rem; }
.chip {
  display: inline-flex; align-items: center; gap: 0.375rem;
  padding: 0.3rem 0.75rem;
  border-radius: 9999px;
  font-size: 0.78rem; font-weight: 600;
}
.chip-dot { width: 7px; height: 7px; border-radius: 50%; }
.chip-total { background: #e2e8f0; color: #475569; }
.chip-total .chip-dot { background: #94a3b8; }
.chip-error { background: #fee2e2; color: #b91c1c; }
.chip-error .chip-dot { background: #ef4444; }
.chip-warn  { background: #fef3c7; color: #92400e; }
.chip-warn  .chip-dot { background: #f59e0b; }
.chip-info  { background: #dbeafe; color: #1d4ed8; }
.chip-info  .chip-dot { background: #3b82f6; }

/* ── Cards ── */
.card {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 0.875rem;
  margin-bottom: 1rem;
  overflow: hidden;
  box-shadow: 0 1px 3px rgba(0,0,0,0.05);
  transition: box-shadow 0.15s;
}
.card:hover { box-shadow: 0 4px 12px rgba(0,0,0,0.08); }

.card-header {
  display: flex; align-items: center; gap: 0.75rem;
  padding: 0.875rem 1.25rem;
  border-bottom: 1px solid #f1f5f9;
  flex-wrap: wrap;
}
.rank {
  flex-shrink: 0;
  width: 2rem; height: 2rem;
  background: #4f46e5; color: #fff;
  border-radius: 0.5rem;
  display: flex; align-items: center; justify-content: center;
  font-size: 0.72rem; font-weight: 700;
}
.who { font-weight: 600; font-size: 0.9rem; }
.duration {
  font-family: 'SFMono-Regular', Consolas, monospace;
  font-size: 0.78rem; font-weight: 600;
  padding: 0.15rem 0.5rem;
  border-radius: 0.375rem;
  background: #fef2f2; color: #dc2626;
}
.source-tag {
  font-size: 0.68rem; font-weight: 500;
  padding: 0.15rem 0.5rem;
  border-radius: 0.375rem;
  background: #f1f5f9; color: #64748b;
  border: 1px solid #e2e8f0;
}
.ts { margin-left: auto; font-size: 0.73rem; color: #94a3b8; }

/* ── Findings list ── */
.findings { list-style: none; margin: 0; padding: 0.875rem 1.25rem; display: flex; flex-direction: column; gap: 0.5rem; }

.finding {
  display: flex; gap: 0.75rem; align-items: flex-start;
  padding: 0.75rem; border-radius: 0.625rem;
}
.finding-error { background: #fff5f5; border: 1px solid #fed7d7; }
.finding-warn  { background: #fffbeb; border: 1px solid #fde68a; }
.finding-info  { background: #ebf8ff; border: 1px solid #bee3f8; }

.badge {
  flex-shrink: 0;
  font-size: 0.58rem; font-weight: 800; letter-spacing: 0.08em;
  padding: 0.2rem 0.45rem;
  border-radius: 0.25rem;
  color: #fff; margin-top: 0.2rem;
}
.badge-error { background: #e53e3e; }
.badge-warn  { background: #dd6b20; }
.badge-info  { background: #3182ce; }

.finding-body { flex: 1; min-width: 0; }
.finding-node {
  font-size: 0.7rem; color: #a0aec0; margin-bottom: 0.2rem;
  font-family: 'SFMono-Regular', Consolas, monospace;
}
.finding-msg  { font-size: 0.875rem; font-weight: 600; color: #1a202c; }
.finding-detail    { font-size: 0.8rem; color: #4a5568; margin-top: 0.3rem; }
.finding-suggestion { font-size: 0.8rem; color: #2b6cb0; margin-top: 0.3rem; }
.finding-key {
  display: inline-block;
  font-size: 0.6rem; font-weight: 700;
  text-transform: uppercase; letter-spacing: 0.07em;
  padding: 0.1rem 0.4rem;
  border-radius: 0.25rem;
  margin-right: 0.4rem;
  vertical-align: baseline;
  background: rgba(0,0,0,0.07);
  color: inherit; opacity: 0.8;
}

/* ── No issues ── */
.no-issues {
  margin: 0.875rem 1.25rem; padding: 0.625rem 0.875rem;
  background: #f0fff4; border: 1px solid #c6f6d5;
  border-radius: 0.5rem;
  font-size: 0.825rem; font-weight: 500; color: #276749;
  display: flex; align-items: center; gap: 0.5rem;
}
.no-issues::before { content: '✓'; font-weight: 700; color: #38a169; }

/* ── Collapsible plan JSON ── */
details.plan-json { border-top: 1px solid #f1f5f9; }
details.plan-json > summary {
  list-style: none;
  padding: 0.5rem 1.25rem;
  font-size: 0.73rem; color: #94a3b8;
  cursor: pointer; user-select: none;
  display: flex; align-items: center; gap: 0.375rem;
}
details.plan-json > summary::-webkit-details-marker { display: none; }
details.plan-json > summary::before {
  content: '▶'; font-size: 0.55rem;
  transition: transform 0.15s ease;
}
details[open].plan-json > summary::before { transform: rotate(90deg); }
details.plan-json > summary:hover { color: #475569; background: #f8fafc; }
details.plan-json pre {
  margin: 0; padding: 1rem 1.25rem;
  background: #f8fafc;
  font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
  font-size: 0.7rem; line-height: 1.7; color: #334155;
  overflow-x: auto; border-top: 1px solid #e2e8f0;
}

/* ── Empty state ── */
.empty { text-align: center; padding: 5rem 2rem; color: #94a3b8; }
.empty-icon { font-size: 2.5rem; margin-bottom: 1rem; }
.empty h2 { font-size: 1.125rem; color: #64748b; font-weight: 600; margin-bottom: 0.5rem; }
.empty p  { font-size: 0.875rem; }

/* ── Footer ── */
.footer { margin-top: 3rem; text-align: center; font-size: 0.73rem; color: #cbd5e1; }
.footer a { color: #94a3b8; text-decoration: none; }
.footer a:hover { text-decoration: underline; }

@media (max-width: 600px) {
  body { padding: 1rem 0.75rem; }
  .ts { margin-left: 0; order: 5; width: 100%; }
}
</style>
</head>
<body>
<div class="container">

  <header class="header">
    <div class="brand">
      <div class="brand-icon">⚡</div>
      <h1>pgwatch</h1>
    </div>
    <span class="generated">{{.GeneratedAt}}</span>
  </header>

  <div class="summary">
    <span class="chip chip-total"><span class="chip-dot"></span>{{.TotalPlans}} plans</span>
    {{if .ErrorCount}}<span class="chip chip-error"><span class="chip-dot"></span>{{.ErrorCount}} errors</span>{{end}}
    {{if .WarnCount}}<span class="chip chip-warn"><span class="chip-dot"></span>{{.WarnCount}} warnings</span>{{end}}
    {{if .InfoCount}}<span class="chip chip-info"><span class="chip-dot"></span>{{.InfoCount}} infos</span>{{end}}
  </div>

  {{if .Reports}}
    {{range .Reports}}
    <div class="card">
      <div class="card-header">
        <span class="rank">#{{.Rank}}</span>
        <span class="who">{{.Who}}</span>
        <span class="duration">{{.Duration}}</span>
        <span class="source-tag">{{.SourceName}}</span>
        <span class="ts">{{.Timestamp}}</span>
      </div>

      {{if .Findings}}
      <ul class="findings">
        {{range .Findings}}
        <li class="finding finding-{{.SeverityClass}}">
          <span class="badge badge-{{.SeverityClass}}">{{.SeverityLabel}}</span>
          <div class="finding-body">
            {{if .NodeType}}<div class="finding-node">{{if .NodeID}}node {{.NodeID}} · {{end}}{{.NodeType}}</div>{{end}}
            <div class="finding-msg">{{.Message}}</div>
            {{if .Detail}}<div class="finding-detail"><span class="finding-key">detail</span>{{.Detail}}</div>{{end}}
            {{if .Suggestion}}<div class="finding-suggestion"><span class="finding-key">suggestion</span>{{.Suggestion}}</div>{{end}}
          </div>
        </li>
        {{end}}
      </ul>
      {{else}}
      <div class="no-issues">No issues found for this plan</div>
      {{end}}

      <details class="plan-json">
        <summary>Raw plan JSON</summary>
        <pre>{{.PlanJSON}}</pre>
      </details>
    </div>
    {{end}}
  {{else}}
  <div class="empty">
    <div class="empty-icon">🔍</div>
    <h2>No plans found</h2>
    <p>No slow queries matched the configured thresholds in the log file.</p>
  </div>
  {{end}}

  <footer class="footer">
    Generated by <a href="https://github.com/bright98/pgwatch">pgwatch</a>
    &middot; powered by <a href="https://github.com/bright98/pgexplain">pgexplain</a>
  </footer>

</div>
</body>
</html>`
