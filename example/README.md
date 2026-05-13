# pgwatch example

A ready-to-run configuration for monitoring PostgreSQL slow queries with pgwatch using `auto_explain`.

---

## 1. Configure PostgreSQL

Add the following to `postgresql.conf` (find it with `SHOW config_file;` inside psql):

```
shared_preload_libraries = 'auto_explain'
auto_explain.log_min_duration = 500
auto_explain.log_format = json
auto_explain.log_analyze = on
log_line_prefix = '%m [%p] %q%u@%d '
```

`shared_preload_libraries` requires a **full server restart**. Apply the remaining settings without a restart:

```sql
SELECT pg_reload_conf();
```

---

## 2. Install pgwatch

```bash
go install github.com/bright98/pgwatch/cmd/pgwatch@latest
```

---

## 3. Find your PostgreSQL log file

Run these inside psql:

```sql
SHOW log_directory;
SHOW data_directory;
```

Common locations:

| Platform | Path |
|---|---|
| macOS (Homebrew) | `/opt/homebrew/var/log/postgresql@<version>.log` |
| Linux (apt) | `/var/log/postgresql/postgresql-<version>-main.log` |
| Linux (systemd) | `journalctl -u postgresql` (no file; use `log_destination = csvlog`) |

Update the `log_file` value in `pgwatch.yaml` and `pgwatch.html.yaml` to match your system.

---

## 4. Run pgwatch in daemon mode (terminal output)

```bash
pgwatch run -c example/pgwatch.yaml
```

pgwatch tails the PostgreSQL log, parses `auto_explain` JSON plan blocks, and prints rule violations to the terminal every 30 seconds.

Example output:

```
=== pgwatch report — Tue, 13 May 2026 14:41:40 UTC ===

#1  2026-05-13 14:41:40 UTC  haleh@pagila  duration=10.48ms  [auto_explain]
    [WARN] node=1 (Sort) — row estimate for Sort was off by 3163x (overestimate: planned 15815, got 5)
           detail:     The planner estimated 15815 rows but this node produced 5 (loops=1). A 3163x
                       overestimate can cause the planner to choose the wrong join strategy.
           suggestion: Run ANALYZE on the tables involved in this node to refresh planner statistics.
```

---

## 5. Generate a one-shot HTML report

```bash
pgwatch report -c example/pgwatch.html.yaml
open pgwatch-report.html
```

Reads all plans from the log file, analyzes them, and writes a self-contained HTML report to `pgwatch-report.html`.

---

## Files

| File | Description |
|---|---|
| `pgwatch.yaml` | Daemon config — tails the log and prints to terminal |
| `pgwatch.html.yaml` | Report config — reads the full log and writes an HTML report |

---

## Generating a test workload

To produce slow queries that trigger pgwatch rules, load the [pagila](https://github.com/devrimgunduz/pagila) sample database and run [sql-load-test](https://github.com/bright98/sql-load-test):

```bash
go run main.go -pagila "host=localhost dbname=pagila user=postgres sslmode=disable"
```
