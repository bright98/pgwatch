// Package config loads and validates pgwatch's YAML configuration file.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Source type constants. The Source field in Config must be one of these.
const (
	SourceAutoExplain = "auto_explain" // tail the PostgreSQL log file for auto_explain output
)

// Output destination constants. The Output field in Config must be one of these.
const (
	OutputTerminal = "terminal" // print findings to stdout
	OutputJSON     = "json"     // write findings to a JSON file (requires OutFile)
	OutputHTML     = "html"     // write a self-contained HTML report (requires OutFile)
)

// Config holds all pgwatch runtime settings loaded from the YAML config file.
type Config struct {
	// Source selects which plan source to use.
	// Supported values: "auto_explain".
	// Default: "auto_explain".
	Source string `yaml:"source"`

	// LogFile is the path to the PostgreSQL log file that contains auto_explain output.
	// Required when source is "auto_explain".
	// Example: /var/log/postgresql/postgresql-14-main.log
	LogFile string `yaml:"log_file"`

	// Interval controls how often accumulated findings are flushed to the reporter.
	// In daemon mode (pgwatch run), a report is printed every Interval.
	// Default: 1h.
	Interval time.Duration `yaml:"interval"`

	// MaxBufferedPlans is the maximum number of plans held in memory between
	// report flushes. When the limit is reached, a flush is triggered immediately
	// instead of waiting for the next Interval tick — preventing unbounded memory
	// growth during high-traffic periods.
	// Default: 1000.
	MaxBufferedPlans int `yaml:"max_buffered_plans"`

	// Output selects the report destination.
	// Supported values: "terminal", "json", "html".
	// Default: "terminal".
	Output string `yaml:"output"`

	// OutFile is the file path for JSON output.
	// Required when Output is "json".
	// Example: /tmp/pgwatch-report.json
	OutFile string `yaml:"out_file"`

}

// Load reads the YAML file at path, applies defaults, and validates the result.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("config: open %q: %w", path, err)
	}
	defer f.Close()

	cfg := &Config{
		Source:           SourceAutoExplain,
		Interval:         time.Hour,
		Output:           OutputTerminal,
		MaxBufferedPlans: 1000,
	}

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true) // fail fast on unknown YAML keys (catches typos)
	if err := dec.Decode(cfg); err != nil {
		return nil, fmt.Errorf("config: decode %q: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	return cfg, nil
}

func (c *Config) validate() error {
	switch c.Source {
	case SourceAutoExplain:
		if c.LogFile == "" {
			return fmt.Errorf("log_file is required when source is %q", SourceAutoExplain)
		}
	default:
		return fmt.Errorf("source must be one of: %s", SourceAutoExplain)
	}
	if c.Interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}
	if c.MaxBufferedPlans <= 0 {
		return fmt.Errorf("max_buffered_plans must be positive")
	}
	switch c.Output {
	case OutputTerminal, OutputJSON, OutputHTML:
	default:
		return fmt.Errorf("output must be one of: %s, %s, %s", OutputTerminal, OutputJSON, OutputHTML)
	}
	if c.OutFile == "" && (c.Output == OutputJSON || c.Output == OutputHTML) {
		return fmt.Errorf("out_file is required when output is %q or %q", OutputJSON, OutputHTML)
	}
	return nil
}
