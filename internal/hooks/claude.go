// Package hooks installs the Claude Code SessionStart and UserPromptSubmit
// hook lines into a repo's .claude/settings.json. The merge is additive:
// existing settings (permissions, other hooks, unknown fields) are preserved
// untouched, and an existing deadend hook is detected so init stays idempotent.
package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HookCommand is the command line Claude Code runs; it must stay on PATH.
const HookCommand = "deadend inject"

// hookTimeoutSeconds bounds one injection run; the plan budgets < 5s.
const hookTimeoutSeconds = 10

const sessionStartMatcher = "startup|resume|clear"

// SettingsPath returns <repoRoot>/.claude/settings.json.
func SettingsPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".claude", "settings.json")
}

// Install merges the two hook groups into settings.json and reports whether
// the file changed. before/after are the canonical (re-marshaled) settings
// texts, so a caller can show a diff that contains only the semantic change,
// never key-ordering churn.
func Install(repoRoot string) (modified bool, before, after string, err error) {
	path := SettingsPath(repoRoot)
	existing := map[string]any{}

	raw, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(raw, &existing); err != nil {
			return false, "", "", fmt.Errorf("%s is not valid JSON; refusing to touch it: %v", path, err)
		}
	case os.IsNotExist(err):
		// fresh repo: start from an empty settings object
	default:
		return false, "", "", err
	}

	beforeRaw, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return false, "", "", err
	}

	hooksMap, ok := existing["hooks"].(map[string]any)
	if !ok {
		hooksMap = map[string]any{}
		existing["hooks"] = hooksMap
	}

	added := false
	if !hasDeadendHook(hooksMap, "SessionStart") {
		hooksMap["SessionStart"] = appendGroup(hooksMap["SessionStart"],
			map[string]any{"matcher": sessionStartMatcher, "hooks": []any{commandHook()}})
		added = true
	}
	if !hasDeadendHook(hooksMap, "UserPromptSubmit") {
		hooksMap["UserPromptSubmit"] = appendGroup(hooksMap["UserPromptSubmit"],
			map[string]any{"hooks": []any{commandHook()}})
		added = true
	}
	if !added {
		return false, string(beforeRaw), string(beforeRaw), nil
	}

	afterRaw, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return false, "", "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, "", "", err
	}
	if err := os.WriteFile(path, append(afterRaw, '\n'), 0o644); err != nil {
		return false, "", "", err
	}
	return true, string(beforeRaw), string(afterRaw), nil
}

// Installed reports whether both hook lines are already present.
func Installed(repoRoot string) bool {
	raw, err := os.ReadFile(SettingsPath(repoRoot))
	if err != nil {
		return false
	}
	var settings map[string]any
	if json.Unmarshal(raw, &settings) != nil {
		return false
	}
	hooksMap, ok := settings["hooks"].(map[string]any)
	if !ok {
		return false
	}
	return hasDeadendHook(hooksMap, "SessionStart") && hasDeadendHook(hooksMap, "UserPromptSubmit")
}

func commandHook() map[string]any {
	return map[string]any{
		"type":    "command",
		"command": HookCommand,
		"timeout": hookTimeoutSeconds,
	}
}

// appendGroup appends one hook group to the existing value for an event,
// keeping whatever shape the user already had (unknown fields included).
func appendGroup(v any, group map[string]any) []any {
	groups, ok := v.([]any)
	if !ok {
		groups = []any{}
	}
	return append(groups, group)
}

func hasDeadendHook(hooksMap map[string]any, event string) bool {
	groups, ok := hooksMap[event].([]any)
	if !ok {
		return false
	}
	for _, g := range groups {
		group, ok := g.(map[string]any)
		if !ok {
			continue
		}
		entries, ok := group["hooks"].([]any)
		if !ok {
			continue
		}
		for _, e := range entries {
			entry, ok := e.(map[string]any)
			if !ok {
				continue
			}
			if cmd, ok := entry["command"].(string); ok && strings.Contains(cmd, HookCommand) {
				return true
			}
		}
	}
	return false
}

// LineDiff renders a +/- diff between two canonical settings texts. Because
// both sides are re-marshaled identically, it shows only what the merge
// actually changes.
func LineDiff(before, after string) string {
	a := strings.Split(strings.TrimRight(before, "\n"), "\n")
	b := strings.Split(strings.TrimRight(after, "\n"), "\n")
	n, m := len(a), len(b)
	// lcs[i][j] = length of the longest common subsequence of a[i:], b[j:]
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			switch {
			case a[i] == b[j]:
				lcs[i][j] = lcs[i+1][j+1] + 1
			case lcs[i+1][j] >= lcs[i][j+1]:
				lcs[i][j] = lcs[i+1][j]
			default:
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	var out []string
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			out = append(out, "  "+a[i])
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			out = append(out, "- "+a[i])
			i++
		default:
			out = append(out, "+ "+b[j])
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, "- "+a[i])
	}
	for ; j < m; j++ {
		out = append(out, "+ "+b[j])
	}
	return strings.Join(out, "\n")
}
