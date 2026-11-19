package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SuperMarioYL/deadend-ledger/internal/ledger"
)

// runCLI drives the real cobra tree in-process and captures what a user sees.
func runCLI(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	rootCmd.SetArgs(args)
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	err = rootCmd.Execute()
	rootCmd.SetArgs(nil)
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
	return out.String(), errOut.String(), err
}

func TestCLILifecycle(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if _, _, err := runCLI(t, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	out, _, err := runCLI(t, "record",
		"--approach", "use client-side token refresh",
		"--scope", "internal/auth/token.go",
		"--locator", "go test ./internal/auth",
		"--revive-when", "the auth backend accepts idempotent refresh",
		"--yes",
	)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if !strings.Contains(out, "tombstone #1 recorded") {
		t.Fatalf("record output: %q", out)
	}

	out, _, err = runCLI(t, "inject", "--context", "client side token refresh in internal auth")
	if err != nil {
		t.Fatalf("inject: %v", err)
	}
	if !strings.Contains(out, "[tombstone #1]") || !strings.Contains(out, "</deadend-ledger>") {
		t.Fatalf("matching task must inject tombstone #1, got %q", out)
	}

	out, _, err = runCLI(t, "review")
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	for _, want := range []string{
		"1 tombstone(s): 1 injecting, 0 past due, 0 expired, 0 revived",
		"#1 [",
		"evidence: failing_test | go test ./internal/auth",
		"revive when: the auth backend accepts idempotent refresh",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("review missing %q:\n%s", want, out)
		}
	}

	id := ledger.NewID("use client-side token refresh", "go test ./internal/auth")
	out, _, err = runCLI(t, "revive", id)
	if err != nil {
		t.Fatalf("revive: %v", err)
	}
	if !strings.Contains(out, "tombstone #1 ("+id+") revived") {
		t.Fatalf("revive output: %q", out)
	}

	out, _, err = runCLI(t, "inject", "--context", "client side token refresh in internal auth")
	if err != nil {
		t.Fatalf("inject after revive: %v", err)
	}
	if out != "" {
		t.Fatalf("a revived tombstone must not inject, got %q", out)
	}

	out, _, err = runCLI(t, "review")
	if err != nil {
		t.Fatalf("review after revive: %v", err)
	}
	if !strings.Contains(out, "1 tombstone(s): 0 injecting, 0 past due, 0 expired, 1 revived") {
		t.Fatalf("review after revive:\n%s", out)
	}

	// The failure recurred: recording it again re-arms the same tombstone.
	out, _, err = runCLI(t, "record",
		"--approach", "use client-side token refresh",
		"--scope", "internal/auth/token.go",
		"--locator", "go test ./internal/auth",
		"--yes",
	)
	if err != nil {
		t.Fatalf("re-record: %v", err)
	}
	if !strings.Contains(out, "tombstone #1 re-armed") {
		t.Fatalf("re-record output: %q", out)
	}
	out, _, err = runCLI(t, "inject", "--context", "client side token refresh in internal auth")
	if err != nil {
		t.Fatalf("inject after re-arm: %v", err)
	}
	if !strings.Contains(out, "[tombstone #1]") {
		t.Fatalf("re-armed tombstone must inject again, got %q", out)
	}
}

func TestInjectSurvivesCorruptLine(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if _, _, err := runCLI(t, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, _, err := runCLI(t, "record",
		"--approach", "use client-side token refresh",
		"--scope", "internal/auth",
		"--locator", "go test ./internal/auth",
		"--yes",
	); err != nil {
		t.Fatalf("record: %v", err)
	}

	f, err := os.OpenFile(filepath.Join(dir, ".deadend", "ledger.jsonl"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("{\"id\":\"broken\"\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	out, errOut, err := runCLI(t, "inject", "--context", "client side token refresh in internal auth")
	if err != nil {
		t.Fatalf("a corrupt ledger line must not fail the hook: %v", err)
	}
	if !strings.Contains(out, "[tombstone #1]") {
		t.Fatalf("the readable tombstone must still inject, got %q", out)
	}
	if !strings.Contains(errOut, "line 2") {
		t.Fatalf("the skipped record must be reported on stderr, got %q", errOut)
	}
}

func TestReviewRendersStatusAndAge(t *testing.T) {
	today := time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)
	entries := []ledger.Entry{
		{
			Ordinal: 1,
			Tombstone: ledger.Tombstone{
				ID:        "aaaaaaaa",
				Approach:  "use client-side token refresh",
				Scope:     []string{"internal/auth"},
				Evidence:  ledger.Evidence{Kind: ledger.KindFailingTest, Locator: "go test ./internal/auth", ObservedAt: "2026-11-10"},
				Status:    ledger.StatusActive,
				CreatedAt: "2026-11-10",
				ExpiresAt: "2026-12-01",
			},
		},
		{
			Ordinal: 2,
			Tombstone: ledger.Tombstone{
				ID:         "bbbbbbbb",
				Approach:   "bump the JWT library to v9",
				Scope:      []string{"go.mod"},
				Evidence:   ledger.Evidence{Kind: ledger.KindBuildLog, Locator: "go build ./...", ObservedAt: "2026-11-01"},
				Status:     ledger.StatusRevived,
				CreatedAt:  "2026-11-01",
				ReviveWhen: "the module graph accepts go 1.25",
			},
		},
	}
	var b strings.Builder
	renderReview(&b, entries, today)
	got := b.String()
	for _, want := range []string{
		"2 tombstone(s): 1 injecting, 0 past due, 0 expired, 1 revived",
		"#1 [aaaaaaaa] active · 10 days old · injecting",
		"#2 [bbbbbbbb] revived · 19 days old · not injecting — revived",
		"expires: 2026-12-01",
		"revive when: the module graph accepts go 1.25",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("review missing %q:\n%s", want, got)
		}
	}
}
