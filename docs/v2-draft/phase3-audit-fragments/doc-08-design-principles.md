# Audit Fragment — doc-08 Design Principles

> **Auditor**: coord-audit (self-audit, Owner-assigned)  
> **Source**: `tau/docs/08-design-principles.md` (157 行)  
> **Standard**: `phase2-tau-design.md` v1.1 + handoff §1 C1-C10 + R1-R6 + P1+P2  
> **Method**: 二选一裁决树（L1 R/C 命中→docs 错；L2 G1-G6 已决策→docs 错；L3 真冲突→trade-off；L4 模糊→escalate）

---

## §A — ✅ 保留项（与 phase2 一致）

| # | docs § / 行 | 内容 | 匹配 phase2 |
|---|---|---|---|
| A1 | §8.1 "Append-only beats mutable state" 行 7-9 | 历史不可变，状态推导自事件流 | phase2 §2.3 Transcript 单一真相 + 不可 mutate（C6）✓ |
| A2 | §8.1 "Hooks at every meaningful boundary" 行 11-15 | chain-of-responsibility patches；**last-non-undefined-wins** 单结果语义；patch field-by-field no deep merge | phase2 §2.8 Chain[T] + LastWins[T] **比 doc 更严格**：phase2 用两个独立 generic interface 编译期阻止混用 |
| A3 | §8.1 "Errors are values" 行 25-27 | `throw` 仅程序员错误；loop 几乎无 `try/catch` | phase2 §2.6 ToolExecuteFn 返 `(ToolResult, error)`；§2.5 ErrorEvent 是事件类型 ✓ |
| A4 | §8.1 "Streaming is the only mode" 行 29-31 | non-streaming 用 `await stream.result()`；消除双路径 | phase2 §2.4 Stream + §2.1 Run.Done ✓ 概念一致 |
| A5 | §8.1 "Multi-session safety = no shared writable state, not locks" 行 33-35 | 不需要 lock/daemon/IPC | phase2 C1/R4 包级无 mutable，**比 doc 更严格**：phase2 是 lint-enforced，doc 是 architectural intent |
| A6 | §8.1 "Pinned deps, no auto-updates" 行 37-39 | 依赖即供应链风险 | phase2 未明文，但 R3 元铁律精神一致（CI 是承诺）；非冲突 |
| A7 | §8.1 "No backward compat unless asked" 行 41-43 | 自由 refactor，无 migration code | phase2 §1 第 4 段"divergence-from-pi"暗合；与 R6（哲学 first-class doc）兼容 ✓ |
| A8 | §8.1 "Provider usage = source of truth for tokens" 行 45-47 | 不重实现 tokenizer | phase2 未明文但 §2.4 不持有 tokenizer，aligned by omission |
| A9 | §8.2 #3 Hook bus chain-of-resp 行 67-69 | 同 A2 | phase2 §2.8 ✓ **强一致** |
| A10 | §8.2 #5 Faux provider 行 75-81 | "no mock layer above provider"；测真实 loop/harness/compaction | phase2 B2 必抄 ✓ + handoff §2.1 行为证据强 |
| A11 | §8.3 "Two queue layers" 行 85-87（Agent + Harness 双 queue 重复）| pi 自承反模式："In Go keep one queue layer" | phase2 §2.1 砍 steer/followUp 推 5+ → 0 queue 比 1 还少 ✓ |
| A12 | §8.3 "AgentHarnessOwnEvent vs AgentEvent siblings" 行 89-91 | pi 自承事件双层并列 | phase2 §2.5 sealed AgentEvent 单 interface + 内部 ProviderEvent 严格双层 ✓ |
| A13 | §8.3 "TS-specific declare module" 行 93-101 | TS-only module augmentation 不可移植 | phase2 §1 隐性砍 TS 特性；与 R1 类型即真相精神一致 ✓ |
| A14 | §8.3 "prepareArguments shim" 行 104-106 | "Not necessary if tool args always parsed JSON" | phase2 §2.6 保留 PrepareArgs 为可选（`PrepareArgs func(raw json.RawMessage) (any, error)`）— **轻 ⚠️**：doc 建议直接砍，phase2 留可选——见 §B-W2 |
| A15 | §8.3 "Compaction prompt is hard-coded" 行 108-110 | doc 建议 `CompactionPolicy` interface + `BuildPrompt(messages, prevSummary)` | **phase2 §6 G4 砍掉了 compaction execute hook**——但保留"prompt 应可配"概念 = 推产品层（产品层若做 compaction，自然该有 policy interface）。Aligned by omission |
| A16 | §8.3 "models.generated.ts 16K LOC" 行 112-114 | doc 建议 swappable JSON | phase2 §2.4 不持 model registry，推产品层 ✓ |
| A17 | §8.3 "Lazy provider loading via import.meta.url" 行 116-118 | doc 建议 build-tag 或 blank-import | phase2 未明文但与 R5（核心层无产品名）一致：build-tag = 编译期排除 vendor-specific 依赖 ✓ |
| A18 | §8.3 "Multi-session safety relies on convention" 行 120-124 | doc 建议规则编进 tool 不在 prompt | 推产品层；与 phase2 R5 一致（核心层不知 git/bash 工具） |
| A19 | §8.4 surprise #1 "no flock anywhere" 行 128-130 | architectural elegance | 推产品层（phase2 不覆盖持久化） |
| A20 | §8.4 surprise #2 "loop has no max-turns/budget cap" 行 132-134 | budget 在 hook 不在 loop | phase2 §2.1 Loop 无字段 + §2.8 Hook 表 ✓ 强一致；budget 通过 BeforeProviderRequest LastWins hook 落地 |

