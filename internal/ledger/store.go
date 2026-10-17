package ledger

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// FileName is the ledger file every repo keeps under its .deadend directory.
const (
	DirName  = ".deadend"
	FileName = "ledger.jsonl"
)

// Entry pairs a tombstone with its 1-based position in the ledger. The
// ordinal ("tombstone #3") is stable because the store is append-only.
type Entry struct {
	Tombstone Tombstone
	Ordinal   int // 1-based position in the ledger file
	Line      int // 1-based physical line, for corruption reports
}

// Store is the append-only JSONL ledger at <repo>/.deadend/ledger.jsonl.
// One line per tombstone, safe to commit, safe to merge for text conflicts.
type Store struct {
	path string
}

// LedgerPath returns the ledger file path for a repo root.
func LedgerPath(repoRoot string) string {
	return filepath.Join(repoRoot, DirName, FileName)
}

// Open returns the store for a repo root. The file itself is created by Init.
func Open(repoRoot string) *Store {
	return &Store{path: LedgerPath(repoRoot)}
}

// Path exposes the backing file for messages and tests.
func (s *Store) Path() string { return s.path }

// Exists reports whether the ledger file is present.
func (s *Store) Exists() bool {
	_, err := os.Stat(s.path)
	return err == nil
}

// Init creates the ledger file if missing. The .deadend directory is created
// with 0755 so the JSONL stays committable; the file gets 0644.
func (s *Store) Init() (created bool, err error) {
	if _, err := os.Stat(s.path); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("stat %s: %w", s.path, err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return false, fmt.Errorf("create %s: %w", filepath.Dir(s.path), err)
	}
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return false, nil // lost a race with another init; that is fine
		}
		return false, fmt.Errorf("create %s: %w", s.path, err)
	}
	return true, f.Close()
}

// Load reads every tombstone in the ledger. Validation runs on each record:
// a corrupt line aborts the load with its line number rather than silently
// dropping data from a do-not-retry ledger.
func (s *Store) Load() ([]Entry, error) {
	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no ledger yet: zero tombstones, not an error
		}
		return nil, err
	}
	defer f.Close()

	var entries []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024) // excerpts cap at 400 chars, but leave headroom
	line := 0
	for sc.Scan() {
		line++
		raw := sc.Bytes()
		if len(raw) == 0 {
			continue
		}
		var t Tombstone
		if err := json.Unmarshal(raw, &t); err != nil {
			return nil, fmt.Errorf("%s line %d: invalid JSON: %w", s.path, line, err)
		}
		if err := t.Validate(); err != nil {
			return nil, fmt.Errorf("%s line %d: %v", s.path, line, err)
		}
		entries = append(entries, Entry{Tombstone: t, Ordinal: len(entries) + 1, Line: line})
	}
	return entries, sc.Err()
}

// Append validates the tombstone, refuses an exact duplicate (same id — the
// same failure with the same locator recorded twice), and writes one JSON
// line. A single Write call on an O_APPEND handle keeps concurrent record /
// inject runs from interleaving partial lines.
func (s *Store) Append(t *Tombstone) error {
	if t.ID == "" {
		t.ID = NewID(t.Approach, t.Evidence.Locator)
	}
	if err := t.Validate(); err != nil {
		return fmt.Errorf("refusing to record invalid tombstone: %v", err)
	}
	existing, err := s.Load()
	if err != nil {
		return err
	}
	for _, e := range existing {
		if e.Tombstone.ID == t.ID {
			return fmt.Errorf("tombstone #%d (%s) already records this approach and evidence; nothing to add", e.Ordinal, e.Tombstone.ID)
		}
	}
	line, err := json.Marshal(t)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return f.Sync()
}
