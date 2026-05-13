package reporter

import (
	"github.com/bright98/pgexplain/advisor"
	"github.com/bright98/pgwatch/internal/source"
)

// QueryReport holds one plan's metadata and all advisor findings for it.
type QueryReport struct {
	Rank     int
	Plan     source.RawPlan
	Findings []advisor.Finding
}

// Reporter is the output destination interface. Implementations write to terminal,
// JSON file, or any other sink without the watcher knowing the difference.
type Reporter interface {
	Report(reports []QueryReport) error
}
