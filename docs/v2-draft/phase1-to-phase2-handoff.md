# Phase 1 → Phase 2 交接契约（handoff）

> 本文档是 Phase 1（pi 批判）到 Phase 2（tau 设计）之间的**契约文档**。
> Phase 2 的 coordinator（**预期不是 coord-critique 兼任**——见 owner 决定）应**先读本文件**再读主稿，以避免被 Phase 1 视角 anchor。
>
> - 主稿（Phase 1 完整证据与论证）：[`phase1-pi-critique.md`](./phase1-pi-critique.md)
> - 子报告：[`phase1-pi-critique-A1.md`](./phase1-pi-critique-A1.md)（packages/agent）+ [`phase1-pi-critique-A2.md`](./phase1-pi-critique-A2.md)（packages/ai + coding-agent 抽样）
> - 持久化结论：`ltm://phase1/summary` + `ltm://team/decisions/002-phase1-key-findings`（owner 持久化的 R1-R6 + W1-W3）

---

## 1. Phase 2 设计的 10 条入口硬约束

**这些是约束，不是建议**。任何 Phase 2 设计违反其中任何一条，必须给出明确的反驳证据 + Phase 2 owner 显式批准。

| # | 约束 | 来源（主稿） | 一句话说明 |
|---|---|---|---|
| C1 | **Registry 必须 scoped** | D4 | Provider Registry / Tool Registry 绑 session 或 workspace 作用域，**禁全局 mutable**。pi 的 `apiProviderRegistry` 是反例：runtime mutable + extension-triggered + 跨 session 互见 |
| C2 | **每条红线绑自动化检查** | D3 / M3 | red line in docs **必须**配 golangci-lint custom rule / go vet analyzer / CI grep gate。**没有 lint 兜底的红线视为不存在**。pi 反例：`biome.json:13` 主动关 `noExplicitAny` 配合 AGENTS.md "No `any`" |
| C3 | **类型签名 = 运行时真相** | D2 | 禁 `interface{}` 假饰；签名说能 cancel 必须真能 fire signal；`Optional` / nullable 不是用来"将来再说"的脱身阀。pi 反例：3 处 `new AbortController().signal` 立即丢 controller |
| C4 | **单一 canonical 抽象** | D1 | 一个概念一个公开抽象；演化通过 refactor 现有抽象，**不通过加相邻类**。pi 反例：Agent vs AgentHarness（已验证非 browser/Node 分层、纯并列冗余）+ AgentTool vs ToolDefinition |
| C5 | **Hook 两种语义两个 API** | D5 | last-wins 和 chain 必须**命名 + 类型签名都区分**（如 `reduceFirst` vs `chain`）。**绝不允许同一 API 按参数暗分语义**。pi 反例：emitHook last-wins vs emitBeforeProviderRequest chain，调用者无法静态判断 |
| C6 | **Transcript 单一 source of truth** | D7 + D9 | Loop 不维护内部 mirror；`transformContext` 的输入显式由调用方注入的 `Transcript` interface 提供。pi 反例："loop-internal-mirror-is-invisible-exhaust"——loop 自累积一份隐式 transcript 给 next-turn transformContext，调用方必须独立 subscribe events 重建 |
| C7 | **Provider 抽象层无产品知识** | D13 | 禁知道任何具体 tool 名 / 产品名；如需按 tool name 特殊处理，必须以"调用者注入的策略"形式（`transformToolCall(name) → name`）。pi 反例：`anthropic.ts:69-89` 硬编码 17 个 Claude Code 工具名 + `isOAuthToken` 强制重命名 |
| C8 | **消息归一化层必保留**（即使禁跨 session 切 provider） | G1（A2 自我修正版）| transform-messages.ts 220 行中**只 30%(~50 行) 是切 provider 专用**，70%(~150 行) 是同 provider 内必需归一化（图像降级 / 孤儿 toolCall 修补 / 跳过 errored assistant）。**砍归一化是错的**，只能砍跨 provider 部分。这条是 Phase 2 设计 message pipeline 的最关键约束 |
| C9 | **Tool Schema 单一来源** | B9 | Phase 2 必须**决策 schema 机制**（不可拖延）：① `reflect` + jsonschema 库 ② schema-first codegen ③ struct-tag + 手写 validator。三选一，且必须保证 "TS=JSONSchema=runtime-validation 三合一" 在 Go 体系里的等价物 |
| C10 | **Stream-as-pure-producer 是基石** | B1 | tau 的 provider 形态是 **producer function**（输入 model+context+options，返回 event stream），**不是 stateful client object**。Go 实现：`type StreamFunc func(ctx, model, opts) <-chan Event` 或 `iter.Seq[Event]`。这是其他设计（B2 Faux 测试、testability 整体）的基础 |

