// Package ledger owns the dead-end tombstone primitive: its schema, validation,
// status semantics, and the append-only JSONL store that persists it.
package ledger

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Tombstone statuses.
const (
	StatusActive  = "active"
	StatusExpired = "expired"
	StatusRevived = "revived"
)

// Evidence kinds. Every tombstone must carry machine-checkable evidence;
// prose recollections of failure are explicitly out of scope.
const (
	KindFailingTest = "failing_test"
	KindErrorSpan   = "error_span"
	KindBuildLog    = "build_log"
)

// DateFormat is the date layout used by observed_at, created_at and expires_at.
const DateFormat = "2006-01-02"

// MaxExcerptRunes caps the evidence excerpt so a single tombstone can never
// flood the planning context it later gets injected into.
const MaxExcerptRunes = 400

// MaxApproachRunes caps the approach wording; a tombstone is a verdict, not a report.
const MaxApproachRunes = 200

// Evidence is the machine-checkable proof that an approach is dead.
type Evidence struct {
	Kind       string `json:"kind"`        // failing_test | error_span | build_log
	Locator    string `json:"locator"`     // e.g. "go test ./internal/auth -run TestTokenRefresh"
	Excerpt    string `json:"excerpt"`     // capped at MaxExcerptRunes
	ObservedAt string `json:"observed_at"` // DateFormat
}

// Tombstone records one failed approach: what was tried, where it applies
// (the matching key), and the evidence that proves it dead.
type Tombstone struct {
	ID         string   `json:"id"`       // short hash of approach + evidence; stable across sessions
	Approach   string   `json:"approach"` // imperative: "use client-side token refresh"
	Scope      []string `json:"scope"`    // files, symbols, packages touched — the matching key
	Evidence   Evidence `json:"evidence"`
	Status     string   `json:"status"`                // active | expired | revived
	CreatedAt  string   `json:"created_at"`            // DateFormat
	ExpiresAt  string   `json:"expires_at,omitempty"`  // tombstone rot: past due stops injection
	ReviveWhen string   `json:"revive_when,omitempty"` // condition under which the approach is worth retrying
}

// NewID derives the stable tombstone id from the approach wording and the
// evidence locator. The same failure recorded twice yields the same id, which
// is what makes duplicate records detectable.
func NewID(approach, locator string) string {
	sum := sha256.Sum256([]byte(approach + "\x00" + locator))
	return hex.EncodeToString(sum[:])[:8]
}

// CapExcerpt truncates an evidence excerpt to MaxExcerptRunes, rune-safe,
// marking the cut with an ellipsis.
func CapExcerpt(s string) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= MaxExcerptRunes {
		return s
	}
	runes := []rune(s)
	return strings.TrimRight(string(runes[:MaxExcerptRunes-1]), " \t") + "…"
}

// Validate checks one tombstone against the schema. Every append and every
// load passes through it: a malformed tombstone never enters the ledger.
func (t *Tombstone) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("id is required")
	}
	if len(t.ID) != 8 {
		return fmt.Errorf("id %q: want 8 hex chars", t.ID)
	}
	switch {
	case t.Approach == "":
		return fmt.Errorf("approach is required")
	case utf8.RuneCountInString(t.Approach) > MaxApproachRunes:
		return fmt.Errorf("approach exceeds %d chars", MaxApproachRunes)
	}
	if len(t.Scope) == 0 {
		return fmt.Errorf("scope needs at least one entry (files, symbols or packages)")
	}
	for i, s := range t.Scope {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("scope[%d] is empty", i)
		}
	}
	if err := validateEvidence(t.Evidence); err != nil {
		return err
	}
	switch t.Status {
	case StatusActive, StatusExpired, StatusRevived:
	default:
		return fmt.Errorf("status %q: want active, expired or revived", t.Status)
	}
	if err := validDate("created_at", t.CreatedAt, true); err != nil {
		return err
	}
	if err := validDate("expires_at", t.ExpiresAt, false); err != nil {
		return err
	}
	if t.Status == StatusRevived && t.ReviveWhen == "" {
		// A revived tombstone must name what changed; otherwise revive is noise.
		return fmt.Errorf("status revived requires revive_when")
	}
	return nil
}

func validateEvidence(e Evidence) error {
	switch e.Kind {
	case KindFailingTest, KindErrorSpan, KindBuildLog:
	default:
		return fmt.Errorf("evidence.kind %q: want failing_test, error_span or build_log", e.Kind)
	}
	if strings.TrimSpace(e.Locator) == "" {
		return fmt.Errorf("evidence.locator is required (the machine-checkable command or span)")
	}
	if n := utf8.RuneCountInString(e.Excerpt); n > MaxExcerptRunes {
		return fmt.Errorf("evidence.excerpt is %d chars, cap is %d", n, MaxExcerptRunes)
	}
	return validDate("evidence.observed_at", e.ObservedAt, true)
}

func validDate(field, value string, required bool) error {
	if value == "" {
		if required {
			return fmt.Errorf("%s is required", field)
		}
		return nil
	}
	if _, err := time.Parse(DateFormat, value); err != nil {
		return fmt.Errorf("%s %q: want %s", field, value, DateFormat)
	}
	return nil
}

// LiveOn reports whether the tombstone should still be injected on the given
// date: active status and not past its expiry (the expiry date itself is the
// last live day). Revived tombstones stay out of injection — revive means
// "worth retrying", which is the opposite of a do-not-retry marker.
func (t *Tombstone) LiveOn(today time.Time) bool {
	if t.Status != StatusActive {
		return false
	}
	if t.ExpiresAt == "" {
		return true
	}
	expires, err := time.Parse(DateFormat, t.ExpiresAt)
	if err != nil {
		return false // unparseable expiry in a loaded record: fail closed
	}
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	return !todayDate.After(expires)
}