**小计**：20 ✅ 保留项。

---

## §B — ⚠️ 冲突项（与 phase2 矛盾，必须裁决）

### W1 — Top borrowable #1: Append-only session **tree** with `LeafEntry`

**docs §8.2 行 53-57**：  
> "Append-only session **tree** with `LeafEntry` pointers. Single most reusable idea. JSONL on disk, in-memory `byId` map, navigation-as-event. Free undo, free fork, free time-travel. ~300 LOC in Go."  
（强调原文）

**phase2 §2.3** Transcript 是 **flat** 结构：`Position int64` 单调递增；Append/Slice/Subscribe 三方法门面；**没有** tree / LeafEntry / navigation entry / fork 概念。

**裁决树**：
- L1 命中？无直接 R/C 红线命中 — Transcript 用什么底层结构 R1-R6 不限定
- L2 G1-G6 已决策？**无明文**——phase2 §6 G1-G6 没有讨论"Transcript 是 flat vs tree"
- → L3 真冲突 trade-off 量化：

| 维度 | flat Position（phase2）| tree+LeafEntry（pi）|
|---|---|---|
| 实现成本 | ≤100 LOC（slice + index）| ~300 LOC（doc 自标）|
| Subscribe 接口兼容性 | ✓ 直接 | ✓ 可包装 |
| Slice 接口兼容性 | ✓ 直接 | ⚠️ 必须先 `getPathToRoot(leafId)` 再 slice，O(depth) |
| Fork / branch / undo | ❌ 不支持（用户需自己实现）| ✓ "免费" |
| Time-travel | ❌ 不支持 | ✓ "免费" |
| persistence-friendly | 不直接（中性）| 直接（JSONL append-only 自然映射）|
| **R2 单一 canonical** | flat = 简单 single source of truth | tree 需"current leaf 是哪个"二级状态——但 doc 解法"setLeafId 也是 entry"消除二级状态 |
| **C6 Loop 不维护 mirror** | flat 直接读 | tree `buildSessionContext(entries)` 重建 message list 即"viewer pure function over events"——满足 C6 |

**我的二选一裁决：phase2 设计**（保留 flat）**+ self-doubt 升级**（不是反向修订）。

**理由**（≤3 行）：
1. tau 范围（agent core + provider，不含产品层），**fork/branch/undo/time-travel 全是产品层需求**——webui 多 session 是隔离的不同 sessions（C1）不是 fork；TUI 重启 cursor 续接是 Subscribe 增量不是 navigation
2. Transcript 接口（Append/Slice/Subscribe + Position）**不阻碍**产品层在上面实现 tree 视图——只要产品层维护 leaf 指针 entries 即可，phase2 Transcript 是"消息流"语义、产品层可加"导航语义"二级层
3. doc 自承 ~300 LOC，但**这 300 LOC 推产品层 = phase2 范围红线吻合**

