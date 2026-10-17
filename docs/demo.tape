Output assets/demo.gif

Set FontSize 14
Set Width 1200
Set Height 660
Set Padding 16
Set Theme Dracula
Set TypingSpeed 10ms

# Build the binary and put it on PATH for the session.
Type "go build -o /tmp/dd-demo/deadend . && export PATH=/tmp/dd-demo:$PATH"
Enter
Sleep 2s
Type "deadend --version"
Enter
Sleep 1s

# A repo where the client-side token-refresh approach already failed twice.
Type "mkdir -p /tmp/dd-demo/repo/internal/auth && cd /tmp/dd-demo/repo && git init -q ."
Enter
Sleep 1s

# init: create the ledger and merge the Claude Code hook lines (diff first).
Type "deadend init"
Enter
Sleep 3s

# record: interactive — approach, scope, evidence, revive condition.
Type "deadend record"
Enter
Sleep 1s
Type "use client-side token refresh"
Enter
Sleep 1s
Type "internal/auth/token.go,TestTokenRefresh"
Enter
Sleep 1s
Type ""
Enter
Sleep 1s
Type "go test ./internal/auth -run TestTokenRefresh"
Enter
Sleep 1s
Type "--- FAIL: TestTokenRefresh (0.00s) refresh loop re-entered"
Enter
Type "END"
Enter
Sleep 1s
Type ""
Enter
Sleep 1s
Type "the auth backend accepts idempotent refresh (AUTH-214)"
Enter
Sleep 1s
Type "y"
Enter
Sleep 2s

# show: every tombstone with its machine-checkable evidence.
Type "deadend show"
Enter
Sleep 3s

# inject: the matching task gets the tombstone block; an unrelated task gets zero bytes.
Type "deadend inject --context 'the token refresh still loops, try the client-side token refresh again'"
Enter
Sleep 3s
Type "deadend inject --context 'fix the flaky retry in the download queue' | wc -c"
Enter
Sleep 2s
Sleep 4s
