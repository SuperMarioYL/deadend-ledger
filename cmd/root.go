package cmd

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "deadend",
	Short: "Tombstones for failed coding-agent approaches",
	Long: `deadend-ledger records failed approaches as tombstones with machine-checkable
evidence, and injects them into planning context when a similar task starts,
so the coding agent skips the dead end instead of re-walking it.

One binary, one append-only JSONL file (.deadend/ledger.jsonl), zero daemons.
The ledger holds negative results only — no facts, no preferences, no prose memory.`,
	SilenceUsage: true,
}

// Execute runs the CLI; version comes from main (VERSION file / release ldflags).
func Execute(version string) {
	rootCmd.Version = version
	rootCmd.SetVersionTemplate("deadend {{.Version}}\n")
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// isTerminal reports whether f is attached to an interactive terminal.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// silentExec runs a command in dir and returns its stdout; stderr is dropped
// so suggestions (e.g. changed files) never pollute command output.
func silentExec(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}