**反向挑战 phase2 触发？** 否——doc 没有提供 phase2 设计时没有的新数据；Phase 2 知道 pi tree 模型（A1 报告中讲过 D7 反 internal-mirror）但有意识把 tree 推产品层。

**self-doubt 升级**：phase2 §2.3 末尾应加一行 design note：
> "Transcript 是 flat 设计；产品层若需 fork/branch/undo 语义，可在 Transcript 之上构建 tree 视图（pi 的 LeafEntry 模式是参考实现），但 core 不强制此结构。"

**Owner 决策点**：是否加这行 design note 到 phase2 v1.2？我倾向加。

---

### W2 — `prepareArguments` shim 强度判断

**docs §8.3 行 104-106 + §8.2 #1 间接 + §1 doc 03**：建议直接砍。

**phase2 §2.6 Tool struct**：保留 `PrepareArgs func(raw json.RawMessage) (any, error) // optional pre-validate normalization`

**裁决树**：
- L1 命中？无 R/C 直接命中
- L2 G1-G6？**无明文**
- → L3 trade-off：

| 维度 | 砍（doc 建议）| 保留可选（phase2）|
|---|---|---|
| 兼容性 | ❌ 老模型 raw-string 输入直接失败 | ✓ 工具可在拿到时归一化 |
| R2 单一抽象 | ✓ 一层 schema 验证 | ⚠️ 看似双层但只 1 个 callsite（Validate 之前）|
| 工具作者负担 | ✓ 不需懂这个字段 | ⚠️ 多一个可选字段需懂"何时用" |

**我的二选一裁决：phase2 设计**（保留可选）。

**理由**：phase2 标 `optional` + `pre-validate normalization`——是兼容性保险栓不是必需。doc 说"if tool args always parsed JSON" 砍 = 假设条件；**真实 LLM 生态老模型仍发 raw-string-args**（doc 03 自承）—— Phase 4 PoC 时若证明所有目标 model 都发干净 JSON 可考虑砍，但这不是 Phase 3 决策。

**升级 phase2？** 否。已是 optional，不需改。

---

### W3 — Top borrowable #4: Two-queue steering model

**docs §8.2 行 71-73**：  
> "`steer` = inject between turns; `followUp` = inject after agent would stop. `QueueMode = "all" | "one-at-a-time"`. Maps cleanly onto interactive UX."

**phase2 §2.1 + self-doubt #2**：明文砍 steer/followUp 推 Phase 5+；self-doubt 已埋 audit 必查点。

**裁决树**：
- L1 命中？无 R/C 直接命中
- L2 **命中 phase2 §6 G-?**：phase2 §6 G1-G6 没明文 G "queue 决策"，但 §2.1 主稿明文砍 + handoff §2.2 列入"待验证必抄"小区，§2.1 自陈"steer/followUp 推迟到 Phase 5+ 验证（A2 #4）"
- → L2 已决策：phase2 错？**否**——phase2 是 evidence-based 推迟（A2 takeaway #4 自承"v2.0 可砍但默认砍"）= **已决账，不重开**

**我的二选一裁决：phase2 设计**（维持砍）+ **A1 必查 G4 evidence-search 平行**：A1 任务包已含 doc 02 §2.7 + 08 surprise #4 evidence-search trigger，结果会反过来印证 W3 是"真砍"还是"漏砍"。

**理由**：doc 08 §8.2 是 pi 视角的"borrowable 排名"，本质是 design recommendation 不是 evidence。phase2 self-doubt #2 已警告此风险；A1 evidence search 后裁决——若 pi 源码有真实 ≥2 alternative steer/followUp 实现 → 升级；若 0 → 维持砍。

**升级 phase2？** 待 A1 evidence。**已 trigger A1 任务包必查项**。

---

### W4 — Top borrowable #2 + Surprise #4: Compaction execute hook（G4 反挑战）

**docs §8.2 行 60-65 + §8.4 surprise #4 行 140-142**：
- §8.2: "Iterative compaction with cut-point validation + previous-summary update + file-op tracking. ~600 LOC in Go." Top borrowable #2
- §8.4: "`AgentHarness.compact()` allows hooks to **supply** the compaction. `session_before_compact` returning `{compaction}` skips the LLM call entirely. Means an extension could do compaction with a cheaper model, a local LLM, or a deterministic algorithm."

