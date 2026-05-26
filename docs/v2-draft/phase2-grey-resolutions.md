# Phase 2 — G1-G6 灰色地带处理详细论证

> **What**：handoff §4 列出的 6 个灰色地带（G1-G6）在 Phase 2 的处理决策详细论证——每项的量化成本、收益、R2 自审、未来 trigger。
> **来自**：主稿 `phase2-tau-design.md` §6 拆出（保留主稿 §6 的速览表，本文是详细论证）。
> **Phase 3 reader 为什么要看**：Phase 3 audit 时如发现 docs/01..09 中有相关线索能影响 Gx 决策（如 G4 出现 ≥2 真实需求），可触发 Gx 决策回退；本文给出回退判据。

## 元原则适用

下列处理大量应用了 P1（review 痛 < 用户痛）和 P2（panic-fast 等同 MustCompile 模式）—— 见 `ltm://team/decisions/005`。

## G1 — Message normalization 砍多少（C8 修正版）

**结论**：保留同 provider 内归一化（70% / ~150 行级），砍跨 provider 切换（30% / ~50 行级）。
**论证**：A2 §A 已确认产品层从未直接调用 `transformMessages`——provider 边界自动调用，对产品层透明。砍跨 provider 切换路径不影响产品层 API。
**实现位置**：归一化代码在 `core/provider/normalize/` 包下，每个 Provider 实现引入。tau 不暴露 `transformMessages` 公共 API（A2 takeaway #3 缩边界）。
**回退 trigger**：无（决策已稳）。

## G2 — AgentTool 双重抽象是真冗余还是分层（D1 实例 #2）

**结论**：撤回 D1 实例资格——AgentTool/ToolDefinition 不是真冗余。
**论证**（A1 §5 判据）：
- 跨包：AgentTool 在 `packages/agent`（agent core 公共契约），ToolDefinition 在 `packages/coding-agent`（产品层私有）
- 抽象层不同：core 不知 UI render / prompt snippet / ExtensionContext
- 字段正交：ToolDefinition 加的字段全是产品 vertical 才需要的（renderResult / promptSnippet / promptGuidelines）
- Wrapper 单向丢信息（不是 ID 守恒）：`wrapToolDefinition` 把 ToolDefinition 降级为 AgentTool，丢弃 UI/prompt 字段；这是健康的"core 不漏 UI"边界

**tau 沿用此分层**：核心层 Tool 一层（§2.6），产品层自由扩展并 wrap 降级。
**D1 反复反模式从 2 例 → 1 例**（仅剩 Agent/AgentHarness 实例）。

## G3 — B8 Skills 行为验证

**结论**：Phase 2 不为 Skills 设计 first-class 抽象。
**论证**：
- handoff §2.2 标注 B8 "待验证必抄"——LLM 是否实际主动 read SKILL.md 无行为数据
- A2 §A.skills 确认 coding-agent 把 skills 注入纯通过 system prompt 拼装，pi-agent-core 对 skills 抽象透明
- 因此 tau 核心层只需 `SystemPromptFn` 接入点；"Skills" 是产品层概念，由产品层在 SystemPromptFn closure 里读 SKILL.md 文件并拼接

**回退 trigger**：Phase 5+ 产品层的 LLM 行为验证（固定 prompt + 同组 SKILL.md 统计 read 触发率，对比强制注入 vs 自主 read 任务完成率）。如证明 Skills 需要核心层 first-class 支持，回头加。

## G4 — B5 Compaction execute hook 是否过度抽象

**结论**：砍。
**三条量化论证**：

### (1) 保留成本量化

如保留 `CompactionExecuteHook`，§2.1 Loop 需加：
- HookSet 加 `LastWins[CompactionRequest]` 字段（~2 行）
- Loop 在 token 阈值触发时调用 hook（~10 行 trigger 逻辑）
- `CompactionRequest` / `CompactionResult` 两个 struct（~10 行）
- 默认 implementation（~30 行：截断 + summarization via Provider.Complete）

**总成本 ≈ 50 行 + 1 个新公开类型 + 1 个新 hook 字段**。这不是"零成本预留"。

### (2) 收益量化

handoff §2.2 / B5 自述：pi 的 compaction execute hook 没观察到 ≥2 alternative 实现。Phase 1 → Phase 2 之间没新增证据。
- coding-agent 用默认 compaction → 不需要 hook
- TUI / webui 是订阅消费方，不参与 compaction 决策 → 不需要 hook
- 假想第三方 extension 自定义 compaction 策略？→ handoff §2.2 明说"只有 1 种实现时，hook 替换是过度抽象（pi 自身可能就是过度抽象）"

**收益 ≈ 0 个已知用户**。

### (3) R2 自审

保留 hook 等于在"自定义 compaction"概念上同时存在两条路径：
- hook 路径（Loop 自动触发）
- `Provider.Complete + Transcript` 路径（产品层手动触发 summarization）

**违反 R2 单一 canonical**。今天 §2.3 + §2.4 已经支持后者；保留 hook 是冗余抽象。

### 未来 trigger

Phase 5+ 第一个 compaction 自定义需求出现时：
1. 先验证 `Provider.Complete + Transcript` 路径是否真支持（如 Transcript Slice/Append 是否对产品层 reachable）
2. 若不支持再加 hook（HookSet 加字段是向后兼容增量，不破坏 §2 抽象稳定性）

**核心假设未验证**（见 `phase2-self-doubt.md` #7）：Transcript 写入路径是否支持产品层做 "summarize → replace tail" 操作。如验证失败，G4 决策要回退。

## G5 — D1 升级为 pattern（≥3 实例）

**结论**：当前仅 1 例（Agent/AgentHarness；G2 撤销后只剩这一例）。
**回退 trigger**：Phase 3 audit docs 时若发现 docs/01..09 中有第 2、第 3 例，可升级为正式 pattern；否则保留为"反复反模式（1 例）"。

## G6 — （无）

handoff §4 列出 5 项，G6 占位空。

## 看完之后

回主稿 §6 看速览表；本文是配 §6 的详细论证。