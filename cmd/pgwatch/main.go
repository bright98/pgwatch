package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cfgPath string

func main() {
	root := &cobra.Command{
		Use:   "pgwatch",
		Short: "PostgreSQL slow query watcher and advisor",
		Long: `pgwatch connects to a PostgreSQL database, finds the slowest queries via
pg_stat_statements, explains each one, and reports actionable findings using
the pgexplain rule engine.`,
	}

	root.PersistentFlags().StringVarP(&cfgPath, "config", "c", "pgwatch.yaml", "path to config file")
	root.AddCommand(newRunCmd())
	root.AddCommand(newReportCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