**phase2 §6 G4**：明文砍 compaction execute hook。论证：(1) ≈ 50 行成本 (2) 已知用户 0 (3) 与 `Provider.Complete + Transcript` 路径冗余 → 违反 R2。

**裁决树**：
- L2 命中 G4 已决策 → **doc 错（不重开账）**
- 但 doc surprise #4 提供"性能优化点"理由（cheaper model/local LLM/deterministic algo）——是否新数据？

**我的二选一裁决：维持 phase2 G4**（砍）。

**理由**：
1. surprise #4 是 doc 作者**hypothesized use case**——不是 evidence。doc 自己说"could do" 不是 "pi 内部有人这么用"
2. **A2 G4 evidence-search trigger 已派**：若 A2 在 doc 04 §4.7+§2.4 找到真实"用便宜 model 做 compaction"≥1 引用 → Owner 二审 G4
3. **phase2 §6 G4 决策路径**："compaction 推产品层"——产品层用 `Provider.Complete` 做"convert n msgs → 1 summary msg"是**直接路径**，不需 core 提供 hook 入口。性能优化（便宜 model）= 产品层选不同 Provider name 即可
4. R2 单一 canonical：core 提供 compaction hook + 产品层用 Complete 直做 = 两条路完成同一目标 = 反 R2

**反向挑战 phase2？** 否——除非 A2 evidence ≥1 命中。

---

### W5 — "Phase machine prevents concurrency bugs without locks"（meta principle）

**docs §8.1 行 21-23**：  
> "A single enum (`idle | turn | compaction | branch_summary | retry`) blocks invalid transitions. Combined with `pendingSessionWrites`, mid-turn state changes are safe."

**phase2 §2.2 Session**：无 phase enum；通过 `ctx context.Context` + Loop 无字段 + Tool.Mode 控制 sequential/parallel + compaction 推产品层达成同等不变量。

**裁决树**：
- L1 命中？无直接命中
- L2 G1-G6 已决策？无明文，但 self-doubt 中我的 Phase 0 候选 #5 已识别此问题
- → L3 trade-off：

| 维度 | 加 phase enum（doc）| 不加（phase2）|
|---|---|---|
| 实现复杂度 | +1 enum + 转换检查 +`pendingSessionWrites` flush 边界 | 0（结构性消除）|
| 防 compaction-during-turn bug | ✓ runtime enum check | ✓ 结构性（compaction 不在 core）|
| 防 fork-during-compaction bug | ✓ runtime enum check | N/A（fork 不在 core）|
| 防"产品层 hook 在 Provider 调用中触发新 turn" | ✓ enum block | ❌ 需 Hook 设计本身保证 |

**我的二选一裁决：phase2 设计**（不加 enum）+ **self-doubt 加一条**。

**理由**：phase2 §2.2 的不变量是**结构性消除**——Loop 无字段、compaction 推产品层、Tool.Mode 控并行——根本不存在"compaction-during-turn"状态空间。pi 的 phase enum 是因 pi 把 loop+harness+compaction 同进程同对象耦合在一起才需要 runtime check；phase2 通过分离已经免除。

**self-doubt 升级**：phase2 §2.2 应加一条注：
> "Session 不持有 phase enum——pi 的 `idle|turn|compaction|...` 状态机是其 loop+harness+compaction 同对象耦合的产物。tau 通过 (a) Loop 无字段 (b) compaction 推产品层 (c) Tool.Mode 控并行 三个结构性分离消除了 phase enum 的存在前提。**注意**：若产品层自实 compaction，需自带 phase 防护（比如以 LastWins hook 形式锁住 BeforeProviderRequest 入口期间的并发触发）。"

**Owner 决策点**：是否加这条 design note？我倾向加。

---

## §C — ➕ 补充项（docs 有 phase2 没考虑/未明文的角度）

