// Package match implements the deterministic similarity used at injection
// time: normalize (lowercase, tokenize, trim file suffixes), then a
// length-weighted overlap between the task context and each tombstone's
// scope tokens. No embeddings, no network, no model call — session-start
// injection must be instant, offline and free.
package match

import "strings"

// Threshold is the constant minimum scope coverage for a tombstone to count
// as a match. It is deliberately not configurable: the product's promise is
// that injection behaves the same on every machine.
const Threshold = 0.35

// fileSuffixes are trimmed from path-like words before tokenizing, so
// "internal/auth/token.go" and a prompt mentioning "internal/auth/token"
// produce the same tokens.
var fileSuffixes = []string{
	".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java", ".kt",
	".rb", ".php", ".c", ".h", ".cpp", ".cs", ".swift", ".md", ".json",
	".yaml", ".yml", ".toml",
}

// Tokenize normalizes free text or a scope entry into comparable tokens:
// lowercase, split on every non-alphanumeric rune, camelCase humps split,
// known file suffixes trimmed. "TestTokenRefresh" and "test token refresh"
// therefore meet on the same three tokens.
func Tokenize(s string) []string {
	words := strings.Fields(s)
	tokens := make([]string, 0, len(words)*2)
	for _, w := range words {
		w = trimFileSuffix(w)
		for _, part := range splitCamel(w) {
			tokens = append(tokens, alnum(part)...)
		}
	}
	return tokens
}

// Score returns the length-weighted coverage of the tombstone's scope tokens
// by the context token set: longer tokens (usually the distinctive ones, a
// symbol or file name) carry more weight than short generic ones. An empty
// scope never matches.
func Score(contextTokens map[string]struct{}, scope []string) float64 {
	total, matched := 0.0, 0.0
	seen := map[string]bool{}
	for _, entry := range scope {
		for _, tok := range Tokenize(entry) {
			if seen[tok] {
				continue
			}
			seen[tok] = true
			w := float64(len(tok))
			total += w
			if _, ok := contextTokens[tok]; ok {
				matched += w
			}
		}
	}
	if total == 0 {
		return 0
	}
	return matched / total
}

// Matches scores one tombstone scope against a context and reports whether
// it clears the Threshold.
func Matches(contextTokens map[string]struct{}, scope []string) (float64, bool) {
	s := Score(contextTokens, scope)
	return s, s >= Threshold
}

// ContextSet tokenizes a whole task context into a set for scoring.
func ContextSet(context string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, tok := range Tokenize(context) {
		set[tok] = struct{}{}
	}
	return set
}

func trimFileSuffix(w string) string {
	lower := strings.ToLower(w)
	for _, suf := range fileSuffixes {
		if strings.HasSuffix(lower, suf) && len(w) > len(suf) {
			return w[:len(w)-len(suf)]
		}
	}
	return w
}

// splitCamel inserts a boundary at lower→upper transitions and at
// acronym→word transitions, so TestTokenRefresh tokenizes as test / token /
// refresh and ParseJSONResponse as parse / json / response.
func splitCamel(w string) []string {
	runes := []rune(w)
	hasUpper := false
	for _, r := range runes {
		if r >= 'A' && r <= 'Z' {
			hasUpper = true
			break
		}
	}
	if !hasUpper {
		return []string{w}
	}
	var parts []string
	start := 0
	for i := 1; i < len(runes); i++ {
		prev, cur := runes[i-1], runes[i]
		boundary := prev >= 'a' && prev <= 'z' && cur >= 'A' && cur <= 'Z'
		if !boundary && prev >= 'A' && prev <= 'Z' && cur >= 'A' && cur <= 'Z' &&
			i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z' {
			boundary = true // end of an acronym run: ...ED|Case
		}
		if boundary {
			parts = append(parts, string(runes[start:i]))
			start = i
		}
	}
	return append(parts, string(runes[start:]))
}

// alnum lowercases and splits a word on non-alphanumeric runes, dropping
// empty pieces.
func alnum(w string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			out = append(out, b.String())
			b.Reset()
		}
	}
	for _, r := range strings.ToLower(w) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			flush()
		}
	}
	flush()
	return out
}
