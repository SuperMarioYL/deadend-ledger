package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/SuperMarioYL/deadend-ledger/internal/ledger"
)

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Render every tombstone in the ledger with its evidence",
	Long: `Lists all tombstones with status, scope and the machine-checkable
evidence locator. This is the v0.1 review surface; status transitions
(expire/revive) ship with the review/revive milestone.`,
	RunE: runShow,
}

func init() {
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	repo, err := os.Getwd()
	if err != nil {
		return err
	}
	entries, err := ledger.Open(repo).Load()
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if len(entries) == 0 {
		fmt.Fprintln(out, "ledger is empty — record the next dead end with `deadend record`")
		return nil
	}
	today := time.Now()
	for _, e := range entries {
		t := e.Tombstone
		fmt.Fprintf(out, "#%d [%s] %s · recorded %s", e.Ordinal, t.ID, t.Status, t.CreatedAt)
		if t.ExpiresAt != "" {
			fmt.Fprintf(out, " · expires %s", t.ExpiresAt)
		}
		fmt.Fprintln(out)
		fmt.Fprintf(out, "   approach: %s\n", t.Approach)
		fmt.Fprintf(out, "   scope: %s\n", strings.Join(t.Scope, ", "))
		fmt.Fprintf(out, "   evidence: %s | %s (observed %s)\n", t.Evidence.Kind, t.Evidence.Locator, t.Evidence.ObservedAt)
		if t.Evidence.Excerpt != "" {
			fmt.Fprintf(out, "   excerpt: %s\n", t.Evidence.Excerpt)
		}
		if t.ReviveWhen != "" {
			fmt.Fprintf(out, "   revive when: %s\n", t.ReviveWhen)
		}
		if !t.LiveOn(today) {
			fmt.Fprintf(out, "   (not injected: %s)\n", notInjectedReason(t, today))
		}
		fmt.Fprintln(out)
	}
	fmt.Fprintf(out, "%d tombstone(s) in %s\n", len(entries), ledger.RelativeLedgerPath())
	return nil
}

func notInjectedReason(t ledger.Tombstone, today time.Time) string {
	switch t.Status {
	case ledger.StatusRevived:
		return "revived — worth retrying, no longer a do-not-retry marker"
	case ledger.StatusExpired:
		return "marked expired"
	default:
		if t.ExpiresAt != "" {
			return "past due (" + t.ExpiresAt + ")"
		}
		return "inactive"
	}
}