---

## 2. 必抄清单与待验证必抄

> **修订（owner-requested）**：原主稿 §2 把 B8 Skills 列入"必抄"，但 §5 自陈"是机制判断非行为数据"——这种应入"待验证必抄"小区。

### 2.1 直接必抄（行为/事实证据已充分）

| # | 设计 | 证据强度 | 移植标注 |
|---|---|---|---|
| B1 | Stream-as-pure-producer | A1+A2 互证 + test/harness/* 0 mock 实证 | LOW—generator→channel 重做 |
| B2 | Faux 走真注册路径 | 测试运行就是验证 | NONE |
| B3 | Result + stable error codes | 测试中错误路径被断言 | NONE—Go 更自然 |
| B4 | Session-as-tree | 数据结构选型，行为=结构 | NONE |
| B6 | Zero process.env in src | grep 实证（事实，非判断）| NONE—Go 更自然 |
| B7 | 并行 tool 确定性顺序 | 测试不需排序断言 = 行为验证 | LOW—Go errgroup |
| B9 | TypeBox 三合一 | 6 provider stream 入口实际调用、运行时校验有真实 code path | **HIGH—机制必须重设计**（C9 决策）|
| B10 | EventStream 双消费 | test/coding-agent 中 `await result()` 实际使用 | MEDIUM—机制不同 |

### 2.2 待验证必抄（Phase 2 必须先做行为验证才能落地）⚠️

| # | 设计 | 待验证什么 | 验证途径 |
|---|---|---|---|
| **B8** | Skills as files (not plugins) | 机制审视显示"优雅"，但 LLM 是否实际**主动 read** Skills、什么场景下不读、不读时会发生什么——**A2 §5 自陈无行为数据** | Phase 2 设计 Skills 系统前先做 LLM 行为实验：固定 prompt + 同一组 SKILL.md，统计 read 触发率；测对比组（强制注入 vs 自主 read）下任务完成率 |
| **B5** | Compaction prepare/execute 拆分 | A1 已验证 `prepare` 可纯函数测（机制证据），但**execute hook 整段替换**是否被实际场景使用？pi 自带的 compaction 行为之外，有没有真实 alternative implementation？A1 没数据 | Phase 2 如设计 compaction 系统，先确认是否真有 ≥2 种 execute 实现需求；只有 1 种实现时，"hook 替换"是过度抽象（pi 自身可能就是过度抽象）|

**判读规则**：待验证必抄类项目 Phase 2 不能直接落地为 design doc 的 first-class API，必须先有验证数据，否则会把 pi 的"看起来好"原样搬到 tau。

### 2.3 我审视过但仍归入"直接必抄"的边缘项（透明记录）

- **B3 Result+codes**：错误处理被实际测过（有行为证据）
- **B7 并行 tool 确定性**：有测试断言（行为证据）
- **B9 TypeBox**：6 provider stream 入口实际调用（行为证据；机制要 C9 重设）
- **B10 EventStream 双消费**：test 中 `await result()` 实际使用（行为证据）

如果 Phase 2 coordinator 重审视后认为这些项中有应进 2.2 的，可调整。

---

## 3. Phase 2 阻塞（必须解 / 应解）

### 🔴 阻塞 1：tau 目标语言用户未亲口确认（必须解）
- 主稿 §2 整张 Criterion A 移植表 + 本 handoff §1 的 C1-C10 都假设 tau = Go
- 依据是 `tau/docs/09-go-module-structure.md` 标题 + parent 给的 working assumption
- **用户从未亲口确认**
- 如非 Go：所有 LOW/HIGH 标注、generator→channel 等机制建议、golangci-lint / go vet / errgroup 等具体工具引用都需重做
- **由 owner 直接对接用户**（已在 owner 决定中明示）

### 🟡 阻塞 2：tau 范围确认（软阻塞）
- 当前 working assumption：agent core + provider 抽象（pi 的 packages/agent + packages/ai 等价物）
- **不含产品层（coding-agent 类）、不含 TUI**
- 如用户希望含产品层：A2 当前仅轻取样 coding-agent，需要回头加深；本 handoff 的 C7 / C8 / B8 / B9 等约束在产品层视角下可能要修订
- **不阻塞 Phase 2 启动**，但启动后第一周必须解掉，否则会做白工

---

## 4. 不阻塞推进但 Phase 2 必处理的开放项

- **AgentTool 形状跨范围盲区**：A1（agent core）和 A2（tools/skills）之间留了 AgentTool 定义文件 + tool-definition-wrapper.ts 没人完整读过。Phase 2 设计 tool 协议时必须补读 — 可激活 A1 (`7971618e-416d-46db-b787-171935727440`) 或 A2 (`9f7bf2ea-2618-4357-9459-ef52c4b9000e`) 异步咨询
- **B9 schema 机制选型**（C9）：reflect / codegen / struct-tag 三选一，必须 Phase 2 决策、不可拖到 Phase 4
- **B8 Skills 行为验证**（见 2.2）
- **B5 Compaction execute 真实场景验证**（见 2.2）
- **D1 是否升级为 pattern**：当前 2 实例（Agent/AgentHarness + AgentTool/ToolDefinition），未达 ≥3 门槛。Phase 3 audit docs 时如发现第 3 实例，可升级为正式 pattern；否则保留为"反复反模式（2 例）"

---

## 5. Phase 1 流程 lessons（Phase 2 工作流候选）

> 来自主稿 §6 Meta Findings + owner 持久化的 W1-W3。

- **W1 — Question-First + §5 self-doubt 联动**：每份 analyst 报告必有 §5"我没读什么 / 我假设了什么 / 我哪里没把握"，coordinator 必发 4-5 个 Q-First 挑战。Phase 1 中 A1+A2 各自自我修正了关键论点（A1 D7 替换 / A2 200→50 / A2 重→轻），准确率从 ~85% → ~98%
- **W2 — Criterion A（独立性追问）+ Criterion B（≥3 实例门槛）**：Phase 2 任何"业内最佳实践"主张都需要 Criterion A 二次评估；任何"系统性 / pattern / 反复"都需要 ≥3 独立实例
- **W3 — 防 anchoring**：Phase 2 不该由 Phase 1 coordinator 兼任（owner 已决定）。本 handoff 文件的存在就是为了让 Phase 2 coord 拿到 Phase 1 的**结论与约束**而不被 Phase 1 的**视角与情绪**沾染

---

## 6. Phase 1 资产清单（Phase 2 可调用）

### 文档
- 主稿：`phase1-pi-critique.md`（最新版本~470 行，本 handoff 拆出后）
- 子报告：`phase1-pi-critique-A1.md`（543 行，agent-core 完整证据）+ `phase1-pi-critique-A2.md`（609 行，ai+tools 完整证据）

### LTM
- `ltm://decisions/001-tau-mission` — 项目核心使命
- `ltm://phase1/summary` — Phase 1 摘要持久化
- `ltm://team/decisions/002-phase1-key-findings` — owner 持久化的 R1-R6 永久铁律 + W1-W3 工作流

### 可激活的 analyst（不需要重新 spawn，他们带 Phase 1 全量 context）
- **A1-agent-core** (`7971618e-416d-46db-b787-171935727440`)：packages/agent 专家。可咨询 agent loop / harness / session / compaction / hooks 等议题
- **A2-provider-tools** (`9f7bf2ea-2618-4357-9459-ef52c4b9000e`)：packages/ai + coding-agent tool 协议 + skills 专家。可咨询 provider 抽象 / tool 协议 / skills 机制 / message normalization 等议题

---

## 7. 文档版本

- v1.0 初版（从主稿 §7 拆出 + 加 §2.2 待验证必抄）
- 由 coord-critique 在 Phase 1 收尾时产出
