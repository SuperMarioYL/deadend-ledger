package ledger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func validTombstone(approach, locator string) Tombstone {
	return Tombstone{
		Approach: approach,
		Scope:    []string{"internal/auth"},
		Evidence: Evidence{
			Kind:       KindFailingTest,
			Locator:    locator,
			Excerpt:    "--- FAIL: TestTokenRefresh",
			ObservedAt: "2026-10-10",
		},
		Status:    StatusActive,
		CreatedAt: "2026-10-10",
	}
}

func TestAppendLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	if created, err := s.Init(); err != nil || !created {
		t.Fatalf("Init: created=%v err=%v", created, err)
	}
	if created, err := s.Init(); err != nil || created {
		t.Fatalf("second Init should be a no-op: created=%v err=%v", created, err)
	}
	for _, a := range []string{"use client-side token refresh", "cache the whole response", "hand-roll the retry loop"} {
		ts := validTombstone(a, "go test ./internal/auth")
		if err := s.Append(&ts); err != nil {
			t.Fatalf("Append %q: %v", a, err)
		}
		if len(ts.ID) != 8 {
			t.Fatalf("Append should assign the id, got %q", ts.ID)
		}
	}
	entries, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("loaded %d entries, want 3", len(entries))
	}
	for i, e := range entries {
		if e.Ordinal != i+1 {
			t.Fatalf("entry %d has ordinal %d", i, e.Ordinal)
		}
	}
	if entries[0].Tombstone.ID == entries[1].Tombstone.ID || entries[1].Tombstone.ID == entries[2].Tombstone.ID {
		t.Fatal("distinct approach+evidence pairs must yield distinct ids")
	}
	if entries[2].Tombstone.Approach != "hand-roll the retry loop" {
		t.Fatalf("append order broken: %q", entries[2].Tombstone.Approach)
	}
}

func TestAppendRejectsInvalidSchema(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	_, _ = s.Init()

	cases := map[string]func(*Tombstone){
		"empty scope":      func(t *Tombstone) { t.Scope = nil },
		"bad kind":         func(t *Tombstone) { t.Evidence.Kind = "vibes" },
		"empty locator":    func(t *Tombstone) { t.Evidence.Locator = " " },
		"long excerpt":     func(t *Tombstone) { t.Evidence.Excerpt = strings.Repeat("x", MaxExcerptRunes+1) },
		"bad observed_at":  func(t *Tombstone) { t.Evidence.ObservedAt = "10/10/2026" },
		"bad status":       func(t *Tombstone) { t.Status = "zombie" },
		"bad created_at":   func(t *Tombstone) { t.CreatedAt = "" },
		"bad expires date": func(t *Tombstone) { t.ExpiresAt = "soon" },
	}
	for name, mutate := range cases {
		ts := validTombstone("approach "+name, "go test ./...")
		mutate(&ts)
		if err := s.Append(&ts); err == nil {
			t.Fatalf("%s: expected validation error", name)
		}
	}
	entries, _ := s.Load()
	if len(entries) != 0 {
		t.Fatalf("invalid tombstones must not reach the ledger, got %d", len(entries))
	}
}

func TestAppendRejectsDuplicate(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	_, _ = s.Init()
	first := validTombstone("use client-side token refresh", "go test ./internal/auth")
	if err := s.Append(&first); err != nil {
		t.Fatal(err)
	}
	dup := validTombstone("use client-side token refresh", "go test ./internal/auth")
	if err := s.Append(&dup); err == nil {
		t.Fatal("expected duplicate rejection")
	} else if !strings.Contains(err.Error(), "already records") {
		t.Fatalf("duplicate error should explain itself: %v", err)
	}
}

func TestLoadReportsCorruptLine(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	_, _ = s.Init()
	good := validTombstone("a good record", "go test ./...")
	if err := s.Append(&good); err != nil {
		t.Fatal(err)
	}
	f, _ := os.OpenFile(s.Path(), os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString("{\"id\":\"broken\"\n")
	f.Close()
	_, err := s.Load()
	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("corrupt line must be reported with its number, got: %v", err)
	}
}

func TestLoadMissingLedgerIsEmpty(t *testing.T) {
	entries, err := Open(filepath.Join(t.TempDir(), "nowhere")).Load()
	if err != nil || entries != nil {
		t.Fatalf("missing ledger = nil entries, nil error; got %v, %v", entries, err)
	}
}

func TestLiveOnExpiry(t *testing.T) {
	today := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	base := validTombstone("a", "b")

	if !base.LiveOn(today) {
		t.Fatal("active tombstone without expiry is live")
	}
	expired := base
	expired.ExpiresAt = "2026-10-16" // the day before: past due
	if expired.LiveOn(today) {
		t.Fatal("expired tombstone must stop injecting")
	}
	boundary := base
	boundary.ExpiresAt = "2026-10-17" // expires today: last live day
	if !boundary.LiveOn(today) {
		t.Fatal("tombstone live through its expiry date")
	}
	revived := base
	revived.Status = StatusRevived
	revived.ReviveWhen = "backend supports idempotent refresh"
	if revived.LiveOn(today) {
		t.Fatal("revived tombstones do not inject")
	}
	badExpiry := base
	badExpiry.ExpiresAt = "not-a-date"
	if badExpiry.LiveOn(today) {
		t.Fatal("unparseable expiry must fail closed")
	}
}

func TestCapExcerpt(t *testing.T) {
	if got := CapExcerpt("short"); got != "short" {
		t.Fatalf("CapExcerpt altered short text: %q", got)
	}
	long := strings.Repeat("汉", MaxExcerptRunes+50)
	got := CapExcerpt(long)
	if n := len([]rune(got)); n != MaxExcerptRunes {
		t.Fatalf("capped excerpt has %d runes, want %d", n, MaxExcerptRunes)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatal("truncation must be marked")
	}
}

