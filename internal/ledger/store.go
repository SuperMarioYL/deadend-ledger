package ledger

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// RelativeLedgerPath is the ledger location as shown to a user working in
// the repo root: ".deadend/ledger.jsonl".
func RelativeLedgerPath() string {
	return filepath.Join(DirName, FileName)
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
// dropping data from a do-not-retry ledger. When one id has several records
// (a revive or re-arm transition), the last record wins while the entry keeps
// the ordinal and line of that id's first appearance.
func (s *Store) Load() ([]Entry, error) {
	entries, _, err := s.read(false)
	return entries, err
}

// LoadBestEffort reads the ledger for the injection hook: records that fail to
// parse or validate are skipped and reported, so one corrupt line cannot stop
// every readable tombstone from injecting. Hooks must never break a session;
// show and record keep using the strict Load.
func (s *Store) LoadBestEffort() ([]Entry, []error) {
	entries, skipped, _ := s.read(true)
	return entries, skipped
}

func (s *Store) read(bestEffort bool) ([]Entry, []error, error) {
	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil // no ledger yet: zero tombstones, not an error
		}
		return nil, nil, err
	}
	defer f.Close()

	var entries []Entry
	var skipped []error
	position := map[string]int{} // id -> index in entries
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
			problem := fmt.Errorf("%s line %d: invalid JSON: %w", s.path, line, err)
			if bestEffort {
				skipped = append(skipped, problem)
				continue
			}
			return nil, nil, problem
		}
		if err := t.Validate(); err != nil {
			problem := fmt.Errorf("%s line %d: %v", s.path, line, err)
			if bestEffort {
				skipped = append(skipped, problem)
				continue
			}
			return nil, nil, problem
		}
		if i, ok := position[t.ID]; ok {
			entries[i].Tombstone = t // last record per id wins; Ordinal/Line stay at first appearance
			continue
		}
		position[t.ID] = len(entries)
		entries = append(entries, Entry{Tombstone: t, Ordinal: len(entries) + 1, Line: line})
	}
	if err := sc.Err(); err != nil {
		if bestEffort {
			return entries, append(skipped, err), nil
		}
		return nil, nil, err
	}
	return entries, skipped, nil
}

// Append validates the tombstone, refuses an exact duplicate (same id — the
// same failure with the same locator recorded twice), and writes one JSON
// line. A tombstone whose latest record is revived can be recorded again: the
// failure recurred, so the appended active record re-arms the same id. A
// single Write call on an O_APPEND handle keeps concurrent record / inject
// runs from interleaving partial lines.
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
		if e.Tombstone.ID == t.ID && e.Tombstone.Status != StatusRevived {
			return fmt.Errorf("tombstone #%d (%s) already records this approach and evidence; nothing to add", e.Ordinal, e.Tombstone.ID)
		}
	}
	return s.appendLine(t)
}

// Revive appends a transition record that marks the tombstone revived. The
// original record is never rewritten: Load resolves the last record per id, so
// the ledger stays append-only and mergeable. condition overrides the
// tombstone's revive_when when non-empty; one of them must be set.
func (s *Store) Revive(id, condition string) (Entry, error) {
	existing, err := s.Load()
	if err != nil {
		return Entry{}, err
	}
	for _, e := range existing {
		if e.Tombstone.ID != id {
			continue
		}
		if e.Tombstone.Status == StatusRevived {
			return Entry{}, fmt.Errorf("tombstone #%d (%s) is already revived", e.Ordinal, id)
		}
		next := e.Tombstone
		next.Status = StatusRevived
		if strings.TrimSpace(condition) != "" {
			next.ReviveWhen = strings.TrimSpace(condition)
		}
		if strings.TrimSpace(next.ReviveWhen) == "" {
			return Entry{}, fmt.Errorf("reviving tombstone #%d (%s) needs a condition: pass --revive-when", e.Ordinal, id)
		}
		if err := next.Validate(); err != nil {
			return Entry{}, fmt.Errorf("refusing to revive: %v", err)
		}
		if err := s.appendLine(&next); err != nil {
			return Entry{}, err
		}
		e.Tombstone = next
		return e, nil
	}
	return Entry{}, fmt.Errorf("no tombstone %q in %s — run `deadend review` to list ids", id, s.path)
}

// appendLine writes one validated record as a single line. A ledger whose
// final record was hand-edited without a trailing newline would otherwise
// concatenate two objects into one unparseable line, so the separator is
// written together with the record.
func (s *Store) appendLine(t *Tombstone) error {
	line, err := json.Marshal(t)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	// O_RDWR (not O_WRONLY) so the last byte can be inspected through ReadAt.
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	record := append(line, '\n')
	missingNewline, err := missingTrailingNewline(f)
	if err != nil {
		return err
	}
	if missingNewline {
		record = append([]byte{'\n'}, record...)
	}
	if _, err := f.Write(record); err != nil {
		return err
	}
	return f.Sync()
}

// missingTrailingNewline reports whether a non-empty ledger's last byte is not
// a newline. A leading blank line is harmless (Load skips empty lines), so an
// extra separator stays safe even if another writer races us.
func missingTrailingNewline(f *os.File) (bool, error) {
	info, err := f.Stat()
	if err != nil {
		return false, err
	}
	if info.Size() == 0 {
		return false, nil
	}
	var last [1]byte
	if _, err := f.ReadAt(last[:], info.Size()-1); err != nil {
		return false, err
	}
	return last[0] != '\n', nil
}
