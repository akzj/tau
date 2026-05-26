# Phase 3 Audit (A1) — docs/01, 02, 05

> Audit 标尺：phase2-tau-design.md (606 行) + handoff §1 C1-C10 + R1-R6 + P1+P2。
> 反 anchoring 三条已读。Phase 1 源码反驳力当资产用——发现 docs 描述与 pi 真实不符直接标 ❌。
> Q-First 与 §5 self-doubt 沿用。

---

## === doc-01-agent-loop ===

### ✅ 保留项 (与 phase2 一致)

- **[doc 01 §How it works 行 5-9] "loop is purely functional over a Context, driven by an event sink rather than a return value"**
  匹配 phase2 §2.1 "Loop 无字段，状态全在 Session" + 双层 stream（Loop 不返回 message list，事件流暴露）。一致。
  匹配 C6 / R4。

- **[doc 01 §Termination model 行 32-39]** 多源退出（`stopReason error/aborted` / 无 tool calls+queues / `shouldStopAfterTurn` / 全批 terminate）
  匹配 phase2 §2.6 Tool 注释 "terminate is hint, 全批 terminate 才真停" + §2.5 TurnEnd Reason。
  匹配 A1 §3 我自己 Phase 1 验证过。

- **[doc 01 §Termination 行 39] "No token-budget cap. No max-turn cap."**
  与 phase2 兼容（phase2 没设 cap）。**这是一条值得显式保留的 design philosophy**——budget 在 hook 层，loop 保持纯——可考虑抽到 phase2 §1 "5 条核心断言"加第 6 条或 §2.1 design note。
  匹配 C7（Loop 不知 budget）+ R5（Loop 不知具体 limit 规则）。

- **[doc 01 §Streaming 行 73-83] StreamFn 严格契约 "Failures must be encoded in the stream as a final error event ... never thrown. Errors are values."**
  匹配 phase2 §2.4 Provider StreamRequest + §2.5 ErrorEvent。
  匹配 R1（错误是 typed value，类型=运行时真相）。

