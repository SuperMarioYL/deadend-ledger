package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/SuperMarioYL/deadend-ledger/internal/hooks"
	"github.com/SuperMarioYL/deadend-ledger/internal/ledger"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create the ledger and install the Claude Code hook lines",
	Long: `Creates .deadend/ledger.jsonl and merges the SessionStart and
UserPromptSubmit hook lines into .claude/settings.json, showing the
diff before anything is written. Existing settings are preserved.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	repo, err := os.Getwd()
	if err != nil {
		return err
	}

	store := ledger.Open(repo)
	created, err := store.Init()
	if err != nil {
		return err
	}
	if created {
		fmt.Printf("created %s\n", ledger.LedgerPath(repo))
	} else {
		fmt.Printf("%s already exists — tombstones kept\n", ledger.LedgerPath(repo))
	}

	modified, before, after, err := hooks.Install(repo)
	if err != nil {
		return err
	}
	if modified {
		fmt.Println("merging hook lines into .claude/settings.json:")
		fmt.Println(hooks.LineDiff(before, after))
	} else {
		fmt.Println("hook lines already present in .claude/settings.json")
	}

	fmt.Println()
	fmt.Println("next: when the agent hits a dead end, capture it with `deadend record`")
	return nil
}
