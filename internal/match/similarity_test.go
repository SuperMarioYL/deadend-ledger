package match

import (
	"testing"
)

func TestTokenizePathsAndSymbols(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"internal/auth/token.go", []string{"internal", "auth", "token"}},
		{"TestTokenRefresh", []string{"test", "token", "refresh"}},
		{"use client-side token refresh", []string{"use", "client", "side", "token", "refresh"}},
		{"token_test.go", []string{"token", "test"}},
		{"  MIXEDCase_words  ", []string{"mixed", "case", "words"}},
		{"retry v2!", []string{"retry", "v2"}},
		{"", nil},
	}
	for _, c := range cases {
		got := Tokenize(c.in)
		if len(got) != len(c.want) {
			t.Fatalf("Tokenize(%q) = %v, want %v", c.in, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("Tokenize(%q) = %v, want %v", c.in, got, c.want)
			}
		}
	}
}

func contextOf(s string) map[string]struct{} { return ContextSet(s) }

func TestScoreMatchingContextClearsThreshold(t *testing.T) {
	scope := []string{"internal/auth/token.go", "TestTokenRefresh"}
	// unique scope tokens: internal(8) auth(4) token(5) test(4) refresh(7), total 28.
	// prompt matches token+refresh = 12/28 ≈ 0.43.
	score, ok := Matches(contextOf("rewrite the client-side token refresh flow"), scope)
	if !ok {
		t.Fatalf("expected match, score = %.2f", score)
	}
	if score < Threshold {
		t.Fatalf("score %.2f below threshold", score)
	}
}

func TestScoreUnrelatedContextDoesNotMatch(t *testing.T) {
	scope := []string{"internal/auth/token.go", "TestTokenRefresh"}
	for _, prompt := range []string{
		"fix the flaky retry logic in the download queue",
		"bump the cli dependency version",
		"",
	} {
		if score, ok := Matches(contextOf(prompt), scope); ok {
			t.Fatalf("prompt %q matched with score %.2f, want no match", prompt, score)
		}
	}
}

func TestScoreSingleGenericWordDoesNotMatch(t *testing.T) {
	// auth alone = 4/28 ≈ 0.14 — one shared word must not inject a tombstone.
	if _, ok := Matches(contextOf("auth"), []string{"internal/auth/token.go", "TestTokenRefresh"}); ok {
		t.Fatal("single generic word matched")
	}
}

func TestScoreIsLengthWeighted(t *testing.T) {
	// Matching the distinctive long token must outweigh a short one.
	ctx := contextOf("tokenrefresh")
	if Score(ctx, []string{"tokenrefresh"}) != 1.0 {
		t.Fatal("full coverage should score 1.0")
	}
	if got := Score(ctx, []string{"tokenrefresh", "go"}); got <= 0.5 {
		t.Fatalf("long token should dominate the weighted score, got %.2f", got)
	}
}

func TestScoreEmptyScopeNeverMatches(t *testing.T) {
	if _, ok := Matches(contextOf("anything at all"), nil); ok {
		t.Fatal("empty scope matched")
	}
}

func TestThresholdBoundary(t *testing.T) {
	// scope: auth(4) token(5) refresh(7) = 16; matching auth+token = 9/16 = 0.5625.
	scope := []string{"auth", "token", "refresh"}
	if s, ok := Matches(contextOf("token auth"), scope); !ok || s != 9.0/16.0 {
		t.Fatalf("score = %.4f ok = %v, want %.4f true", s, ok, 9.0/16.0)
	}
	// matching refresh only = 7/16 = 0.4375 — still above 0.35.
	if _, ok := Matches(contextOf("refresh"), scope); !ok {
		t.Fatal("0.4375 should clear the 0.35 threshold")
	}
	// matching token only = 5/16 = 0.3125 — below threshold.
	if _, ok := Matches(contextOf("token"), scope); ok {
		t.Fatal("0.3125 should stay below the 0.35 threshold")
	}
}
