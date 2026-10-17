package inject

import (
	"strings"
	"testing"
	"time"

	"github.com/SuperMarioYL/deadend-ledger/internal/ledger"
)

var today = time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)

func entry(approach string, scope []string, observed string) ledger.Entry {
	return ledger.Entry{
		Ordinal: 3,
		Tombstone: ledger.Tombstone{
			ID:       ledger.NewID(approach, "go test ./internal/auth -run TestTokenRefresh"),
			Approach: approach,
			Scope:    scope,
			Evidence: ledger.Evidence{
				Kind:       ledger.KindFailingTest,
				Locator:    "go test ./internal/auth -run TestTokenRefresh",
				Excerpt:    "--- FAIL: TestTokenRefresh (0.00s)",
				ObservedAt: observed,
			},
			Status:     ledger.StatusActive,
			CreatedAt:  "2026-10-10",
			ReviveWhen: "the auth backend supports idempotent refresh",
		},
	}
}

func TestBlockEmptyWithoutMatches(t *testing.T) {
	if got := Block(nil, "unrelated prompt about the download queue", today); got != "" {
		t.Fatalf("no entries must inject nothing, got %q", got)
	}
	entries := []ledger.Entry{entry("use client-side token refresh", []string{"internal/auth/token.go", "TestTokenRefresh"}, "2026-10-10")}
	if got := Block(entries, "fix the flaky retry in the download queue", today); got != "" {
		t.Fatalf("unrelated task must inject nothing, got %q", got)
	}
}

func TestBlockRendersMatchedTombstone(t *testing.T) {
	entries := []ledger.Entry{entry("use client-side token refresh", []string{"internal/auth/token.go", "TestTokenRefresh"}, "2026-10-10")}
	got := Block(entries, "continue the client-side token refresh refactor", today)
	for _, want := range []string{
		"<deadend-ledger active-tombstones>",
		"</deadend-ledger>",
		"[tombstone #3] use client-side token refresh",
		"scope: internal/auth/token.go, TestTokenRefresh",
		"evidence: failing_test | go test ./internal/auth -run TestTokenRefresh (observed 2026-10-10)",
		"do-not-retry unless: the auth backend supports idempotent refresh",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered block missing %q:\n%s", want, got)
		}
	}
}

func TestBlockSkipsExpiredAndRevived(t *testing.T) {
	e := entry("use client-side token refresh", []string{"internal/auth", "TestTokenRefresh"}, "2026-10-10")
	expired := e
	expired.Tombstone.ExpiresAt = "2026-10-16"
	revived := e
	revived.Tombstone.Status = ledger.StatusRevived
	revived.Tombstone.ReviveWhen = "backend now idempotent"
	entries := []ledger.Entry{expired, revived}
	if got := Block(entries, "continue the client-side token refresh refactor", today); got != "" {
		t.Fatalf("expired/revived must not inject:\n%s", got)
	}
}

func TestBlockOrdersByScore(t *testing.T) {
	strong := entry("use client-side token refresh", []string{"client", "side", "token", "refresh"}, "2026-10-10")
	strong.Ordinal = 1
	weak := entry("cache auth tokens on disk", []string{"internal", "auth", "token", "refresh"}, "2026-10-11")
	weak.Ordinal = 2
	got := Block([]ledger.Entry{weak, strong}, "client side token refresh", today)
	iStrong := strings.Index(got, "tombstone #1")
	iWeak := strings.Index(got, "tombstone #2")
	if iStrong < 0 || iWeak < 0 || iStrong > iWeak {
		t.Fatalf("higher score must render first:\n%s", got)
	}
}

func TestBlockNeverFloodsContext(t *testing.T) {
	var entries []ledger.Entry
	for i := 0; i < 40; i++ {
		e := entry("approach number of the failing kind", []string{"client", "side", "token", "refresh", "scope", "word"}, "2026-10-10")
		e.Ordinal = i + 1
		e.Tombstone.Evidence.Excerpt = strings.Repeat("x", 380)
		e.Tombstone.ID = ledger.NewID(e.Tombstone.Approach, e.Tombstone.Evidence.Locator+string(rune('a'+i%26))+string(rune('a'+i/26)))
		entries = append(entries, e)
	}
	got := Block(entries, "client side token refresh scope word", today)
	if got == "" {
		t.Fatal("expected some matches to render")
	}
	if n := len([]rune(got)); n > MaxBlockRunes {
		t.Fatalf("block is %d runes, cap is %d", n, MaxBlockRunes)
	}
	if !strings.Contains(got, "omitted") {
		t.Fatalf("dropped matches must be counted:\n%s", got)
	}
	if strings.Count(got, "</deadend-ledger>") != 1 {
		t.Fatal("block must close exactly once")
	}
}

func TestRenderCollapsesMultilineExcerpt(t *testing.T) {
	e := entry("use client-side token refresh", []string{"token", "refresh"}, "2026-10-10")
	e.Tombstone.Evidence.Excerpt = "--- FAIL: TestTokenRefresh\n    token_test.go:11:\trefresh loop re-entered\nFAIL"
	got := Render([]Match{{Tombstone: e.Tombstone, Ordinal: e.Ordinal, Score: 1}})
	if strings.Contains(got, "excerpt: --- FAIL: TestTokenRefresh\n    token_test.go") {
		t.Fatalf("excerpt must render as one line:\n%s", got)
	}
	want := "excerpt: --- FAIL: TestTokenRefresh token_test.go:11: refresh loop re-entered FAIL"
	if !strings.Contains(got, want) {
		t.Fatalf("collapsed excerpt missing:\n%s", got)
	}
}

func TestRenderTombstoneOmitsEmptyExcerpt(t *testing.T) {
	e := entry("use client-side token refresh", []string{"token", "refresh"}, "2026-10-10")
	e.Tombstone.Evidence.Excerpt = ""
	e.Tombstone.ReviveWhen = ""
	got := Render([]Match{{Tombstone: e.Tombstone, Ordinal: e.Ordinal, Score: 1}})
	if strings.Contains(got, "excerpt:") {
		t.Fatalf("empty excerpt must not render:\n%s", got)
	}
	if strings.Contains(got, "do-not-retry unless:") {
		t.Fatalf("empty revive condition must not render:\n%s", got)
	}
}