- **[doc 01 §What's elegant 行 109-112] "two-queue steering model: steering messages inject between, follow-up after"**
  与 phase2 G3-style 推迟一致（phase2 §2.1 divergence-from-pi: "steer/followUp 推迟到 Phase 5+ 验证"）。doc 描述的是 pi 现实，phase2 推迟决策——**不冲突**。doc 的描述给 Phase 5+ 重审提供了基线材料。

### ⚠️ 冲突项 (与 phase2 矛盾)

- **[doc 01 §Tool-result feedback 行 41-45] "appended directly to the live `currentContext.messages` array"**
  ⚠️ + ❌ 双重问题。这条**既是 docs 错误描述 pi**（见下面 ❌ 项），也是与 phase2 §2.3 "Loop 不维护 mirror" 在描述层面的冲突。
  phase2 §2.3 主张：Transcript 是单一真相源，Loop 不持有内部 mirror。
  **裁决：docs 错（双重错——pi 描述错+与 phase2 不一致；phase2 是对的）**
  理由 (≤3 行)：runAgentLoop:104-107 实际做 spread copy，loop 内的 mutation 是在内部副本上，不是 caller 的 currentContext.messages（我 Phase 1 §3.2 self-correction 已证）。phase2 §2.3 沿用了正确观察。

- **[doc 01 §Agent ↔ Harness contract 行 50-66] callbacks 平铺：`convertToLlm` / `transformContext` / `getApiKey` / `beforeToolCall` / `afterToolCall` / `prepareNextTurn` / `shouldStopAfterTurn` / `getSteeringMessages` / `getFollowUpMessages`**
  phase2 §2.8 主张 Hook 用双 generic interface (Chain[T] / LastWins[T])，**不允许平铺 func 字段**。
  doc 这里直接复述 pi 的 `AgentLoopConfig` 平铺布局——pi 反例正是 phase2 C5 反对的。
  **裁决：docs 错（pi 偏见——把 pi 的 callback bag 当通用契约写）**
  理由：pi 的平铺 callback bag 在 Phase 1 已批为反 C5 模式（last-wins 与 chain 混在一个 API）。phase2 §2.8 已显式拆解，编译期可拦。

- **[doc 01 §Go sketch 行 87-104] `Hooks struct` 平铺 7 个 func 字段**
  phase2 §2.8 主张 `HookSet` 用 `Chain[T]` 和 `LastWins[T]` 各 generic interface，编译期区分语义。
  **裁决：docs 错（pi 偏见——直接把 TS callback bag 翻成 Go func 字段，丢失 phase2 D5 lesson）**
  理由：doc 的 `Hooks struct` 把 PrepareNextTurn / ShouldStopAfterTurn 平铺成 func 字段——签名上看不出哪个是 chain 哪个是 last-wins。命中 D5（pi 反例）+ 违反 C5（phase2 硬约束）。如 phase4 PRODUCE 阶段照此 sketch 实现 = 系统性返工。

- **[doc 01 §What's elegant 行 116] "Live-context mutation: tool results land in `currentContext.messages` immediately. No 'build the next request payload' step."**
  与 phase2 §2.3 C6 直接冲突，且**事实层面错**（同上 §Tool-result feedback）。
  **裁决：docs 错（双重）**
  理由：runAgentLoop:104-107 spread copy。这是把 pi 内部 mirror 当 feature 描述——正是我 Phase 1 §3.2 修正后版本的 anti-pattern "loop-internal-mirror-is-invisible-exhaust"。

### ➕ 补充项 (docs 有 phase2 没考虑的角度)

- **[doc 01 §Termination model 行 39] "No token-budget cap. No max-turn cap. The loop runs until something says stop. ... budget enforcement lives in hooks"**
  phase2 没明说"core 不做 budget 拦截"——这是 design philosophy 层级的隐含决策。
  phase2 没覆盖原因 (我推测)：phase2 §1 "Tau is not"段隐含了"无 framework 决策"，但具体到"无 turn cap / 无 token cap"没明说。
  应否补到 phase2 (我倾向)：**应**——补到 phase2 §1 "5 条核心断言"或 §2.1 Loop 注释作为 explicit design note。理由：phase4 builder 容易在"防呆"驱动下加 max turn cap "为了安全"，这种隐式决策不显式化容易漂移。

- **[doc 01 §Streaming 行 73-83] "Failures must be encoded in the stream as a final error event with stopReason error/aborted ... never thrown"**
  phase2 §2.4 Provider StreamRequest 有 ErrorEvent 但**没明说"provider 实现的契约"**——pi 这里的"never throw, errors are values"是给 provider 实现者的硬约束。
  phase2 没覆盖原因 (我推测)：phase2 把 Provider 当 interface 定，没规定 implementation contract。
  应否补到 phase2 (我倾向)：**应**——补到 phase2 §2.4 Provider 注释。理由：R1 类型=运行时真相要求"Stream 返回 channel 里就一定是事件"——provider 偷偷 throw 会破坏这个契约。Go 里如果没显式声明 "Stream 必须把 error encode 为 event 不能直接 return error from goroutine"，运行时 panic 会破坏 stream 消费方。这是 R1 的子约束，不显式化容易漂移。

- **[doc 01 §What's elegant 行 117-118] "`terminate` only honored when ALL tools agree: prevents one tool from forcing exit when others want to continue"**
  phase2 §2.6 Tool 注释里有 "terminate 是 hint，全批 terminate 才真停"——已覆盖。但 doc 这里给出**反例 rationale**（防 single-tool 暴政）——比 phase2 的"全批"更有教育意义。
  phase2 没覆盖原因：phase2 是契约层只说 what，不说 why。
  应否补到 phase2 (我倾向)：**否**——phase2 §2.6 已有契约描述足够；rationale 可放 ADR/PHILOSOPHY.md 而非主稿。

### ❌ 弃用项 (明显 pi 偏见)

- **[doc 01 §How it works 行 11-30] "double `while`" 描述 + 9 步 inner loop / 3 步 outer loop**
  pi 偏见类型：**TS-specific + pi 实现细节**。
  doc 把 pi 的具体 control-flow（while-while 嵌套、`hasMoreToolCalls` flag、`pendingMessages` array）当作"how loop works" 描述——但这是 pi 的具体实现选择，不是必然形态。Go 用 channel select 表达同样语义（loop 等待 provider event / tool result / cancel / queue drain）形态会非常不同。phase2 §2.1 Loop interface 是契约层，不规定内部 control flow——**这是对的**。
  phase4 builder 不应被 doc 的 while-while 框架影响。

- **[doc 01 §Tool-result feedback 行 41-45] 整段**
  pi 偏见类型：**类名延续 + 错误描述 pi**。doc 把"live mutation"当 feature——这是基于错误的 pi 阅读（实际 spread copy）。
  双错（pi 描述错+phase2 设计错位），全段废。

- **[doc 01 §TS-specific (skip in Go) 行 119-121] `declare module` for extending CustomAgentMessages → sealed union in Go**
  这条**自身合理**（doc 已认领是 TS-specific），但要注意：phase2 §2.5 AgentEvent 用 sealed interface (`eventMarker()` 私有方法)。doc 的 sealed union 建议给 message types 用——与 phase2 兼容。**不弃用，标作 phase4 PRODUCE 实施时参考**。

---

## === doc-02-harness ===

### ✅ 保留项 (与 phase2 一致)

- **[doc 02 §2.1 整段, 行 11-26] Append-only session tree, every entry `{id, parentId, timestamp}`，10 种 SessionTreeEntry 类型**
  匹配 phase2 §2.3 Transcript "Position" + Append/Slice/Subscribe 三方法。
  **但 phase2 §2.3 没列具体 entry 类型 union**——见 ➕ 补充项。

- **[doc 02 §2.2 行 30] phase machine `idle|turn|compaction|branch_summary|retry`**
  ⚠️ 中度冲突——见 ⚠️ 项。

- **[doc 02 §2.3 行 50-52] "All mutations are appendFile calls — no rewrites, no fsync gymnastics, no locks"**
  匹配 R4 / C1（Registry scoped 的实现侧推论：append-only = 无锁 = 自然多 session 安全）。
  与 phase2 §2.2 Session 相容。

- **[doc 02 §2.3 行 56] "`setLeafId(id)` doesn't mutate state directly — it appends a `LeafEntry`. Navigation is itself an event."**
  匹配 phase2 §2.3 Transcript "Append/Slice/Subscribe" + R1 类型=运行时真相（leaf 是 entry 不是隐式状态）。
  **优秀的 design pattern，phase2 §2.3 没显式说但 Transcript 接口是兼容的**。

- **[doc 02 §2.3 行 47] "IDs are 8-char prefixes of UUIDv7 with collision retry"**
  匹配 phase2 §2.2 SessionID + R4。无冲突。

- **[doc 02 §2.6 行 105-115] systemPrompt 可以是 string OR async function；formatSkillsForSystemPrompt 拼装**
  匹配 phase2 §2.2 `SystemPromptFn func(sess *Session) (string, error)` + §2.9 Skills 推 Phase 5+。
  doc 描述 pi 现实，phase2 选 fn 形态——一致。

### ⚠️ 冲突项 (与 phase2 矛盾)

- **[doc 02 §2.2 行 30] phase machine 5 状态作为"prevents concurrent compaction-during-turn or fork-during-compaction without locks"**
  phase2 §2.2 Session 没有 phase 字段——主张"compaction 已推产品层（G4 砍 hook）故 core 不需要"。
  **裁决：真两难——证据层面 phase machine 在 pi 是真存在的防御，但 G4 砍后 phase2 可能漏掉一个真问题**。
  理由 (≤3 行)：grep 实证 — pi 在 4 入口（prompt/skill/promptFromTemplate/compact/navigateTree）都断言 `phase === "idle"`，否则 throw `"busy"`；但**test/ 里 grep `"busy"` 0 命中**——0 测试覆盖。是防御性设计，非 evidence-driven race 防护。结合 G4 砍 compaction（已成 phase2 决策）→ phase machine 4 状态 collapse 到只剩 `idle | turn`，其实就是 `Run.IsActive()` 一个 bool 字段。
  **倾向**：不在 phase2 §2.2 加 enum 字段，但**应在 phase2 §2.1 Run 接口加 `IsActive() bool`** 或类似 guard——这样产品层重启 prompt 时能检测"已经在跑了"。这是 G4 砍后的副产品 cleanup。
  **escalate to coord-audit**：phase2 §2.2 Session 是否需要加这个 guard（trade-off：增加一个字段+contract 复杂度 vs 防止"双 prompt 并发触发未定义行为"）—— coord-audit 裁决。

- **[doc 02 §2.4 整段 行 60-103] Compaction 作为 first-class harness 功能**
  phase2 G4 已砍 `session_before_compact` hook，整章 compaction 在 phase2 中**推产品层**（用 `Provider.Complete + Transcript` 自己做）。
  **裁决：真两难，且 G4 evidence trigger 已激活——见 §G4-evidence 子节**
  理由 (≤3 行)：phase2 G4 砍 hook 的 stated 理由是"已知用户 0"。我 grep 证实：**coding-agent 在 agent-session.ts:1665, 1938 两处真实消费 `extensionCompaction` 路径**（手动 compact + auto-compact 各一处），有完整 if-else 分支实现。**G4 "known users 0" 主张需要复审**。
  **倾向**：**反向挑战 phase2** — G4 砍的决策证据基础不足。但承认：coding-agent 的消费可能只是"contract 完整性"而非"真有 alternative 实现需求"——需要 coord-audit 决定是否升级 G4 二审。

  **G4 evidence trigger 量化**：
  - hook 类型定义 1 处（pi-agent-core）
  - hook 调用点 1 处（pi-agent-core）
  - hook 消费分支 2 处（coding-agent，real production code 不是测试）
  - hook 测试 4 处（pi-coding-agent test/）
  - "便宜 model 做 compaction"具体引用：**未找到** —— coord-audit 提醒的"≥1 具体引用 with 行号"**未达成**
  - 总评：**现实有 contract 消费（值 ≥0），但"alternative compaction 实现"的真实需求 case 0**。phase2 G4 "已知用户 0" 主张**严格不准确**（消费分支存在），但 G4 决策**实质方向可能仍对**（消费分支只是 contract 完整性，不是真有 cheap-model compaction）。
  - **建议**：coord-audit 升级 G4 二审，让 owner 评判"contract 消费 ≥ 1 但实质需求 = 0" 是否触发 G4 回滚。

- **[doc 02 §2.7 行 137-150] Hook bus 整段**
  doc 把 pi 的 `before_provider_request` (chain) 与 `tool_call` (last-wins-ish) **并列到一张表**，并称为 "chain-of-responsibility pattern at every meaningful boundary" "the last non-undefined return wins for hooks that produce a single result"。
  这正是 phase2 D5 / C5 反对的 anti-pattern——把 last-wins 与 chain 混进一个 API。
  **裁决：docs 错（pi 偏见——把 anti-pattern 当 elegant 写）**
  理由：phase2 §2.8 拆 Chain[T] / LastWins[T] 双 generic interface，编译期可拦混用。doc 的"unified hook bus"是把 pi 反例直接当指南——若 phase4 builder 照搬 = 重新犯 pi D5。

- **[doc 02 §2.8 Go sketch 行 178] `On(event string, handler HookHandler) func() // returns unsubscribe`**
  phase2 §2.8 用 typed `HookSet { BeforeProviderRequest LastWins[StreamRequest]; TransformContext Chain[[]Message]; ... }` —— **不是** `On(string, handler)` 这种字符串-key API。
  **裁决：docs 错（pi 偏见——TS event-emitter 模式直译，丢失 type safety）**
  理由：`On(event string, handler HookHandler)` 把 event type 退化为字符串 key，typo 编译期不报错；handler 是 `HookHandler` 单一类型（要么 union 要么 `any`），违反 R1。phase2 §2.8 typed field 是正解。

### ➕ 补充项 (docs 有 phase2 没考虑的角度)

- **[doc 02 §2.1 行 12-25] 10 种 SessionTreeEntry 类型完整 union**
  phase2 §2.3 Transcript 接口设计为 Append/Slice/Subscribe 三方法，但**没列出 entry 具体 type union**。
  phase2 没覆盖原因 (我推测)：phase2 把 entry 类型当作产品层决策（ModelChange/ThinkingLevelChange/CompactionEntry/BranchSummaryEntry/CustomEntry/CustomMessageEntry/LabelEntry/SessionInfoEntry/LeafEntry——其中后 4 种是产品层 metadata；前 4 种是 core 概念但 phase2 G4 已砍 compaction）。
  应否补到 phase2 (我倾向)：**部分应**——core 至少需要 Message 一种 entry。其他（LeafEntry / ModelChange / ThinkingLevelChange）属产品层。phase2 §2.3 应明说"Transcript stores Message entries; product layer extends via custom entry types via Subscribe consumer"。**核心是把 entry 类型扩展性写显式**。

- **[doc 02 §2.3 行 64-66] SessionRepo.fork(source, {entryId, position}) 整段**
  phase2 §2.2 没说 fork/copy 操作。
  phase2 没覆盖原因 (我推测)：fork 是产品层用例（用户切分支体验），core 不需要。
  应否补到 phase2 (我倾向)：**否——产品层**。fork 是产品层"重启对话"的体验，core 提供 Transcript.Slice 已足够（产品层自己开新 Session + Append 副本）。

- **[doc 02 §2.6 行 116-120] Skills 与 prompt templates 加载（YAML frontmatter SKILL.md / `.md` files / `$1, $@, $ARGUMENTS` substitution）**
  phase2 §2.9 把 Skills 推 Phase 5+，明说"产品层在 closure 里 loadSkills + formatSkillsForPrompt 拼到 system prompt"。
  应否补到 phase2 (我倾向)：**否——已显式推 Phase 5+**。

### ❌ 弃用项 (明显 pi 偏见)

- **[doc 02 §2.4 整段 §2.5 整段] Compaction + Truncation 作为 harness first-class 功能**
  pi 偏见类型：**包级 mutable + 类名延续**。
  doc 把 pi 的 `compaction/compaction.ts` (755 LOC) 和 `harness/utils/truncate.ts` (344 LOC) 当作 harness 必备组件描述。phase2 G4 砍 compaction hook（争议见 ⚠️），但**整套 compaction 算法移到产品层是 phase2 决策**——doc 的"this is the most reusable piece of pi"在 tau 不成立。**Truncation** (344 LOC) 是 UX 决策（"超长 tool output 截断+保存全文到 temp"）—— 100% 产品层。
  全段标产品层弃用（除了 G4 evidence 触发部分）。

- **[doc 02 §2.7 整段] Hook bus 与"chain-of-responsibility pattern at every meaningful boundary"赞美**
  pi 偏见类型：**把 anti-pattern 当 elegant**。
  整段把 pi 的 last-wins/chain 混合 API 当作 design pattern 描述。phase2 §2.8 已修正——这段不能保留，否则会 anchor phase4 builder。

- **[doc 02 §2.8 Go sketch] HookBus + `On(event string, handler HookHandler)` 整段**
  pi 偏见类型：**TS event-emitter 模式直译**。废，使用 phase2 §2.8 typed Chain/LastWins。

- **[doc 02 §2.2 行 30 phase enum 中的 "retry"]**
  pi 偏见类型：**dead state delivery as live**。
  我 Phase 1 §3.6 已证 `"retry"` 在 pi 源码中**0 处赋值**（grep 完整 src+test+docs 仅 type 定义本身和 docs 复述）。doc 把它当 live 状态写——废。

### G4-evidence 子节（coord-audit 提醒的 evidence-search trigger）

| 检查项 | 结果 |
|---|---|
| `session_before_compact` hook 类型定义 | 1 处 (pi-agent-core/types.ts:569, 695) |
| hook emit 调用点 | 1 处 (pi-agent-core/agent-harness.ts:697) |
| 真实生产代码 consumer | **2 处** (pi-coding-agent/agent-session.ts:1665, 1938) — 完整 if-else 实现 |
| 测试 consumer | 4 处 (pi-coding-agent/test/) |
| "便宜 model 做 compaction" 具体引用 | **0 处** — coord-audit 提醒的"≥1 具体引用 with 行号"未达成 |
| `haiku/cheap/mini/gpt-3.5` near compaction | grep 命中只在 cli/args.ts 例子说明，不在 compaction context |

**结论**：G4 砍的决策**部分不准确**——"已知用户 0" 主张被 coding-agent 消费分支证伪。但**实质需求未证实**（无"cheap model do compaction"案例）。**建议 coord-audit 升级 G4 二审**，让 owner 评判"contract 完整性消费 vs 真实 alternative 实现需求"。

---

## === doc-05-multi-session-safety ===

### ✅ 保留项 (与 phase2 一致)

- **[doc 05 §1 整段 行 17-25] file-per-session, 不同并发 pi 落不同文件，filename = `<timestamp>_<uuid>.jsonl`**
  匹配 phase2 §5.4 多 Session 并发 + R4 + C1。doc 描述 pi 实现侧策略，phase2 §2.2 Session 是"per-session mutable state container"，**没规定存储**——但兼容（产品层选 file-per-session 即可）。

- **[doc 05 §2 行 33-37] "Append-only writes via appendFile ... atomic on POSIX up to PIPE_BUF"**
  匹配 phase2 §2.3 Transcript Append 语义。doc 给的 implementation rationale 强（POSIX 原子性约束）。phase2 §2.3 没明说但兼容。

- **[doc 05 §3 行 41-43] UUIDv7 + per-cwd 目录分区**
  匹配 phase2 §2.2 SessionID + R4。

- **[doc 05 §"Use the same model" 行 73-76] "Per-session append-only JSONL file ... UUIDv7-keyed ... cwd-partitioned ... append via os.OpenFile + Write"**
  匹配 phase2 §2.2 Session 实现侧 hint，与 R4 / C1 兼容。**很好的 Go-port 实施 hint**——不是冲突，是 phase4 PRODUCE 阶段的实施指南。

- **[doc 05 §"Do NOT reach for flock" 行 78-83] "flock is a footgun: NFS / Windows / 不解决问题"**
  匹配 phase2 §2.2 Session "禁包级 mutable" + R4 隐含。**值得保留作为 phase4 builder 的 cautionary note**。

- **[doc 05 §"Lock-free ≠ conflict-free" 行 95-100] "two sessions writing same source file ... two sessions running git ... CPU/disk competition"**
  与 phase2 §5.4 兼容（"不同 session 互不可见；不需要锁；不需要切 session 操作"）。doc 这里给的 caveat 也是产品层范畴。

### ⚠️ 冲突项

无 — doc 05 整体与 phase2 兼容，主要是 implementation hint 层级。

### ➕ 补充项

- **[doc 05 §"Encode the git rules INSIDE the tools" 行 85-92]** Go 实现示例，把 git 命令 ban 编码进 tool 而非系统 prompt
  phase2 没覆盖原因：phase2 §1 明说"不含产品层本体"——bash tool 是产品层 (coding-agent)。
  应否补到 phase2 (我倾向)：**否——产品层**。phase2 §5.1 接入点已暴露 BeforeToolCall hook，产品层用此 hook 实现 git ban 即可。

- **[doc 05 §"Caveats" 行 102-106] "Multi-session 'safety' relies on convention for git ... Make this code in the Go port"**
  这是产品层 safety advice，与 phase2 范围无关。

### ❌ 弃用项

无明显 pi 偏见。doc 05 整体是关于 multi-session 的实施侧 advice，没有 pi-specific 类名延续或 TS 模式直译。

**总结 doc 05**：与 phase2 几乎完全对齐，scope 错位（产品层 vs core）非冲突。**phase2 §5.4 可在 caveat 引用 doc 05 作为 phase4 PRODUCE 阶段的 implementation guidance**。

---

## === 整合摘要 ===

### 总体数字

| Doc | ✅ 保留 | ⚠️ 冲突 | ➕ 补充 | ❌ 弃用 |
|---|---|---|---|---|
| 01 agent-loop | 5 | 4 | 3 | 3 |
| 02 harness | 6 | 4 | 3 | 4 |
| 05 multi-session | 6 | 0 | 2 | 0 |
| **合计** | **17** | **8** | **8** | **7** |

### 最严重的 3 个 ⚠️ 冲突 + 总裁决

#### #1 doc 02 §2.4 Compaction + G4 evidence trigger 激活 — escalate

- **冲突**：phase2 G4 砍 `session_before_compact` hook，stated 理由"已知用户 0"。**evidence 证伪**——coding-agent agent-session.ts:1665, 1938 真有 2 处生产代码消费 extensionCompaction 路径。
- **总裁决**：**escalate to coord-audit**——这是反向挑战 phase2 候选。砍决策方向可能仍对（无"cheap model"案例），但 stated 理由不准确。owner 应判断"contract 完整性 ≥1 vs 真实需求 = 0" 是否需要回滚 G4。
- **风险**：若不回滚，phase4 PRODUCE 不实现 compaction hook = coding-agent 移植到 tau 时**原样砍掉**这两处 if-else 分支；若 owner 觉得 coding-agent 的 contract 消费不是充分理由，G4 决策成立。
- **触发条件已满足**：coord-audit 任务包明说"phase2 §2.7 + 08 surprise #4 把 hooks supply compaction 当 feature。**你必须找证据**" — 我找了，**结果是部分支持 phase2 G4 砍但 stated 理由不准确**。

#### #2 doc 02 §2.2 phase machine — 真两难

- **冲突**：pi 4 入口断言 `phase === "idle"` 否则 busy，**但 0 测试覆盖**。phase2 §2.2 Session 没有 phase 字段。
- **总裁决**：**phase2 半对**——G4 砍后 phase 状态 collapse 到 `idle | turn`，但 phase2 §2.1 Run 接口**仍应加一个 `IsActive() bool`** 或类似 guard，防"产品层在 prompt 已 in-flight 时再调 prompt"未定义行为。
- **建议**：coord-audit 决定是否在 phase2 §2.1 Run 加这条 guard contract（增加 1 个 method，代价 ≤5 行）。

#### #3 doc 01 §Agent ↔ Harness contract 全段 + doc 02 §2.7 §2.8 Hook bus — pi 偏见系统性传播

- **冲突**：3 处都把 pi 的 callback bag / event-emitter 模式当通用契约描述（doc 01 §50-66, §87-104；doc 02 §137-150, §178）。phase2 §2.8 已用双 generic interface 修正。
- **总裁决**：**docs 错（系统性 pi 偏见——D5 反例当 elegant 写）**。这是 anti-anchoring 角度**最危险**的——phase4 builder 若先读 docs 再读 phase2，可能自然把双 interface 当"phase2 过度设计"。
- **建议**：docs/01 §50-66, §87-104 + docs/02 §2.7, §2.8 在 phase4 PRODUCE 时**完全重写**，用 phase2 §2.8 的 Chain[T] / LastWins[T] 替换。

### 高层模式观察

1. **D1 升级触发未达成**（G5）：3 docs 中没发现 ≥3 个新双重抽象实例。维持"反复反模式（仅 Agent/AgentHarness 1 例 + G2 撤销后空）"。
2. **R3 lint enforcement 在 docs 体系不存在**：3 docs 都写了"原则 / what's elegant"但没挂任何 linter——这本身就是 R3 反例。phase4 PRODUCE 阶段必须给 docs 也建立 doc-lint sync（phase2 §3 R3 已埋点，需要执行）。
3. **phase machine 的"防御性设计 0 测试覆盖" pattern** 出现在 pi 多处（phase 5 状态 retry dead；abort signal 3 处 fake）—— 是 pi 设计哲学的特征：**契约层加门面，实测覆盖薄**。tau 应避免（R1 + R3 联合：契约必须有 lint 或 test 兜底）。

---

## §5 self-doubt（强制）

1. **我读了什么 / 我跳了什么**
   - 完整读了：phase2-tau-design.md (606 行) + handoff (125) + R1-R6 (LTM 002) + P1+P2 (LTM 005) + docs/01 (120) + docs/02 (204) + docs/05 (130)
   - **未读**：phase2-self-doubt.md / phase2-naming-map.md / phase2-rule-enforcement.md / phase2-grey-resolutions.md（4 个拆出文件）。⚠️ 风险：**我可能复述了 phase2 主稿已知盲区**而没意识到——self-doubt.md 可能已写明 phase machine / G4 evidence 的反思。如果 coord-audit 觉得我的 ⚠️ 项重复了 self-doubt.md 已记录内容，我承认未充分准备。
   - **未读**：tau/docs/03-tool-protocol.md / 04-provider-abstraction.md / 06-self-extensibility.md / 07-testing-faux-provider.md / 08-design-principles.md / 09-go-module-structure.md（不在我任务范围，但 doc 02 提到的 surprise #4 在 doc 08 里——coord-audit 让我"找证据"我用了 grep 而非读 doc 08，可能漏 doc 08 的具体引用）

2. **我假设了什么（关于 phase2 的推测可能错）**
   - 假设 phase2 G4 砍 hook = 整套 compaction 都推产品层。**phase2 主稿 §6 G4 行实际只说"砍 hook"**，整套 compaction 是不是产品层我**推断**的（基于 phase2 §1 "Tau is not 全家桶"）。如果 phase2 实际意图是"core 保留 compaction 实现但不暴露 hook"，我整个 doc 02 §2.4 ❌ 弃用判断要修正。
   - 假设 doc 01 §Tool-result feedback 的"live mutation"描述是基于错误读 pi。**这是 Phase 1 我自己的 self-correction（§3.2）**——但**doc 01 写于 Phase 0 速读，可能不是基于 pi 源码而是基于 pi README 的 marketing 描述**。如果 pi README 真把 live mutation 当 feature 写，doc 01 不是错读源码而是对的描述了 pi 的**意图**——这种情况下 ❌ 等级降为 ⚠️。**值得 cross-check**。
   - 假设 phase2 §2.8 双 interface 决策是稳定的（v0.7 回滚）。phase2-self-doubt.md #8 提示"未来 reviewer 可能误读折叠尝试为合理"——我没读 self-doubt #8 全文，可能 phase2 自己已警惕。

3. **我的 audit 倾向哪些是受 Phase 1 anchoring 影响**
   - **doc 01 §Tool-result feedback 标 ❌**：直接动用我 Phase 1 §3.2 self-correction 作为反驳——这正是 coord-audit 让我用"源码反驳力"的场景。但⚠️ 风险：如果 doc 01 描述的是 pi **声称的** live mutation（README 层面），不是它 **实际做的**——我把"声称 vs 现实"的区分用源码碾压可能过度。
   - **doc 02 §2.7 Hook bus 标"pi 偏见，把 anti-pattern 当 elegant"**：这是 Phase 1 D5 anchor 直接复用。但⚠️ 风险：**doc 写于 Phase 0，作者可能没意识到 last-wins/chain 混用是 anti-pattern**——doc 把它当 elegant 的因果是"作者 Phase 0 时还没发现这个反例"，不是"作者偏向 pi"。区别细微但是 fairness 不同：第一种是 "doc 视角不全"（应补不应弃），第二种是"doc 偏见"（弃）。我倾向后者，但承认前者解释也合理。**coord-audit 自己判**。
   - **doc 02 §2.4 Compaction 标 ❌**：我 Phase 1 在 §2.2 把 compaction prepare/execute split 标为"值得 tau 借鉴"。现在 audit 时给 phase2 G4 砍辩护——**情绪上有"Phase 1 我点赞过的好东西被 Phase 2 砍了"的轻微 anti-instinct**。我用 evidence trigger 抑制了这股情绪（grep 出 coding-agent 真在用），但 ⚠️ 风险：我对 doc 02 §2.4 判 ❌ 比 ⚠️ 严，可能因为我**情感上想保住"Phase 1 借鉴清单 §2.2"的正确性**。**coord-audit 校准**。

4. **我没把握的判断（标 ⚠️-low-confidence）**
   - ⚠️-low-confidence：**doc 01 §Termination model "No turn cap" 该不该补到 phase2 §1**。我倾向"应"，但承认**phase2 故意不显式可能是为了 R5（核心层禁知产品规则）**——明说"无 cap"反而暴露"我考虑过 cap"。这是 trade-off 没明确量化。
   - ⚠️-low-confidence：**phase machine 升级到 phase2 §2.1 Run 加 IsActive() bool**。理由站住（G4 砍后 collapse 到 2 状态），但**phase2 §2.2 已有 ctx + cancel**——`IsActive() bool` 是否就是 `ctx.Err() == nil`？如果是，则不需要新加方法。我没在 phase2 §2.2 里查清 ctx 与 Run 关系——**可能我提的 IsActive() 是冗余**。
   - ⚠️-low-confidence：**G4 evidence trigger 触发的"反向挑战 phase2"严重程度**。"contract 消费 2 处但实质需求 0" 的判断——coding-agent 1665/1938 两处可能就是"为了 contract 完整性而写"的死代码。我没追这两处的实际 invocation 路径（who actually invokes that hook with provided.compaction set?）。如果 coord-audit 让我下钻，我能给更确切答案。**当前 evidence 是"消费分支存在"，不是"消费分支被生产代码 invoke"**。这点我承认不充分。

---

## 完成

3 docs audit 完。  
- 1 处 escalate（G4 evidence trigger）  
- 1 处建议小补（phase machine collapse 后 phase2 §2.1 是否加 IsActive()）  
- 1 处系统性 pi 偏见警告（Hook bus 在 doc 01 §50-66 §87-104 + doc 02 §2.7 §2.8 — phase4 PRODUCE 必须重写）  
- 整体 phase2 设计**主线对**，docs 主要问题是 pi 偏见传播，不是设计缺陷  
- **G5（D1 升级 ≥3 实例）**：3 docs 中未发现新实例，维持反复反模式仅 1 例

§5 self-doubt 4 条，含 3 处 anchoring 自检 + 3 处 low-confidence 标记。

待 coord-audit 决策：①G4 二审是否启动 ②phase2 §2.1 IsActive() 是否补 ③docs 01/02 重写预算何时投入。
