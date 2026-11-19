package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/SuperMarioYL/deadend-ledger/internal/ledger"
)

var recordFlags struct {
	approach   string
	scope      []string
	kind       string
	locator    string
	excerpt    string
	file       string
	observed   string
	expires    string
	reviveWhen string
	yes        bool
}

var recordCmd = &cobra.Command{
	Use:   "record",
	Short: "Record one failed approach as a tombstone with machine evidence",
	Long: `Records a dead-end tombstone into .deadend/ledger.jsonl.

All fields can be passed as flags (scriptable); anything missing is asked
for interactively, with scope defaulting to the files changed in this repo.
The evidence excerpt is capped at 400 characters.`,
	RunE: runRecord,
}

func init() {
	recordCmd.Flags().StringVarP(&recordFlags.approach, "approach", "a", "", "what was tried, imperative (\"use client-side token refresh\")")
	recordCmd.Flags().StringSliceVarP(&recordFlags.scope, "scope", "s", nil, "files, symbols or packages touched (comma-separated)")
	recordCmd.Flags().StringVar(&recordFlags.kind, "kind", ledger.KindFailingTest, "evidence kind: failing_test, error_span or build_log")
	recordCmd.Flags().StringVarP(&recordFlags.locator, "locator", "l", "", "machine-checkable locator (\"go test ./internal/auth -run TestTokenRefresh\")")
	recordCmd.Flags().StringVarP(&recordFlags.excerpt, "excerpt", "e", "", "capped error excerpt")
	recordCmd.Flags().StringVarP(&recordFlags.file, "file", "f", "", "read the evidence output from a file (\"-\" for stdin)")
	recordCmd.Flags().StringVar(&recordFlags.observed, "observed-at", "", "date the failure was observed (YYYY-MM-DD, default today)")
	recordCmd.Flags().StringVar(&recordFlags.expires, "expires", "", "optional expiry date; past due stops injection (YYYY-MM-DD)")
	recordCmd.Flags().StringVar(&recordFlags.reviveWhen, "revive-when", "", "condition under which the approach is worth retrying")
	recordCmd.Flags().BoolVarP(&recordFlags.yes, "yes", "y", false, "scripted mode: take everything from flags, no prompts, no confirm")
	rootCmd.AddCommand(recordCmd)
}

