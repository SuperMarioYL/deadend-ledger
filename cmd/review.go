package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// errUnimplemented marks the v0.1 stub surface. The review/revive operations
// (status transitions, age listing, revive conditions) ship with the
// review/revive milestone; the ledger schema already carries status,
// expires_at and revive_when for them.
var errUnimplemented = errors.New("not implemented in v0.1")

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "List tombstones with status, age and evidence (planned)",
	Long: `Planned review surface: weekly sweep over the ledger with status,
age and evidence per tombstone. Not implemented in v0.1 — use
` + "`deadend show`" + `, which already lists every tombstone with its evidence.`,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.ErrOrStderr(), "deadend review ships with the review/revive milestone and is not implemented in v0.1.")
		fmt.Fprintln(cmd.ErrOrStderr(), "Until then: deadend show lists every tombstone with its status and evidence.")
		return errUnimplemented
	},
}

var reviveCmd = &cobra.Command{
	Use:   "revive <id>",
	Short: "Lift a tombstone whose revive condition now holds (planned)",
	Args:  cobra.ExactArgs(1),
	Long: `Planned operation: mark a tombstone revived once its revive condition
holds, so it stops gating planning context. Not implemented in v0.1.`,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.ErrOrStderr(), "deadend revive ships with the review/revive milestone and is not implemented in v0.1.")
		fmt.Fprintln(cmd.ErrOrStderr(), "Tombstones with a past expires_at already stop injecting on their own.")
		return errUnimplemented
	},
}

func init() {
	rootCmd.AddCommand(reviewCmd)
	rootCmd.AddCommand(reviveCmd)
}
