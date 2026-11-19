# Changelog

All notable changes to deadend-ledger are recorded here. Versions follow
semantic versioning.

## v0.2.0 — 2026-11-20

The review/revive lifecycle announced for v0.1 is implemented, and three
ledger-safety defects found in the shipped v0.1 code are fixed.

### Fixes

- **Append is safe on a hand-edited ledger.** Recording onto a file whose last
  line had lost its trailing newline used to concatenate both JSON objects into
  one unparseable line, losing both records. Append now writes the separator
  when it is missing.
- **The scope suggestion is usable as a matching key.** `deadend record` reads
  `git status --porcelain -z`, so a non-ASCII filename is no longer recorded as
  git's octal-quoted escape, and the `.deadend/` and `.claude/` directories that
  `init` creates are never offered as scope.
- **Injection survives a corrupt ledger line.** `deadend inject` skips records
  it cannot parse or validate, reports them on stderr, and still renders the
  readable tombstones. One damaged line no longer turns the session hook into an
  error that disables every tombstone.

### Added

- **`deadend review`** lists every tombstone with its status, age in days,
  whether it injects today, and its evidence.
- **`deadend revive <id>`** appends a status-transition record once a revive
  condition holds (`--revive-when`, or the condition stored on the tombstone).
  Status changes stay append-only: the ledger keeps the original record, and
  reads resolve the last record per id while the ordinal stays at the id's first
  appearance.
- **Re-recording a revived failure re-arms it.** If the same approach and
  locator fail again after a revive, `deadend record` appends an active record
  for the same tombstone instead of refusing it as a duplicate.
- **Command-layer end-to-end tests** cover the record → inject → review →
  revive → re-arm lifecycle.

### Docs and site

- The bilingual README now documents the shipped lifecycle; the roadmap and
  limitations no longer describe review/revive as planned.
- Product site content version refreshed to 0.2.0.

## v0.1.0

Initial release: tombstone schema and validation, append-only JSONL ledger,
deterministic scope matching, fixed-template injection with a hard context cap,
and the Claude Code hook installer.
