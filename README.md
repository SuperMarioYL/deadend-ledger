[English](./README.en.md) | **简体中文**

<picture>
  <source media="(max-width: 640px) and (prefers-color-scheme: dark)" srcset="assets/presentation/hero-mobile-dark.svg">
  <source media="(max-width: 640px)" srcset="assets/presentation/hero-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="assets/presentation/hero-dark.svg">
  <img src="assets/presentation/hero-light.svg" width="1000" alt="deadend-ledger — 把失败尝试写成带证据的墓碑,注入给编码智能体;右侧是三枚墓碑的台账脊柱。">
</picture>

<div align="center">
  <img src="https://readme-typing-svg.demolab.com?font=Noto+Sans+SC&weight=600&size=20&pause=1400&color=E656AF&center=true&vCenter=true&width=720&height=46&lines=%E6%8A%8A%E5%A4%B1%E8%B4%A5%E5%B0%9D%E8%AF%95%E5%86%99%E6%88%90%E5%B8%A6%E8%AF%81%E6%8D%AE%E7%9A%84%E5%A2%93%E7%A2%91%3B%E8%AE%A9%E7%BC%96%E7%A0%81%E6%99%BA%E8%83%BD%E4%BD%93%E5%BC%80%E5%B1%80%E5%B0%B1%E7%9F%A5%E9%81%93%E5%93%AA%E4%BA%9B%E8%B7%AF%E8%B5%B0%E4%B8%8D%E9%80%9A%3B%E4%B8%80%E4%B8%AA%E5%8F%AF%E6%8F%90%E4%BA%A4%E7%9A%84%20JSONL%20%E6%96%87%E4%BB%B6%20%C2%B7%20%E9%9B%B6%E5%AE%88%E6%8A%A4%E8%BF%9B%E7%A8%8B" alt="把失败尝试写成带证据的墓碑;让编码智能体开局就知道哪些路走不通;一个可提交的 JSONL 文件,零守护进程">
</div>

**每次开工,Claude Code 都把上一个会话已经失败的路再走一遍。deadend-ledger 把失败尝试写成带失败测试证据的墓碑,在相似任务开始时自动注入,让编程智能体第一眼就知道哪些路走不通。**