| # | docs § / 行 | 内容 | 应否补 phase2 |
|---|---|---|---|
| C1 | §8.4 surprise #7 行 152-154 | `Usage` 4 字段：`{cacheRead, cacheWrite, input, output}` 分离（cache 10× 便宜，cost 计算 4 字段独立）| **应**：phase2 §2.4 没有 Usage 类型；Provider 返 token usage 是真实接入点（compaction 触发 + cost 计算依赖此）。建议 phase2 §2.4 加 `type Usage struct { Input, Output, CacheRead, CacheWrite int }` |
| C2 | §8.4 surprise #8 行 156-158 | `getApiKey` 是 per-call 不是 per-session（OAuth 短 token + tool 跑分钟级）| **轻补**：phase2 §2.4 OAuth 注释说"是实现细节"，但**没说 per-call 重新 resolve**。建议 phase2 §2.4 OAuth 段加一行："OAuth provider 实现内部应支持 per-call token 刷新（pi A1 §A `getApiKey` 模式）—— short-lived token 在长 tool 跑期间可能过期" |
| C3 | §8.1 "Append-only beats mutable state" 哲学概括 | 历史不可变作为**架构默认**——不只是 Transcript，是整个数据流原则 | **否**：phase2 §1 5 断言 + R2/R6 已隐含此精神；R6 哲学 first-class doc 路径会自然把这条写进 PHILOSOPHY.md（Phase 4 任务） |
| C4 | §8.2 #2 "iterative compaction" 算法细节（cut-point/previous-summary/file-op tracking）| 产品层做 compaction 时的算法蓝本 | **否（推产品层）**：phase2 §6 G4 已决策 compaction 推产品层；这套算法应进 docs/v2/ 的"产品层 compaction reference algorithm"（Phase 4 deliverable）而非 core 设计 |
| C5 | §8.3 "encode git rules in tools not prompt" | multi-session safety 的产品层最佳实践 | **否（推产品层）**：phase2 R5 已禁止 core 知 git/bash 等工具名；这条应进 Phase 4 docs/v2/ 的"产品层 multi-session safety 指南" |
| C6 | §8.4 surprise #5 "setLeafId is itself an entry" | navigation = event 模式（避免 tree 状态需要二级 mutable 字段）| **否（产品层）**：与 W1 同议题；推产品层"若实现 tree 视图请用此模式" |

**小计**：6 ➕ 项，2 项**应补**（C1 Usage / C2 OAuth per-call），4 项推产品层。

---

## §D — ❌ 弃用项（明显 pi 偏见，不能进 docs/v2）

| # | docs § / 行 | 内容 | pi 偏见类型 |
|---|---|---|---|
| D1 | §8.3 行 100-102 `RegisterCustomMessageType` Go sketch | 用 `RegisterCustomMessageType("bashExecution", &BashMessageHandlers{...})` 替换 TS `declare module` | **包级 mutable 注入风险**——sketch 是 package-level register，违反 R4。Go port 应是 Session-scoped CustomMessageRegistry 或编译期 type registration（不暴露 runtime register）。**doc 建议本身没错（替换 TS-only 机制是必要的），但 sketch 形态是 pi 偏见的 Go 投影**。Phase 4 设计 custom message 时按 R4/C1 重设 |
| D2 | §8.3 行 116-118 "Lazy provider loading via `import.meta.url` 替换为 blank-import" | doc 建议 Go 用 blank-import 让 linker dead-strip | **TS-specific 残留思维**——Go 的"blank import"自动注册是 init-time 包级 mutable，与 phase2 §2.4 ProviderRegistry Session-scoped 直接矛盾（R4）。正确 Go 做法是**显式 `sess.Providers.Register(...)`**——blank-import 让 init() 偷偷往全局注册 = 反 phase2 设计哲学。Phase 4 docs/v2/ 必须明文反对 init() 自动注册模式 |

**小计**：2 ❌ 项，都是 pi → Go 朴素移植造成的反 R4 包级 mutable 风险。

---

## §E — 整合裁决（doc-08 总账）

- ✅ 保留：20 项
- ⚠️ 冲突：5 项（W1 LeafEntry / W2 prepareArguments / W3 steer queue / W4 compaction hook / W5 phase machine）
  - W1：phase2 赢 + self-doubt 升级（design note）
  - W2：phase2 赢（已是 optional 不需改）
  - W3：phase2 赢（L2 已决策不重开 + A1 evidence search 平行验证）
  - W4：phase2 赢（L2 G4 已决策 + A2 evidence search 平行验证）
  - W5：phase2 赢 + self-doubt 升级（design note）
- ➕ 补充：6 项，**2 项应补 phase2**（C1 Usage 类型 / C2 OAuth per-call 注释），4 项推产品层
- ❌ 弃用：2 项（D1 包级 register Go sketch / D2 blank-import 暗注册）

