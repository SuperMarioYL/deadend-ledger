// Command deadend is the tombstone ledger for failed coding-agent approaches:
// record a dead end once with machine evidence, and every later session gets
// it injected before the agent re-walks that path.
package main

import "github.com/SuperMarioYL/deadend-ledger/cmd"

// version is the release single source of truth in the root VERSION file;
// release builds override it via -ldflags "-X main.version=...".
var version = "0.1.0"

func main() {
	cmd.Execute(version)
}
