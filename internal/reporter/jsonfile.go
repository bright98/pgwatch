package reporter

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/bright98/pgexplain/advisor"
)

type jsonFinding struct {
	Severity   string `json:"severity"`
	NodeID     int    `json:"node_id"`
	NodeType   string `json:"node_type,omitempty"`
	Message    string `json:"message"`
	Detail     string `json:"detail,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

type jsonPlan struct {
	Rank       int           `json:"rank"`
	Timestamp  time.Time     `json:"timestamp"`
	User       string        `json:"user"`
	Database   string        `json:"database"`
	DurationMs float64       `json:"duration_ms"`
	Source     string        `json:"source"`
	Findings   []jsonFinding `json:"findings"`
}

type jsonOutput struct {
	GeneratedAt time.Time  `json:"generated_at"`
	Reports     []jsonPlan `json:"reports"`
}

type JSONFile struct {
	path string
}

func NewJSONFile(path string) *JSONFile {
	return &JSONFile{path: path}
}

func (j *JSONFile) Report(reports []QueryReport) error {
	out := jsonOutput{
		GeneratedAt: time.Now().UTC(),
		Reports:     make([]jsonPlan, 0, len(reports)),
	}
	for _, r := range reports {
		out.Reports = append(out.Reports, toJSONPlan(r))
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("reporter: marshal json: %w", err)
	}
	if err := os.WriteFile(j.path, data, 0o644); err != nil {
		return fmt.Errorf("reporter: write %q: %w", j.path, err)
	}
	return nil
}

func toJSONPlan(r QueryReport) jsonPlan {
	jp := jsonPlan{
		Rank:       r.Rank,
		Timestamp:  r.Plan.Timestamp.UTC(),
		User:       r.Plan.User,
		Database:   r.Plan.Database,
		DurationMs: r.Plan.DurationMs,
		Source:     r.Plan.SourceName,
		Findings:   make([]jsonFinding, 0, len(r.Findings)),
	}
	for _, f := range r.Findings {
		jp.Findings = append(jp.Findings, toJSONFinding(f))
	}
	return jp
}

func toJSONFinding(f advisor.Finding) jsonFinding {
	return jsonFinding{
		Severity:   f.Severity.String(),
		NodeID:     f.NodeID,
		NodeType:   f.NodeType,
		Message:    f.Message,
		Detail:     f.Detail,
		Suggestion: f.Suggestion,
	}
}

