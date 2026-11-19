package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/SuperMarioYL/deadend-ledger/internal/ledger"
)

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "List tombstones with status, age and evidence",
	Long: `Lists every tombstone with its status, age in days, whether it injects
today, and its machine-checkable evidence. Revived and expired tombstones stay
in the list so a weekly sweep sees the whole ledger.`,
	RunE: runReview,
}

var reviveCmd = &cobra.Command{
	Use:   "revive <id>",
	Short: "Lift a tombstone whose revive condition now holds",
	Args:  cobra.ExactArgs(1),
	Long: `Appends a status-transition record marking the tombstone revived, so it stops
gating planning context. The condition comes from --revive-when or the
tombstone's own revive_when. The ledger stays append-only: the original record
is never rewritten.`,
	RunE: runRevive,
}

var reviveFlags struct {
	reviveWhen string
}

func init() {
	reviveCmd.Flags().StringVar(&reviveFlags.reviveWhen, "revive-when", "", "condition that now holds; overrides the tombstone's stored condition")
	rootCmd.AddCommand(reviewCmd)
	rootCmd.AddCommand(reviveCmd)
}

func runReview(cmd *cobra.Command, args []string) error {
	repo, err := os.Getwd()
	if err != nil {
		return err
	}
	entries, err := ledger.Open(repo).Load()
	if err != nil {
		return err
	}
	renderReview(cmd.OutOrStdout(), entries, time.Now())
	return nil
}

func runRevive(cmd *cobra.Command, args []string) error {
	repo, err := os.Getwd()
	if err != nil {
		return err
	}
	entry, err := ledger.Open(repo).Revive(args[0], reviveFlags.reviveWhen)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "tombstone #%d (%s) revived — it stops injecting; revive when: %s\n",
		entry.Ordinal, entry.Tombstone.ID, entry.Tombstone.ReviveWhen)
	return nil
}

// renderReview draws the weekly sweep: a one-line summary, then every tombstone
// with its age and whether it still gates planning context.
func renderReview(w io.Writer, entries []ledger.Entry, today time.Time) {
	if len(entries) == 0 {
		fmt.Fprintln(w, "ledger is empty — record the next dead end with `deadend record`")
		return
	}
	var injecting, pastDue, expired, revived int
	for _, e := range entries {
		t := e.Tombstone
		switch {
		case t.Status == ledger.StatusRevived:
			revived++
		case t.Status == ledger.StatusExpired:
			expired++
		case t.LiveOn(today):
			injecting++
		default:
			pastDue++
		}
	}
	fmt.Fprintf(w, "%d tombstone(s): %d injecting, %d past due, %d expired, %d revived\n\n",
		len(entries), injecting, pastDue, expired, revived)

	for _, e := range entries {
		t := e.Tombstone
		state := "injecting"
		if !t.LiveOn(today) {
			state = "not injecting — " + notInjectedReason(t, today)
		}
		fmt.Fprintf(w, "#%d [%s] %s · %s · %s\n", e.Ordinal, t.ID, t.Status, ageInDays(today, t.CreatedAt), state)
		fmt.Fprintf(w, "   approach: %s\n", t.Approach)
		fmt.Fprintf(w, "   scope: %s\n", strings.Join(t.Scope, ", "))
		fmt.Fprintf(w, "   evidence: %s | %s (observed %s)\n", t.Evidence.Kind, t.Evidence.Locator, t.Evidence.ObservedAt)
		if t.Evidence.Excerpt != "" {
			fmt.Fprintf(w, "   excerpt: %s\n", t.Evidence.Excerpt)
		}
		if t.ExpiresAt != "" {
			fmt.Fprintf(w, "   expires: %s\n", t.ExpiresAt)
		}
		if t.ReviveWhen != "" {
			fmt.Fprintf(w, "   revive when: %s\n", t.ReviveWhen)
		}
		fmt.Fprintln(w)
	}
}

// ageInDays renders how long a tombstone has been in the ledger. created_at is
// validated on load, so an unparseable value only appears in hand-built entries.
func ageInDays(today time.Time, created string) string {
	createdAt, err := time.Parse(ledger.DateFormat, created)
	if err != nil {
		return "age unknown"
	}
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	days := int(todayDate.Sub(createdAt).Hours() / 24)
	if days <= 0 {
		return "recorded today"
	}
	if days == 1 {
		return "1 day old"
	}
	return fmt.Sprintf("%d days old", days)
}
