// Package autoexplain implements PlanSource by tailing a PostgreSQL log file
// that has auto_explain enabled. It parses the EXPLAIN (ANALYZE, FORMAT JSON)
// blocks that auto_explain writes and emits one RawPlan per block.
//
// Required PostgreSQL settings:
//
//	shared_preload_libraries = 'auto_explain'
//	auto_explain.log_min_duration = 1000   # controls which queries Postgres logs; pgwatch reads all of them
//	auto_explain.log_format = json
//	auto_explain.log_analyze = on
//	log_line_prefix = '%m [%p] %q%u@%d '  # must include timestamp, pid, user, database
package autoexplain

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bright98/pgwatch/internal/source"
)

// Name is the canonical source identifier emitted in every RawPlan.SourceName
// and matched against config.SourceAutoExplain in wire.go.
const Name = "auto_explain"

// tailPollInterval is how long the streaming goroutine sleeps before re-reading
// the file when it reaches EOF. 250 ms is imperceptible for a slow-query advisor
// and avoids the overhead of an fsnotify dependency.
const tailPollInterval = 250 * time.Millisecond

// logRe detects an auto_explain trigger line and captures its named groups:
//
//	m[1] = timestamp  (e.g. "2026-05-10 14:23:01.887 UTC")
//	m[2] = user       (e.g. "myuser")
//	m[3] = database   (e.g. "mydb")
//	m[4] = duration   (e.g. "1243.821")
//
// Matches log_line_prefix patterns that include %m, [%p], and %u@%d.
// Example trigger line:
//
//	2026-05-10 14:23:01.887 UTC [8821] myuser@mydb LOG:  duration: 1243.821 ms  plan:
var logRe = regexp.MustCompile(
	`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d+\s+\S+)\s+\[\d+\]\s+(\S+)@(\S+)\s+LOG:\s+duration:\s+([\d.]+)\s+ms\s+plan:\s*$`,
)

// tsLayouts lists the timestamp formats we try when parsing the log line prefix.
// PostgreSQL uses the server timezone abbreviation (e.g. UTC, EST) which Go's
// time.Parse accepts with the MST placeholder.
var tsLayouts = []string{
	"2006-01-02 15:04:05.000 MST",
	"2006-01-02 15:04:05.999 MST",
	"2006-01-02 15:04:05 MST",
}

// Config configures the auto_explain log source.
type Config struct {
	// LogFile is the path to the PostgreSQL log file containing auto_explain output.
	LogFile string

	// Tail controls reading behaviour after EOF:
	//   true  — keep polling for new lines (daemon mode, pgwatch run)
	//   false — stop at EOF and close the channel (one-shot mode, pgwatch report)
	Tail bool
}

// Source reads EXPLAIN ANALYZE JSON blocks from a PostgreSQL log file written by auto_explain.
type Source struct {
	cfg Config
}

// New returns a Source configured with cfg. Call Plans() to start streaming.
func New(cfg Config) *Source {
	return &Source{cfg: cfg}
}

// Name returns the canonical source identifier.
func (s *Source) Name() string { return Name }

// Close is a no-op for the file-based source; the file is closed by the stream goroutine.
func (s *Source) Close() error { return nil }

// Plans opens the log file and starts a background goroutine that parses and streams
// RawPlans onto the returned channel. The channel is closed when the goroutine exits.
//
// In Tail mode the goroutine seeks to EOF before reading, so only lines written
// after Plans() is called are processed. In one-shot mode it reads from the start.
func (s *Source) Plans(ctx context.Context) (<-chan source.RawPlan, error) {
	f, err := os.Open(s.cfg.LogFile)
	if err != nil {
		return nil, fmt.Errorf("autoexplain: open %q: %w", s.cfg.LogFile, err)
	}
	if s.cfg.Tail {
		if _, err := f.Seek(0, io.SeekEnd); err != nil {
			f.Close()
			return nil, fmt.Errorf("autoexplain: seek to end: %w", err)
		}
	}
	ch := make(chan source.RawPlan, 64)
	go s.stream(ctx, f, ch)
	return ch, nil
}

// stream is the background goroutine that reads lines from f and sends parsed plans to ch.
func (s *Source) stream(ctx context.Context, f *os.File, ch chan<- source.RawPlan) {
	defer f.Close()
	defer close(ch)

	r := bufio.NewReaderSize(f, 1<<20) // 1 MB read buffer
	var pending string                  // accumulates a partial line across reads

	for {
		if ctx.Err() != nil {
			return
		}

		chunk, err := r.ReadString('\n')
		pending += chunk

		if err == io.EOF {
			if !s.cfg.Tail {
				return
			}
			// Wait for the log writer to append more data before retrying.
			select {
			case <-ctx.Done():
				return
			case <-time.After(tailPollInterval):
			}
			continue
		}
		if err != nil {
			return
		}

		line := strings.TrimRight(pending, "\r\n")
		pending = ""

		m := logRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		durMs, err := strconv.ParseFloat(m[4], 64)
		if err != nil {
			continue
		}

		planJSON, err := s.collectJSON(ctx, r)
		if err != nil || len(planJSON) == 0 {
			continue
		}

		select {
		case ch <- source.RawPlan{
			PlanJSON:   planJSON,
			DurationMs: durMs,
			Timestamp:  parseTimestamp(m[1]),
			User:       m[2],
			Database:   m[3],
			SourceName: Name,
		}:
		case <-ctx.Done():
			return
		}
	}
}

// collectJSON reads lines from r until the JSON bracket depth returns to zero,
// indicating the end of the EXPLAIN FORMAT JSON array written by auto_explain.
// It tracks [ ] { } characters to find the boundary without a full JSON parse.
func (s *Source) collectJSON(ctx context.Context, r *bufio.Reader) ([]byte, error) {
	var sb strings.Builder
	var pending string
	depth := 0
	started := false

	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		chunk, err := r.ReadString('\n')
		pending += chunk

		if err == io.EOF {
			if !s.cfg.Tail {
				if len(pending) > 0 {
					sb.WriteString(pending)
				}
				break
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(tailPollInterval):
			}
			continue
		}
		if err != nil {
			return nil, err
		}

		for _, c := range pending {
			switch c {
			case '[', '{':
				depth++
				started = true
			case ']', '}':
				depth--
			}
		}
		sb.WriteString(pending)
		pending = ""

		if started && depth == 0 {
			break
		}
	}

	return []byte(strings.TrimSpace(sb.String())), nil
}

// parseTimestamp tries each layout in tsLayouts and returns the first successful
// parse. Falls back to time.Now() if the string doesn't match any known format.
func parseTimestamp(s string) time.Time {
	for _, layout := range tsLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Now()
}
