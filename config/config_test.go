package config

import (
	"os"
	"testing"
	"time"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "pgwatch-cfg-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	f.WriteString(content)
	f.Close()
	return f.Name()
}

func TestLoad_Defaults(t *testing.T) {
	path := writeTemp(t, "log_file: /var/log/postgresql/postgresql.log\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Source != "auto_explain" {
		t.Errorf("expected source=auto_explain, got %q", cfg.Source)
	}
	if cfg.Interval != time.Hour {
		t.Errorf("expected interval=1h, got %v", cfg.Interval)
	}
	if cfg.Output != "terminal" {
		t.Errorf("expected output=terminal, got %q", cfg.Output)
	}
}

func TestLoad_MissingLogFile(t *testing.T) {
	path := writeTemp(t, "source: auto_explain\n")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing log_file")
	}
}

func TestLoad_InvalidOutput(t *testing.T) {
	path := writeTemp(t, "log_file: /tmp/pg.log\noutput: slack\n")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid output value")
	}
}

func TestLoad_JSONWithoutOutFile(t *testing.T) {
	path := writeTemp(t, "log_file: /tmp/pg.log\noutput: json\n")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error: json output requires out_file")
	}
}

func TestLoad_FullConfig(t *testing.T) {
	yaml := `
log_file: /var/log/postgresql/postgresql.log
interval: 30m
output: json
out_file: /tmp/report.json
`
	path := writeTemp(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Interval != 30*time.Minute {
		t.Errorf("expected interval=30m, got %v", cfg.Interval)
	}
	if cfg.OutFile != "/tmp/report.json" {
		t.Errorf("expected out_file set, got %q", cfg.OutFile)
	}
}