func runRecord(cmd *cobra.Command, args []string) error {
	repo, err := os.Getwd()
	if err != nil {
		return err
	}
	stdinIsTTY := isTerminal(os.Stdin)
	// --yes forces the scripted path: flags only, no prompts, no confirm.
	interactive := stdinIsTTY && !recordFlags.yes
	prompt := newPrompter(os.Stdin, cmd.OutOrStdout())

	approach := recordFlags.approach
	if approach == "" {
		if !interactive {
			return errors.New("approach is required: pass --approach or run interactively in a terminal")
		}
		approach = prompt.required("approach (imperative, e.g. use client-side token refresh)")
	}

	scope := cleanScope(recordFlags.scope)
	if len(scope) == 0 && interactive {
		scope = prompt.scope(suggestScope(repo))
	}
	if len(scope) == 0 {
		return errors.New("scope needs at least one entry: pass --scope file,symbol,package")
	}

	kind := recordFlags.kind
	if kind == "" {
		kind = ledger.KindFailingTest
	}

	locator := recordFlags.locator
	if locator == "" {
		if !interactive {
			return errors.New("locator is required: pass --locator (the machine-checkable command or span)")
		}
		locator = prompt.required("evidence locator (e.g. go test ./internal/auth -run TestTokenRefresh)")
	}

	excerpt, err := resolveExcerpt(cmd, prompt, interactive)
	if err != nil {
		return err
	}

	observed := recordFlags.observed
	if observed == "" {
		observed = time.Now().Format(ledger.DateFormat)
	}
	expires := recordFlags.expires
	if expires == "" && interactive {
		expires = prompt.date("expires, past due stops injection (YYYY-MM-DD, Enter = never)")
	}
	reviveWhen := recordFlags.reviveWhen
	if reviveWhen == "" && interactive {
		reviveWhen = prompt.optional("revive when — what would make this worth retrying (Enter = none)")
	}
	ts := ledger.Tombstone{
		Approach:   approach,
		Scope:      scope,
		Status:     ledger.StatusActive,
		CreatedAt:  observed,
		ExpiresAt:  expires,
		ReviveWhen: reviveWhen,
		Evidence: ledger.Evidence{
			Kind:       kind,
			Locator:    locator,
			Excerpt:    ledger.CapExcerpt(excerpt),
			ObservedAt: observed,
		},
	}
	ts.ID = ledger.NewID(ts.Approach, ts.Evidence.Locator)

	if err := ts.Validate(); err != nil {
		return fmt.Errorf("refusing to record: %v", err)
	}

	printSummary(cmd.OutOrStdout(), ts)
	if interactive {
		if !prompt.confirm("Record this tombstone?") {
			fmt.Fprintln(cmd.OutOrStdout(), "aborted — nothing written")
			return nil
		}
	}

	store := ledger.Open(repo)
	existing, err := store.Load()
	if err != nil {
		return err
	}
	rearmed := false
	for _, e := range existing {
		if e.Tombstone.ID == ts.ID && e.Tombstone.Status == ledger.StatusRevived {
			rearmed = true
		}
	}
	if err := store.Append(&ts); err != nil {
		return err
	}
	entries, err := store.Load()
	if err != nil {
		return err
	}
	ordinal := 0
	for _, e := range entries {
		if e.Tombstone.ID == ts.ID {
			ordinal = e.Ordinal
			break
		}
	}
	if ordinal == 0 {
		return fmt.Errorf("tombstone %s is missing after the append", ts.ID)
	}
	verb := "recorded"
	if rearmed {
		verb = "re-armed"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "tombstone #%d %s (%s) — the next matching session starts with it\n", ordinal, verb, ts.ID)
	return nil
}

// resolveExcerpt applies the excerpt precedence: --excerpt, then --file
// (including "-" for stdin), then the interactive paste, else empty.
func resolveExcerpt(cmd *cobra.Command, p *prompter, interactive bool) (string, error) {
	switch {
	case recordFlags.excerpt != "":
		return recordFlags.excerpt, nil
	case recordFlags.file != "":
		var data []byte
		var err error
		if recordFlags.file == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(recordFlags.file)
		}
		if err != nil {
			return "", err
		}
		return string(data), nil
	case interactive:
		return p.paste("paste the failing output, finish with a line containing only END"), nil
	default:
		return "", nil
	}
}

func printSummary(w io.Writer, ts ledger.Tombstone) {
	trim := func(s string, n int) string {
		if utf8.RuneCountInString(s) > n {
			return string([]rune(s)[:n]) + "…"
		}
		return s
	}
	fmt.Fprintf(w, "\napproach    %s\n", ts.Approach)
	fmt.Fprintf(w, "scope       %s\n", strings.Join(ts.Scope, ", "))
	fmt.Fprintf(w, "evidence    %s | %s (observed %s)\n", ts.Evidence.Kind, ts.Evidence.Locator, ts.Evidence.ObservedAt)
	if ts.Evidence.Excerpt != "" {
		fmt.Fprintf(w, "excerpt     %s\n", trim(ts.Evidence.Excerpt, 120))
	}
	expires := ts.ExpiresAt
	if expires == "" {
		expires = "never"
	}
	fmt.Fprintf(w, "expires     %s\n", expires)
	revive := ts.ReviveWhen
	if revive == "" {
		revive = "—"
	}
	fmt.Fprintf(w, "revive when %s\n\n", revive)
}

// prompter asks for missing record fields. It only exists when stdin is a
// terminal; scripted runs must pass flags instead.
type prompter struct {
	reader *bufio.Reader
	out    io.Writer
}

