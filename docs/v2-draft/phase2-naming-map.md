# Phase 2 — tau ↔ pi 命名映射表

> **What**：tau 核心两层（agent core + provider 抽象）的概念命名与 pi（TypeScript 原型）类似抽象的对照表，含每条差异的 divergence 理由。
> **来自**：`phase2-tau-design.md` 主稿附录 A 拆出。
> **Phase 3 reader 为什么要看**：Phase 3 audit `docs/01..09` 时会遇到 pi 命名（"Agent" / "AgentTool" / "AgentHarness"），本表给出对应 tau 抽象。两边都不是的概念（如 `LoopOptions`）属于 tau 新增。

## 反 anchoring 的事后挂载原则

**这张表是事后挂载，不是事前桥梁**：tau 抽象的命名是从 Go 第一性原理 + 功能性视角决定的（不读 pi 也可独立理解），表中"差异"列只是把已成形的 tau 与 pi 对照，**不是用 pi 名字推 tau 形态**。这避免了 anchoring 复发：reader 可以独立理解 tau，再用此表回看 pi 坐标。

## 映射表

| tau 名 | pi 类似抽象 | 差异 (divergence-from-pi) |
|---|---|---|
| `Loop` | `Agent.prompt()` + `Agent.continue()` | tau 把 Loop 与 Session 拆开；pi 的 Agent 既是入口又是状态容器 |
| `Session` | `Agent.state` (隐式) + `AgentHarness` 部分 | tau 显式提一等抽象；pi 隐没在 state 里 |
| `Transcript` | `agent.state.messages: Message[]` | tau 给方法门面（Append/Slice/Subscribe）；pi 直接 mutable array |
| `Provider.Stream` | `streamSimple(model, ctx, opts)` | tau 是 Session-scoped 实例方法；pi 是包级函数（违反 R4） |
| `Provider.Complete` | `completeSimple(model, ctx, opts)` | 同上 |
| `Tool` | `AgentTool` from `packages/agent` | tau 字段集 = AgentTool 字段集；命名去掉 "Agent" 前缀（tau 不是"Agent 的 X"嵌套结构） |
| `ToolRegistry` | （pi 用 array 线性查找，无独立 registry 抽象） | tau 给 registry 抽象 + Session-scoped；pi 用 `agent.state.tools: AgentTool[]` 数组 |
| `AgentEvent`（高层）| （pi 没有明确分层）| tau 强制 raw `ProviderEvent` 与 `AgentEvent` 双层；pi 的 `AssistantMessageEvent` 同时承担两职 |
| `ProviderEvent`（raw）| `AssistantMessageEvent` | 仅用于 Loop 内部；产品层不接触 |
| `Chain[T]` / `LastWins[T]` | `emitHook` 单一 API 暗分语义 | tau 拆两 generic interface，编译期可拦误注册（C5 字面落地）；重复 Set → panic-fast（P2） |
| `SystemPromptFn`（function）| `agent.state.systemPrompt: string` (mutable field) | tau 用 closure 避免状态突变（R1 / A2 takeaway #7） |
| `ProviderRegistry`（Session-scoped）| `apiProviderRegistry` (包级 mutable) | tau R4 / C1 落地；pi D4 反例 |

## 看完之后

回主稿 `phase2-tau-design.md` §2 看每个 tau 抽象的完整设计；本表只是坐标转换工具。