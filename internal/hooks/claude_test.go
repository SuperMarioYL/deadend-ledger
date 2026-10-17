package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readSettings(t *testing.T, root string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(SettingsPath(root))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestInstallFreshRepo(t *testing.T) {
	root := t.TempDir()
	modified, before, _, err := Install(root)
	if err != nil || !modified {
		t.Fatalf("Install: modified=%v err=%v", modified, err)
	}
	if before != "{}" {
		t.Fatalf("empty repo canonical before = %q", before)
	}
	settings := readSettings(t, root)
	hooksMap := settings["hooks"].(map[string]any)
	if _, ok := hooksMap["SessionStart"]; !ok {
		t.Fatal("SessionStart hook missing")
	}
	if _, ok := hooksMap["UserPromptSubmit"]; !ok {
		t.Fatal("UserPromptSubmit hook missing")
	}
	if !Installed(root) {
		t.Fatal("Installed() must report true after install")
	}
}

func TestInstallPreservesUnknownFields(t *testing.T) {
	root := t.TempDir()
	path := SettingsPath(root)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	original := `{
  "permissions": {"allow": ["Bash(go test:*)"]},
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "lint.sh", "unknownField": true}]}
    ]
  }
}`
	_ = os.WriteFile(path, []byte(original), 0o644)

	if _, _, _, err := Install(root); err != nil {
		t.Fatal(err)
	}
	settings := readSettings(t, root)
	if _, ok := settings["permissions"]; !ok {
		t.Fatal("permissions block was dropped")
	}
	pre := settings["hooks"].(map[string]any)["PreToolUse"].([]any)
	group := pre[0].(map[string]any)
	entry := group["hooks"].([]any)[0].(map[string]any)
	if _, ok := entry["unknownField"]; !ok {
		t.Fatal("unknown fields inside existing hook entries were dropped")
	}
	if entry["command"] != "lint.sh" {
		t.Fatalf("existing hook modified: %v", entry)
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	root := t.TempDir()
	if _, _, _, err := Install(root); err != nil {
		t.Fatal(err)
	}
	modified, _, _, err := Install(root)
	if err != nil {
		t.Fatal(err)
	}
	if modified {
		t.Fatal("second install must be a no-op")
	}
	settings := readSettings(t, root)
	hooksMap := settings["hooks"].(map[string]any)
	if n := len(hooksMap["SessionStart"].([]any)); n != 1 {
		t.Fatalf("SessionStart has %d groups after reinstall, want 1", n)
	}
}

func TestInstallRefusesCorruptSettings(t *testing.T) {
	root := t.TempDir()
	path := SettingsPath(root)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	broken := "{ not json"
	_ = os.WriteFile(path, []byte(broken), 0o644)
	_, _, _, err := Install(root)
	if err == nil || !strings.Contains(err.Error(), "not valid JSON") {
		t.Fatalf("corrupt settings must be refused, got: %v", err)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != broken {
		t.Fatal("corrupt settings file must be left untouched")
	}
}

func TestLineDiffShowsOnlyAdditions(t *testing.T) {
	// Merge into an existing settings file: every original line survives, so
	// the canonical diff must contain additions only.
	root := t.TempDir()
	path := SettingsPath(root)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte("{\n  \"permissions\": {\"allow\": [\"Bash(go test:*)\"]}\n}"), 0o644)
	modified, before, after, err := Install(root)
	if err != nil || !modified {
		t.Fatalf("Install: modified=%v err=%v", modified, err)
	}
	diff := LineDiff(before, after)
	if strings.Contains(diff, "- ") {
		t.Fatalf("additive merge must not produce removals:\n%s", diff)
	}
	for _, want := range []string{"+   \"hooks\": {", "\"SessionStart\": [", "deadend inject", "Bash(go test:*)"} {
		if !strings.Contains(diff, want) {
			t.Fatalf("diff missing %q:\n%s", want, diff)
		}
	}
}