func newPrompter(in io.Reader, out io.Writer) *prompter {
	return &prompter{reader: bufio.NewReader(in), out: out}
}

func (p *prompter) required(label string) string {
	for {
		fmt.Fprintf(p.out, "%s: ", label)
		line, _ := p.reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
		fmt.Fprintln(p.out, "  (required)")
	}
}

// optional accepts an empty answer as "not set".
func (p *prompter) optional(label string) string {
	fmt.Fprintf(p.out, "%s: ", label)
	line, _ := p.reader.ReadString('\n')
	return strings.TrimSpace(line)
}

// date keeps asking until the answer is empty or a valid ledger date.
func (p *prompter) date(label string) string {
	for {
		line := p.optional(label)
		if line == "" {
			return ""
		}
		if _, err := time.Parse(ledger.DateFormat, line); err == nil {
			return line
		}
		fmt.Fprintf(p.out, "  (want %s)\n", ledger.DateFormat)
	}
}

// scope asks for the matching key, offering the suggested files as default.
func (p *prompter) scope(suggestion []string) []string {
	for {
		if len(suggestion) > 0 {
			fmt.Fprintf(p.out, "scope (comma-separated; Enter = %s): ", strings.Join(suggestion, ", "))
		} else {
			fmt.Fprint(p.out, "scope (comma-separated files, symbols or packages): ")
		}
		line, _ := p.reader.ReadString('\n')
		if got := cleanScope(strings.Split(line, ",")); len(got) > 0 {
			return got
		}
		if len(suggestion) > 0 {
			return suggestion
		}
		fmt.Fprintln(p.out, "  (at least one entry is required)")
	}
}

// paste reads a multi-line evidence excerpt until a lone END line or EOF.
func (p *prompter) paste(label string) string {
	fmt.Fprintf(p.out, "%s\n", label)
	var lines []string
	for {
		line, err := p.reader.ReadString('\n')
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "END" || errors.Is(err, io.EOF) {
			break
		}
		lines = append(lines, trimmed)
		if err != nil {
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (p *prompter) confirm(question string) bool {
	fmt.Fprintf(p.out, "%s [y/N] ", question)
	line, _ := p.reader.ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes"
}

func cleanScope(in []string) []string {
	var out []string
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// suggestScope offers the files changed in this repo as the default matching
// key; outside a git repo or on a clean tree it returns nothing.
func suggestScope(repo string) []string {
	out, err := runGit(repo, "status", "--porcelain", "-z")
	if err != nil {
		return nil
	}
	return parseStatusPaths(out)
}

// maxSuggestedScope caps how many paths the interactive default offers.
const maxSuggestedScope = 8

// parseStatusPaths turns NUL-separated `git status --porcelain -z` output into
// scope keys. -z keeps paths unquoted and unescaped, so a non-ASCII filename
// stays the real path instead of git's C-quoted octal escape. Rename and copy
// records carry the destination path first, then the source path as the next
// NUL record. The tool's own .deadend/ and .claude/ state are never a matching
// key, and deadend init creates both, so they are skipped.
func parseStatusPaths(out string) []string {
	var files []string
	records := strings.Split(out, "\x00")
	for i := 0; i < len(records) && len(files) < maxSuggestedScope; i++ {
		rec := records[i]
		if len(rec) < 4 {
			continue
		}
		status, path := rec[:2], rec[3:]
		if status[0] == 'R' || status[0] == 'C' {
			i++ // the next record is the pre-rename source; keep the destination
		}
		if ignoredScopePath(path) {
			continue
		}
		if path != "" {
			files = append(files, path)
		}
	}
	return files
}

func ignoredScopePath(path string) bool {
	path = strings.TrimSuffix(path, "/")
	return path == ".deadend" || path == ".claude" ||
		strings.HasPrefix(path, ".deadend/") || strings.HasPrefix(path, ".claude/")
}

func runGit(dir string, args ...string) (string, error) {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return "", fmt.Errorf("not a git repo")
	}
	return silentExec(dir, "git", args...)
}
