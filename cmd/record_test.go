package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The interactive default scope is the tombstone's matching key, so it must be
// a real path that a task prompt can reproduce: not git's C-quoted escape for a
// non-ASCII name, and not the tool's own bookkeeping directories.
func TestSuggestScopeUnquotedPathsAndStateDirs(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")

	for _, f := range []string{
		".deadend/ledger.jsonl",
		".claude/settings.json",
		"internal/auth/token.go",
		"刷新逻辑.go",
	} {
		p := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got := suggestScope(dir)
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "刷新逻辑.go") {
		t.Fatalf("non-ASCII path must be suggested verbatim, got %q", got)
	}
	for _, p := range got {
		if strings.HasPrefix(p, ".deadend") || strings.HasPrefix(p, ".claude") {
			t.Fatalf("state path %q must not be offered as a scope key, got %q", p, got)
		}
	}
}
