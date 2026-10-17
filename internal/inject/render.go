// Package inject renders matched tombstones into the fixed block that lands
// at the top of planning context. Rendering is a pure function of the ledger
// and the task context: no model call, no network, no daemon.
package inject

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/SuperMarioYL/deadend-ledger/internal/ledger"
	"github.com/SuperMarioYL/deadend-ledger/internal/match"
)

// MaxBlockRunes is the hard cap on the rendered block. Whatever matches,
// injection can never flood planning context.
const MaxBlockRunes = 2400

// Match is a tombstone that cleared the threshold, with its ledger ordinal
// and score.
type Match struct {
	Tombstone ledger.Tombstone
	Ordinal   int
	Score     float64
}

// Collect scores every live tombstone against the context and returns the
// matches above the threshold, most relevant first.
func Collect(entries []ledger.Entry, context string, today time.Time) []Match {
	set := match.ContextSet(context)
	var matches []Match
	for _, e := range entries {
		if !e.Tombstone.LiveOn(today) {
			continue // expired or revived tombstones stop injecting
		}
		if score, ok := match.Matches(set, e.Tombstone.Scope); ok {
			matches = append(matches, Match{Tombstone: e.Tombstone, Ordinal: e.Ordinal, Score: score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].Score > matches[j].Score })
	return matches
}

// Render draws the fixed injection template. Matches that no longer fit are
// dropped (never truncated mid-record) and counted in a closing note.
func Render(matches []Match) string {
	if len(matches) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<deadend-ledger active-tombstones>\n")
	fmt.Fprintf(&b, "The following approach(es) already failed in this repo. Do NOT retry them unless the revive condition now holds.\n\n")

	shown, omitted := 0, 0
	reserve := len(closingNote(9999)) // always keep room for the closing note
	for _, m := range matches {
		candidate := renderTombstone(m)
		if utf8.RuneCountInString(b.String())+utf8.RuneCountInString(candidate)+reserve > MaxBlockRunes {
			omitted = len(matches) - shown
			break
		}
		b.WriteString(candidate)
		shown++
	}
	if omitted > 0 {
		b.WriteString(closingNote(omitted))
	}
	b.WriteString("</deadend-ledger>\n")
	return b.String()
}

func closingNote(n int) string {
	return fmt.Sprintf("\n(%d more matching tombstone(s) omitted — run: deadend show)\n", n)
}

func renderTombstone(m Match) string {
	t := m.Tombstone
	var b strings.Builder
	fmt.Fprintf(&b, "[tombstone #%d] %s\n", m.Ordinal, oneLine(t.Approach))
	fmt.Fprintf(&b, "  scope: %s\n", oneLine(strings.Join(t.Scope, ", ")))
	fmt.Fprintf(&b, "  evidence: %s | %s (observed %s)\n", t.Evidence.Kind, oneLine(t.Evidence.Locator), t.Evidence.ObservedAt)
	if t.Evidence.Excerpt != "" {
		fmt.Fprintf(&b, "  excerpt: %s\n", oneLine(t.Evidence.Excerpt))
	}
	if t.ReviveWhen != "" {
		fmt.Fprintf(&b, "  do-not-retry unless: %s\n", oneLine(t.ReviveWhen))
	}
	return b.String()
}

// oneLine collapses whitespace runs so the injection template keeps one
// field per line; the ledger itself stores the verbatim paste.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// Block is the one-call entry point: ledger entries plus task context in,
// finished injection block (possibly empty) out.
func Block(entries []ledger.Entry, context string, today time.Time) string {
	return Render(Collect(entries, context, today))
}
