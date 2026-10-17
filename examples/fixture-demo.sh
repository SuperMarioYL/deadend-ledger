#!/usr/bin/env bash
# Reproduces docs/demo-results.json on a throwaway fixture repo:
# builds the CLI, creates the authservice fixture (the repo where a
# client-side token-refresh refactor already failed), records three
# tombstones, lists them, and injects for a matching and an unrelated task.
# Dates are pinned so the output is byte-stable across runs.
set -euo pipefail
cd "$(dirname "$0")/.."

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
BIN="$WORK/deadend"
go build -o "$BIN" .

FIXTURE="$WORK/authservice"
mkdir -p "$FIXTURE/internal/auth"
cd "$FIXTURE"
git init -q .

cat > go.mod <<'EOF'
module authservice

go 1.24
EOF

cat > internal/auth/token.go <<'EOF'
package auth

// TokenRefresher tried to refresh access tokens client-side. It broke the
// refresh loop twice across two sessions; the approach is tombstoned.
type TokenRefresher struct{}

func (TokenRefresher) Refresh() error {
	return errRefreshBroken
}
EOF

cat > internal/auth/token_test.go <<'EOF'
package auth

import "testing"

// TestTokenRefresh is the evidence locator recorded in the ledger: the
// client-side refresh approach fails this test by construction.
func TestTokenRefresh(t *testing.T) {
	r := TokenRefresher{}
	if err := r.Refresh(); err == nil {
		t.Fatal("expected the client-side refresh to fail here")
	}
}
EOF

cat > internal/auth/store.go <<'EOF'
package auth

import "errors"

var errRefreshBroken = errors.New("client-side token refresh breaks the loop: internal/auth/store.go:88: open ~/.authcache: permission denied")
EOF

cat > "$WORK/token-fail.txt" <<'EOF'
--- FAIL: TestTokenRefresh (0.00s)
    token_test.go:11: refresh loop re-entered: token used to refresh itself,
        next expiry moved backwards. 2 attempts, 2 failures this session.
FAIL
FAIL	authservice/internal/auth	0.312s
FAIL
EOF

echo '$ deadend init'
"$BIN" init
echo
echo '$ deadend record --approach "cache access tokens on disk at ~/.authcache" ...'
"$BIN" record \
  --approach "cache access tokens on disk at ~/.authcache" \
  --scope internal/auth/store.go \
  --kind error_span \
  --locator "internal/auth/store.go:88: open ~/.authcache: permission denied" \
  --excerpt "open ~/.authcache: permission denied — token cache write fails on locked-down CI runners" \
  --observed-at 2026-10-16 \
  --yes
echo
echo '$ deadend record --approach "bump the JWT library to v9 to fix the refresh loop" ...'
"$BIN" record \
  --approach "bump the JWT library to v9 to fix the refresh loop" \
  --scope go.mod,internal/auth \
  --kind build_log \
  --locator "go build ./... (v9 requires module bump of 3 downstream deps)" \
  --excerpt "github.com/golang-jwt/jwt/v9: version 9.0.0 requires go >= 1.25; 3 downstream deps pin go 1.24" \
  --observed-at 2026-10-17 \
  --yes
echo
echo '$ deadend record --approach "use client-side token refresh" --scope internal/auth/token.go,TestTokenRefresh --kind failing_test --locator "go test ./internal/auth -run TestTokenRefresh" --file token-fail.txt --observed-at 2026-10-15 --revive-when "..."'
"$BIN" record \
  --approach "use client-side token refresh" \
  --scope internal/auth/token.go,TestTokenRefresh \
  --kind failing_test \
  --locator "go test ./internal/auth -run TestTokenRefresh" \
  --file "$WORK/token-fail.txt" \
  --observed-at 2026-10-15 \
  --revive-when "the auth backend accepts idempotent refresh (tracked as AUTH-214)" \
  --yes
echo
echo '$ deadend show'
"$BIN" show
echo
echo '$ deadend inject --context "the token refresh still loops; try the client-side token refresh approach in internal/auth/token.go"'
"$BIN" inject --context "the token refresh still loops; try the client-side token refresh approach in internal/auth/token.go"
echo
echo '$ deadend inject --context "fix the flaky retry in the download queue" | wc -c'
"$BIN" inject --context "fix the flaky retry in the download queue" | wc -c
