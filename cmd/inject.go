package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/SuperMarioYL/deadend-ledger/internal/inject"
	"github.com/SuperMarioYL/deadend-ledger/internal/ledger"
)

const (
	maxTranscriptMessages = 3
	maxTranscriptRunes    = 8000
)

var injectFlags struct {
	context string
}

var injectCmd = &cobra.Command{
	Use:   "inject",
	Short: "Print the tombstone block for a task context (Claude Code hook entry point)",
	Long: `Reads the Claude Code hook payload on stdin (UserPromptSubmit prompt,
SessionStart transcript) or --context, scores it against live tombstones and
prints matched tombstones as a fixed block for planning context.

No match, no output. No ledger, no output. No model call, no network.`,
	SilenceUsage: true,
	RunE:         runInject,
}

func init() {
	injectCmd.Flags().StringVar(&injectFlags.context, "context", "", "task context text (default: read the hook payload on stdin)")
	rootCmd.AddCommand(injectCmd)
}

func runInject(cmd *cobra.Command, args []string) error {
	repo, err := os.Getwd()
	if err != nil {
		return err
	}

	context := injectFlags.context
	if context == "" {
		if isTerminal(os.Stdin) {
			return errors.New("no task context: pass --context or pipe the Claude Code hook payload on stdin")
		}
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		context = hookContext(data)
	}
	if strings.TrimSpace(context) == "" {
		return nil
	}

	entries, err := ledger.Open(repo).Load()
	if err != nil {
		return err
	}
	fmt.Print(inject.Block(entries, context, time.Now()))
	return nil
}

// hookContext extracts the task context from a Claude Code hook payload:
// the prompt for UserPromptSubmit, the recent user messages for SessionStart.
// Anything unparseable degrades to raw text; anything that fails to yield
// context yields none — a hook must never break a session.
func hookContext(data []byte) string {
	text := strings.TrimSpace(string(data))
	if text == "" {
		return ""
	}
	var payload struct {
		Prompt         string `json:"prompt"`
		TranscriptPath string `json:"transcript_path"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return capRunes(text, maxTranscriptRunes) // plain text, not a hook payload
	}
	if p := strings.TrimSpace(payload.Prompt); p != "" {
		return p
	}
	if payload.TranscriptPath != "" {
		return transcriptContext(payload.TranscriptPath)
	}
	return ""
}

// transcriptContext pulls the last few user messages out of a Claude Code
// session transcript (JSONL). Best effort by design.
func transcriptContext(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	ring := make([]string, 0, maxTranscriptMessages)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var line struct {
			Type    string `json:"type"`
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(sc.Bytes(), &line) != nil || line.Type != "user" {
			continue
		}
		if text := userText(line.Message.Content); text != "" {
			ring = append(ring, capRunes(text, 2000))
			if len(ring) > maxTranscriptMessages {
				ring = ring[1:]
			}
		}
	}
	context := strings.Join(ring, "\n")
	return capRunes(context, maxTranscriptRunes)
}

// userText extracts plain text from a message content field, which Claude
// Code transcripts represent either as a string or as a block array.
func userText(content json.RawMessage) string {
	if len(content) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(content, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(content, &blocks); err != nil {
		return ""
	}
	var texts []string
	for _, b := range blocks {
		if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
			texts = append(texts, strings.TrimSpace(b.Text))
		}
	}
	return strings.Join(texts, "\n")
}

func capRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