func TestValidateRevivedRequiresCondition(t *testing.T) {
	ts := validTombstone("a", "b")
	ts.ID = NewID(ts.Approach, ts.Evidence.Locator)
	ts.Status = StatusRevived
	if err := ts.Validate(); err == nil {
		t.Fatal("revived without revive_when must be invalid")
	}
	ts.ReviveWhen = "the backend now accepts idempotent refresh"
	if err := ts.Validate(); err != nil {
		t.Fatalf("revived with condition should validate: %v", err)
	}
}

func TestAppendAfterUnterminatedLastLine(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	if _, err := s.Init(); err != nil {
		t.Fatal(err)
	}
	first := validTombstone("first approach", "one")
	if err := s.Append(&first); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		t.Fatal("test setup: a complete append ends with a newline")
	}
	// Simulate a hand-edited or merge-repaired ledger whose last record lost
	// its trailing newline.
	if err := os.WriteFile(s.Path(), raw[:len(raw)-1], 0o644); err != nil {
		t.Fatal(err)
	}

	second := validTombstone("second approach", "two")
	if err := s.Append(&second); err != nil {
		t.Fatalf("append onto an unterminated tail: %v", err)
	}
	entries, err := s.Load()
	if err != nil {
		t.Fatalf("ledger must load after the append: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("loaded %d entries, want 2 (the append must not merge records)", len(entries))
	}
}

func TestLoadBestEffortSkipsCorruptLine(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	if _, err := s.Init(); err != nil {
		t.Fatal(err)
	}
	good := validTombstone("a good record", "go test ./...")
	if err := s.Append(&good); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(s.Path(), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("{\"id\":\"broken\"\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	entries, skipped := s.LoadBestEffort()
	if len(entries) != 1 {
		t.Fatalf("best-effort load returned %d entries, want the 1 readable record", len(entries))
	}
	if len(skipped) != 1 || !strings.Contains(skipped[0].Error(), "line 2") {
		t.Fatalf("skipped = %v, want one error naming line 2", skipped)
	}
	if _, err := s.Load(); err == nil {
		t.Fatal("strict Load must still reject the corrupt line")
	}
}

func TestReviveAppendsTransition(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	if _, err := s.Init(); err != nil {
		t.Fatal(err)
	}
	ts := validTombstone("use client-side token refresh", "go test ./internal/auth")
	ts.ReviveWhen = "the auth backend accepts idempotent refresh"
	if err := s.Append(&ts); err != nil {
		t.Fatal(err)
	}

	entry, err := s.Revive(ts.ID, "")
	if err != nil {
		t.Fatalf("Revive: %v", err)
	}
	if entry.Tombstone.Status != StatusRevived {
		t.Fatalf("revived entry status = %q", entry.Tombstone.Status)
	}
	if entry.Ordinal != 1 {
		t.Fatalf("revived entry ordinal = %d, want the first-seen 1", entry.Ordinal)
	}

	entries, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("Load resolved %d entries, want 1 tombstone", len(entries))
	}
	if entries[0].Tombstone.Status != StatusRevived {
		t.Fatalf("resolved status = %q, want revived (last record wins)", entries[0].Tombstone.Status)
	}
	if entries[0].Tombstone.LiveOn(time.Now()) {
		t.Fatal("a revived tombstone must not inject")
	}

	raw, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("ledger has %d lines, want the original plus one appended transition", len(lines))
	}
	if !strings.Contains(lines[0], `"status":"active"`) {
		t.Fatal("the original record must stay untouched")
	}
	if !strings.Contains(lines[1], `"status":"revived"`) {
		t.Fatal("the transition record must carry the revived status")
	}
}

func TestAppendRearmsRevived(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	if _, err := s.Init(); err != nil {
		t.Fatal(err)
	}
	ts := validTombstone("use client-side token refresh", "go test ./internal/auth")
	ts.ReviveWhen = "the auth backend accepts idempotent refresh"
	if err := s.Append(&ts); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Revive(ts.ID, ""); err != nil {
		t.Fatal(err)
	}

	// The same failure recurred after the revive: recording it must re-arm the
	// same tombstone instead of being rejected as a duplicate.
	again := validTombstone("use client-side token refresh", "go test ./internal/auth")
	if err := s.Append(&again); err != nil {
		t.Fatalf("re-record after revive: %v", err)
	}
	entries, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Ordinal != 1 {
		t.Fatalf("want one tombstone at ordinal 1, got %+v", entries)
	}
	if entries[0].Tombstone.Status != StatusActive {
		t.Fatalf("re-armed status = %q, want active", entries[0].Tombstone.Status)
	}

	// While active, an exact duplicate is still refused.
	dup := validTombstone("use client-side token refresh", "go test ./internal/auth")
	if err := s.Append(&dup); err == nil {
		t.Fatal("an active tombstone must still reject the same approach and locator")
	}
}

func TestReviveRequiresCondition(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	if _, err := s.Init(); err != nil {
		t.Fatal(err)
	}
	ts := validTombstone("use client-side token refresh", "go test ./internal/auth")
	if err := s.Append(&ts); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Revive(ts.ID, ""); err == nil || !strings.Contains(err.Error(), "condition") {
		t.Fatalf("revive without any condition must explain itself, got %v", err)
	}
	if _, err := s.Revive("deadbeef", "now holds"); err == nil {
		t.Fatal("reviving an unknown id must fail")
	}
	if _, err := s.Revive(ts.ID, "the auth backend now accepts idempotent refresh"); err != nil {
		t.Fatalf("revive with a condition: %v", err)
	}
	if _, err := s.Revive(ts.ID, "again"); err == nil {
		t.Fatal("reviving twice must fail")
	}
}
