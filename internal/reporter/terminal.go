package reporter

import (
	"fmt"
	"io"
	"os"
	"time"
)

type Terminal struct {
	w io.Writer
}

func NewTerminal() *Terminal {
	return &Terminal{w: os.Stdout}
}

func (t *Terminal) Report(reports []QueryReport) error {
	if len(reports) == 0 {
		fmt.Fprintln(t.w, "pgwatch: no plans found matching the configured thresholds.")
		return nil
	}

	fmt.Fprintf(t.w, "\n=== pgwatch report — %s ===\n\n", time.Now().Format(time.RFC1123))

	for _, r := range reports {
		p := r.Plan
		fmt.Fprintf(t.w, "#%d  %s  %s@%s  duration=%.2fms  [%s]\n",
			r.Rank,
			p.Timestamp.UTC().Format("2006-01-02 15:04:05 UTC"),
			p.User,
			p.Database,
			p.DurationMs,
			p.SourceName,
		)

		if len(r.Findings) == 0 {
			fmt.Fprintln(t.w, "    (no findings)")
		}
		for _, f := range r.Findings {
			fmt.Fprintf(t.w, "    [%s] node=%d (%s) — %s\n", f.Severity, f.NodeID, f.NodeType, f.Message)
			if f.Detail != "" {
				fmt.Fprintf(t.w, "           %s\n", f.Detail)
			}
			if f.Suggestion != "" {
				fmt.Fprintf(t.w, "           → %s\n", f.Suggestion)
			}
		}
		fmt.Fprintln(t.w)
	}
	return nil
}
