# 演示夹具 — authservice 仓库的 before / after

这是 deadend-ledger 的演示夹具:`authservice` 是一个 token 刷新方案已经失败过两次的小仓库。同一份夹具跑两遍 —— 第一遍没有台账,第二遍有一枚墓碑。

> 说明:文中 agent 的对白是夹具脚本,用来展示注入机制的设计行为;所有 `deadend` 命令输出均为真实运行结果,可用 [examples/fixture-demo.sh](../../examples/fixture-demo.sh) 逐步复现。仓库内置的 `TestTokenRefresh` 会真实失败,所以证据定位符随时可以机器验证。

场景:上个会话里,agent 提出了「client-side token refresh」方案,`go test ./internal/auth -run TestTokenRefresh` 失败:

```text
--- FAIL: TestTokenRefresh (0.00s)
    token_test.go:11: refresh loop re-entered: token used to refresh itself,
        next expiry moved backwards. 2 attempts, 2 failures this session.
FAIL
FAIL	authservice/internal/auth	0.312s
FAIL
```

## Before — 台账为空,同一个坑再踩一遍

```text
$ claude
> 修一下 token refresh 循环,上次没修好。

Agent: Looking at the token refresh loop, the cleanest fix is a client-side
       refresh — the token can refresh itself before expiry...
       [writes internal/auth/token.go, runs the auth tests]

--- FAIL: TestTokenRefresh (0.00s)
    token_test.go:11: refresh loop re-entered: token used to refresh itself, ...

Agent: Let me try adjusting the refresh ordering...
```

没有台账,上次失败只剩开发者的记忆。agent 把同一方案再走一遍,烧掉同样的 token 与等待时间。

```text
$ ls .deadend 2>/dev/null || echo "(no ledger)"
(no ledger)
```

## After — 一枚墓碑,会话开局就知道走不通

先把失败写成墓碑(真实命令,真实输出):

```text
$ deadend record --approach "use client-side token refresh" \
    --scope internal/auth/token.go,TestTokenRefresh \
    --kind failing_test \
    --locator "go test ./internal/auth -run TestTokenRefresh" \
    --file token-fail.txt \
    --observed-at 2026-10-15 \
    --revive-when "the auth backend accepts idempotent refresh (tracked as AUTH-214)"

approach    use client-side token refresh
scope       internal/auth/token.go, TestTokenRefresh
evidence    failing_test | go test ./internal/auth -run TestTokenRefresh (observed 2026-10-15)
excerpt     --- FAIL: TestTokenRefresh (0.00s)
    token_test.go:11: refresh loop re-entered: token used to refresh itself,
        …
expires     never
revive when the auth backend accepts idempotent refresh (tracked as AUTH-214)

tombstone #3 recorded (2a8b91d0) — the next matching session starts with it
```

下一个会话,UserPromptSubmit hook 运行 `deadend inject`,任务描述与墓碑的 scope 命中,规划上下文开头出现这段(真实输出):

```text
<deadend-ledger active-tombstones>
The following approach(es) already failed in this repo. Do NOT retry them unless the revive condition now holds.

[tombstone #3] use client-side token refresh
  scope: internal/auth/token.go, TestTokenRefresh
  evidence: failing_test | go test ./internal/auth -run TestTokenRefresh (observed 2026-10-15)
  excerpt: --- FAIL: TestTokenRefresh (0.00s) token_test.go:11: refresh loop re-entered: token used to refresh itself, next expiry moved backwards. 2 attempts, 2 failures this session. FAIL FAIL authservice/internal/auth 0.312s FAIL
  do-not-retry unless: the auth backend accepts idempotent refresh (tracked as AUTH-214)
[tombstone #1] cache access tokens on disk at ~/.authcache
  scope: internal/auth/store.go
  evidence: error_span | internal/auth/store.go:88: open ~/.authcache: permission denied (observed 2026-10-16)
  excerpt: open ~/.authcache: permission denied — token cache write fails on locked-down CI runners
[tombstone #2] bump the JWT library to v9 to fix the refresh loop
  scope: go.mod, internal/auth
  evidence: build_log | go build ./... (v9 requires module bump of 3 downstream deps) (observed 2026-10-17)
  excerpt: github.com/golang-jwt/jwt/v9: version 9.0.0 requires go >= 1.25; 3 downstream deps pin go 1.24
</deadend-ledger>
```

于是:

```text
> 修一下 token refresh 循环,上次没修好。

Agent: skipped: tombstone #3 — client-side token refresh already failed
       TestTokenRefresh twice, do-not-retry unless the backend accepts
       idempotent refresh (AUTH-214). Planning a server-side rotation
       endpoint instead.
```

同一个任务,开局一秒之内,agent 已经知道哪条路走不通。

## 无关任务注入为空

不在 scope 内的任务,一个字也不注入(真实输出,`wc -c` 计数为 0):

```text
$ deadend inject --context "fix the flaky retry in the download queue" | wc -c
       0
```