**总反向挑战 phase2 触发数**：0（W1+W5 是 self-doubt 升级即 design note，不是反向修订；C1+C2 是补充而非修正）

---

## §F — Self-doubt（强制 ≥4 条）

1. **我跳了什么**：doc 08 是 cross-cutting 哲学层，没有具体代码；我没去 doc 02/04 验证 doc 08 引用的"Top borrowable #1 ~300 LOC""#2 ~600 LOC"等数字是否准——但这不是关键判断点，因为 trade-off 不依赖精确 LOC。
2. **我假设了什么**：W1 裁决依赖"产品层可在 Transcript 上构建 tree 视图"——**未验证**。phase2 §2.3 Subscribe/Slice 接口是消费导向的，**写入路径是否支持产品层做"插入 leaf 指针 entry"未明**——phase2 self-doubt #7 已埋同问题（"Transcript 写入路径是否支持 summarize→replace tail 这类操作未验证"）。Phase 4 PoC 必须验。
3. **我受 anchoring 影响哪里**：phase2 §6 G1-G6 + self-doubt #2 已决策 W3 (steer) + W4 (compaction) → 我裁决"维持 phase2"是 phase2-anchored 的（"已决账不重开"是 L2 准则但本身有 anchoring 嫌疑）。**保护机制**：A1+A2 evidence-search trigger 是反 anchoring 平行验证——若 evidence ≥1 就升级。
4. **我没把握的判断**（标 ⚠️-low-confidence）：
   - W2 prepareArguments：doc 直接砍 vs phase2 留 optional，trade-off 接近一比一，我裁 phase2 的偏向有"不动 phase2 节省修订成本"嫌疑
   - C1 Usage 类型应补：我倾向"应补"，但具体位置（§2.4 还是新独立 §2.10）未定，留 Owner 裁
5. **我的预测错误模式（元学习）**：Phase 0 我把 doc 08 当"哲学层 → 与 phase2 §1 5 断言互补"整体预测，没分 §8.1 哲学/§8.2 借鉴/§8.3 反例/§8.4 surprise 4 层。实际：§8.1 + §8.3 + §8.4 与 phase2 强对齐（21 ✅）；**冲突全部集中在 §8.2 "Top borrowable" 段**（5 ⚠️ 中 4 项来自 §8.2）。下次 audit 哲学+借鉴+反例混合 doc 时，分层预测，不同段不同期望。

---

## §G — 给 coord-audit 整合用的 evidence 摘要

| 关键 evidence | 出处 | 影响 |
|---|---|---|
| doc 08 §8.1 last-non-undefined-wins 单结果语义 | doc 08 行 13-15 | 印证 phase2 §2.8 LastWins[T] 设计是 pi 实践真要素 |
| doc 08 §8.3 Agent vs AgentHarness 双 queue 重复 | doc 08 行 85-87 | pi 自承反模式——印证 phase2 §1 砍 AgentHarness 决策 ✓ |
| doc 08 §8.3 AgentHarnessOwnEvent vs AgentEvent siblings | doc 08 行 89-91 | pi 自承事件双层并列差——印证 phase2 §2.5 sealed AgentEvent + 双层 stream 决策 ✓ |
| doc 08 §8.4 surprise #4 hooks supply compaction 是 hypothesized use case 不是 actual user | doc 08 行 140-142 | A2 G4 evidence-search 应能进一步定性"是否有真实用户" |
| doc 08 §8.4 surprise #7 Usage 4 字段分离 | doc 08 行 152-154 | C1 Usage 类型应补 phase2 §2.4 |
| doc 08 §8.4 surprise #8 getApiKey per-call | doc 08 行 156-158 | C2 OAuth per-call 注释应补 phase2 §2.4 |
| doc 08 §8.2 Top 5 中 #3 Hook chain 排第 3 而非更高 | doc 08 行 67-69 | 印证 phase2 §2.8 双 generic interface 投资是值得的（pi 把 hook 排第 3）|

---

## §H — 给 Owner 的 1 行简报

**doc-08 audit 完成**：20 ✅ / 5 ⚠️（全 phase2 赢）/ 6 ➕（2 应补：Usage 类型 + OAuth per-call）/ 2 ❌（包级 register sketch）。**0 反向挑战触发**——W3+W4 待 A1/A2 evidence 平行验证；W1+W5 仅 self-doubt 升级（加 2 行 design note）。