[![ci](https://img.shields.io/github/actions/workflow/status/SuperMarioYL/deadend-ledger/ci.yml?style=flat-square&label=ci)](https://github.com/SuperMarioYL/deadend-ledger/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/SuperMarioYL/deadend-ledger?style=flat-square)](https://github.com/SuperMarioYL/deadend-ledger/releases)
[![Go 1.24+](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white&style=flat-square)](https://go.dev)
[![Go Reference](https://pkg.go.dev/badge/github.com/SuperMarioYL/deadend-ledger.svg)](https://pkg.go.dev/github.com/SuperMarioYL/deadend-ledger)
[![license](https://img.shields.io/badge/license-MIT-e656af?style=flat-square)](LICENSE)

`v0.1.0` · [演示记录](docs/demo-results.json) · [十分钟上手](#快速开始)

## <img src="https://cdn.jsdelivr.net/npm/@tabler/icons@3.34.1/icons/outline/flag-x.svg" height="18" alt=""> 为什么使用

在同一仓库反复开编码智能体（coding agent）会话的人,都见过同一幕:上次会话刚证伪一个方案,这个会话它又作为「新想法」出现,烧掉同样的 token、测试时间和你的信任。散文式的会话记忆能**复述**失败 —— 一段「经验教训」、一条 gotcha —— 但它不在规划时**排除**任何选项:失败的方案和没试过的方案在模型眼里同样可行。

deadend-ledger 补上缺失的记录类型:**墓碑（tombstone）**。一枚墓碑记录一个已证伪的方案、它的作用域（文件、符号、包）、一份可机器验证的证据（失败测试、错误定位或构建日志）,以及一个可选的复活条件。会话开始时,hook 运行 `deadend inject`,把命中的墓碑渲染成固定模版放进规划上下文 —— 智能体第一眼看到的就是「这条路走不通,除非复活条件成立」。

台账只收负面结果:没有事实、没有偏好、没有散文记忆。这是它和通用 agent 记忆工具的根本分工。

## 架构

<picture>
  <source media="(max-width: 640px) and (prefers-color-scheme: dark)" srcset="assets/presentation/architecture-mobile-dark.svg">
  <source media="(max-width: 640px)" srcset="assets/presentation/architecture-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="assets/presentation/architecture-dark.svg">
  <img src="assets/presentation/architecture-light.svg" width="1000" alt="会话链路:Claude Code hooks 到 deadend inject、internal/match、规划上下文;记录链路:deadend record 到校验、追加 ledger.jsonl;inject 读台账,record 是唯一写入者。">
</picture>

一个二进制、一个文件、零守护进程。会话路径在每次 prompt 下运行,5 秒预算;记录路径只在开发者显式执行 `record` 时写入。台账是 append-only 的 JSONL,一行一枚墓碑,小到可以放心提交进 git —— 失败历史跟着仓库走,而不是跟着某个人走。

源码入口:[internal/ledger](internal/ledger/tombstone.go) · [internal/match](internal/match/similarity.go) · [internal/inject](internal/inject/render.go) · [internal/hooks](internal/hooks/claude.go) · [cmd](cmd/root.go)

匹配是确定性的:归一化（小写、拆驼峰、去文件后缀）之后,算任务上下文对墓碑 scope token 的长度加权覆盖率,常量阈值 0.35。没有 embedding、没有网络、没有模型调用 —— 代价是任务措辞漂移过大时会漏配,这是刻意取舍:用离线秒级的漏配,换掉每次会话开始时的模型调用。scope 键请用路径、符号、包名这类漂移小的写法。

## 安装

需要 Go 1.24+。

```bash
go install github.com/SuperMarioYL/deadend-ledger@latest
deadend --version
```

或者从 [Releases](https://github.com/SuperMarioYL/deadend-ledger/releases) 下载对应平台的静态二进制;或者从源码构建:

```bash
git clone https://github.com/SuperMarioYL/deadend-ledger.git
cd deadend-ledger && go build .
```

## 快速开始

下面每一步都可以在自带的夹具仓库上复现:`examples/fixture-demo.sh` 会构建 CLI、创建一个 token 刷新方案已经失败的 `authservice` 夹具,并完整跑一遍记录、查看与注入。

**1. 初始化仓库** —— 创建 `.deadend/ledger.jsonl`,并把 SessionStart 与 UserPromptSubmit 两行 hook 合入 `.claude/settings.json`(先展示 diff;已有配置原样保留):

```bash
cd your-repo
deadend init
```

**2. 智能体踩坑后,记录一枚墓碑** —— 直接跑 `deadend record` 会交互式提问:approach、scope(默认取本仓库改动的文件)、证据定位符、错误摘录(自动截到 400 字符)、可选的过期日期与复活条件。脚本化场景加 `--yes`(零提问、免确认),全量用 flags:

```bash
deadend record \
  --approach "use client-side token refresh" \
  --scope internal/auth/token.go,TestTokenRefresh \
  --kind failing_test \
  --locator "go test ./internal/auth -run TestTokenRefresh" \
  --file test-output.txt \
  --revive-when "the auth backend accepts idempotent refresh" \
  --yes
```

**3. 验证注入** —— 相似任务命中,无关任务零输出:

```bash
deadend inject --context "the token refresh still loops, try the client-side token refresh again"
# <deadend-ledger active-tombstones> ... </deadend-ledger>

deadend inject --context "fix the flaky retry in the download queue" | wc -c
# 0
```

**4. 把失败历史提交进仓库**(推荐):

```bash
git add .deadend/ledger.jsonl .claude/settings.json
git commit -m "record dead ends: client-side token refresh"
```

## 实际演示

<picture>
  <source media="(max-width: 640px) and (prefers-color-scheme: dark)" srcset="assets/presentation/process-mobile-dark.svg">
  <source media="(max-width: 640px)" srcset="assets/presentation/process-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="assets/presentation/process-dark.svg">
  <img src="assets/presentation/process-light.svg" width="1000" alt="四个阶段:记录死路、校验并追加、会话开始时评分、命中注入或保持沉默;结果:智能体跳过墓碑方案。">
</picture>

完整的前后对比 —— 没有台账时 agent 把失败方案再走一遍,有了墓碑后它打印 `skipped: tombstone #3` —— 见 [demo/fixture/before_after.md](demo/fixture/before_after.md)。其中 agent 对白是夹具脚本,`deadend` 命令输出均为真实运行结果。

下面是真实运行([examples/fixture-demo.sh](examples/fixture-demo.sh),完整记录见 [docs/demo-results.json](docs/demo-results.json)):相似任务命中三枚墓碑,按得分排序,#3(client-side token refresh)得分最高排第一:

```text
$ deadend inject --context "the token refresh still loops; try the client-side token refresh approach in internal/auth/token.go"
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

$ deadend inject --context "fix the flaky retry in the download queue" | wc -c
       0
```

终端实录(GIF 由 [docs/demo.tape](docs/demo.tape) 驱动真实二进制渲染):

![Terminal recording](assets/demo.gif) · [Recording script](docs/demo.tape)

## 命令

| 命令 | 作用 | 状态 |
| --- | --- | --- |
| `deadend init` | 创建台账,合入 Claude Code hook 行(先展示 diff) | v0.1 |
| `deadend record` | 记录一枚墓碑;flags 全量可用,终端里交互式提问 | v0.1 |
| `deadend show` | 渲染全部墓碑及其证据定位符 | v0.1 |
| `deadend inject` | 读取任务上下文,命中则打印注入块(hook 入口) | v0.1 |
| `deadend review` | 按状态、年龄、证据巡检台账 | 规划中(当前用 `show`) |
| `deadend revive <id>` | 复活条件成立时解除墓碑 | 规划中 |

注入块带硬上限(2400 字符):无论命中多少,都不会淹没规划上下文;放不下的按得分舍弃,并附一条 `deadend show` 提示。

## 台账格式

`.deadend/ledger.jsonl`,一行一枚墓碑,一次 append 一次完整 schema 校验。真实的一枚:

```json
{
  "id": "2a8b91d0",
  "approach": "use client-side token refresh",
  "scope": ["internal/auth/token.go", "TestTokenRefresh"],
  "evidence": {
    "kind": "failing_test",
    "locator": "go test ./internal/auth -run TestTokenRefresh",
    "excerpt": "--- FAIL: TestTokenRefresh (0.00s) ...",
    "observed_at": "2026-10-15"
  },
  "status": "active",
  "created_at": "2026-10-15",
  "revive_when": "the auth backend accepts idempotent refresh (tracked as AUTH-214)"
}
```

字段语义:`id` 由 approach + locator 哈希得出,同一失败重复记录会被拒绝;`scope` 是匹配键;`evidence.locator` 必须是可机器验证的命令或错误定位;`status` 取 `active` / `expired` / `revived`;`expires_at` 过期即停止注入(墓碑腐烂);`revive_when` 说明什么条件下这条路值得重试。JSONL 格式与具体 agent 无关 —— Claude Code 只是最先接入的会话环境。

## 与散文记忆的分工

会话记忆工具负责把会话压缩成可检索的散文;deadend-ledger 负责把已证伪的方案变成会被排除的选项。两类记录解决两类问题:

| | 散文式会话记忆 | deadend-ledger 墓碑 |
| --- | --- | --- |
| 记录类型 | 会话摘要、观察、偏好 | 只收负面结果(已证伪的方案) |
| 证据 | 描述性总结 | 机器可验证的 locator(测试命令、错误定位) |
| 注入语义 | 「这里有过一段经验」 | 「此路不通,除非复活条件成立」 |
| 生命周期 | 随摘要滚动 | `expires_at` 腐烂、`revive_when` 复活 |
| 匹配 | 语义检索 | 确定性 token 覆盖,常量阈值 |

同一仓库里两者可以并存:记忆负责「发生过什么」,台账负责「别再做什么」。

## <img src="https://cdn.jsdelivr.net/npm/@tabler/icons@3.34.1/icons/outline/coin.svg" height="18" alt=""> 付费

v0.1 是本地运行的开源 CLI,单人文档式的失败台账不收费。产品要解决的下一层问题在团队:**只有一个人看得见的台账,只修一个人的记忆** —— 墓碑无人巡检地腐烂,没有跨仓库的视图,也没有机制阻止别的 PR 重走死路。

因此商业方向是**托管团队层**(在开源 CLI 验证之后上线,本仓库的 v0.1 不含任何服务端):

| | 开源 CLI(v0.1) | 托管团队层(规划) |
| --- | --- | --- |
| 形态 | 本地二进制 + 仓库内 JSONL | 托管服务 |
| 范围 | 单仓库、单人 | 跨仓库墓碑同步 |
| 巡检 | `deadend show` | review / revive 队列 |
| CI | — | PR 试图重走活跃墓碑时自动评论 |

价格:$19/席/月,≤3 席免费,年付赠 2 个月;国际结算走 Stripe,国内自助渠道(爱发电/面包多)随后跟上。

如果你的团队(3–10 人、多个共享仓库、每天都在跑 Claude Code)想上团队层,在 [Issues](https://github.com/SuperMarioYL/deadend-ledger/issues) 里留下团队规模和仓库数量,我们会安排一次 15 分钟的演示。

## 路线图

- **v0.1(已实现)**:墓碑记录、schema 校验、append-only 台账、确定性匹配、注入渲染、Claude Code hook 安装器。
- **下一步**:`review` / `revive` 的状态流转(过期扫描、复活条件检查),补齐墓碑生命周期的操作面。
- **之后**:更多 agent 集成(台账格式与 agent 无关,期待 Cursor、Codex 等会话环境接入)、托管团队层(见[付费](#付费))。

## 局限

- **确定性匹配会漏**:任务措辞与 scope 键漂移过大时,墓碑不会注入。这是刻意的:离线、免费、秒级,好过每次会话开始时的模型调用。写好 scope 键(路径、符号、包名)能显著减少漏配。
- **匹配只看 scope**:approach 文本不参与打分;命中质量取决于 scope 键的质量。
- **只接 Claude Code**:hook 安装器只写 Claude Code 的 settings;其他集成是后续版本的事。
- **review / revive 未实现**:`show` 是当前的查看面;过期墓碑按日期自动停止注入,但状态流转要等 `review` / `revive` 落地。

## 许可证

[MIT](LICENSE)

<p align="center"><sub><a href="./LICENSE">MIT</a> © 2026 SuperMarioYL</sub></p>
