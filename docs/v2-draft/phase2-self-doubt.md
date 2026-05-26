# Phase 2 — 自我盲区与已知未消化项

> **What**：Phase 2 设计稿（`phase2-tau-design.md`）的 §5 self-doubt 强制项——作者（coord-design）显式自陈"我没读什么 / 我假设了什么 / 我哪里没把握"。
> **来自**：主稿附录 B 拆出。
> **Phase 3 reader 为什么要看**：Phase 3 audit 时如果发现 docs/01..09 中有相关线索能消化下面任一项，应当回头修订主稿对应章节；本表是 audit 的优先调查目标清单。

## 协议来源

W1 Question-First + §5 self-doubt 联动（来自 Phase 1 工作流 `ltm://team/decisions/002`）：每份产出必有此节，准确率从 ~85% → ~98%。

## 9 项盲区/已知未消化项

1. **没真读 pi 的 `Agent.subscribe` 内部实现** — A1/A2 都没下钻 raw provider event → 高层 `AgentEvent` 的 dispatch 逻辑。tau 主稿 §2.5 的 sealed AgentEvent 类型集（MessageStart/Delta/End/ToolCall*/Turn*/Error）是从产品层"想消费什么"反推的，不是从 pi 实际事件类型一一对应。**风险**：如果 pi 还有第 9、第 10 类语义事件（如重试 / cache hit / token usage），可能漏。Phase 3 audit 应查 docs/01..09。

2. **⚠️ Phase 3 audit 必查项**：**假设 Go 的 `context` + `goroutine` 取消语义足够表达 pi 的 abort 语义** — pi 的 `agent.abort()` + `waitForIdle()` 在 Go 里用 `ctx.Done()` 表达，但 pi abort 是否还涉及"清空 follow-up 队列"等 stateful 操作（A2 §A `clearAllQueues`）未逐字段验证。A2 takeaway #4 说"v2.0 可砍"——这是 G4-style "已知用户 0 + R2 自审" 模式之外的另一处砍决策，但**论证强度弱于 G4**（没做"保留成本量化 / R2 自审 / 未来 trigger"三件套）。**Phase 3 audit 应主动挑战此决策**：docs/01..09 中如发现 follow-up 队列被多处假设/使用，即 evidence 反对默认砍，应触发回滚为 §2.1 `Run` 接口扩展（加 follow-up 队列）。如果 Phase 5+ 产品层证明 follow-up 队列必要，§2.1 `Run` 接口需扩。

3. **§2.7 ToolSchema interface 形态依赖 §4 决策** — 已落 codegen，但生成代码的具体 API 形态（Go-side union 类型如何暴露 type-switch helper）未实测。Phase 4 PRODUCE 实际生成代码后才能定型。

4. **R2 自动化降级未量化** — R2 "同义概念两个 type 名" 在自然语言层无法 100% lint，妥协为 review checkpoint + grep heuristic。如果 Phase 3/4 发现实际 PR 中 R2 违反频繁逃过 review，需回头加强（如训 LLM-based reviewer，超出 Phase 2 范围）。

5. **W3-lite cross-system 仅 3 库** — langchaingo / go-openai / openai-go，每库 ≤15min 看签名。生态代表性有限：未读 Anthropic 官方 Go SDK / Google genai SDK；未深入 langchaingo main HEAD 演进方向；未读 OpenAI Responses API（其 `previous_response_id` 可能改变 Session N/A 判断）。

6. **§5 demo case 没编译验证** — 作为 coord 不写代码。`schema.For[EchoArgs]()` 是占位假设。Phase 4 PRODUCE 必须由 builder 把 demo case 真编译跑通才算 §5 closed。

7. **G4 砍 compaction execute hook 核心假设未验证** — 已用 R2 + 量化成本论证升级（主稿 §6 G4），并显式化"未来 trigger"。但**核心假设仍未验证**：产品层 Phase 5+ 能否真用 `Provider.Complete + Transcript` 自己做 compaction 今天没有 PoC。Transcript 的 Subscribe/Slice 是为消费设计的，**写入路径是否支持产品层做 "summarize → replace tail" 操作**未验证。验证失败则 G4 决策回退。

8. **Hook 形态 v0.6→v0.7 演化留痕** — 主稿 §2.8 保留了演化记录（v0.5 双 interface → v0.6 折叠尝试 → v0.7 回滚双 interface + panic-fast）。**留痕风险**：未来 reviewer 可能误读"折叠尝试"为"折叠合理"的证据。Phase 4 PRODUCE 时若该节稳定，可考虑只保留最终决策段，把演化历程移到 ADR。

9. **B9 决策的 Anthropic 网关 caveat** — PoC 走米哈游网关。Anthropic claude-haiku 上 `oneOf` 失效未区分协议/网关问题。**Phase 5+ 触发条件**：第一个真实 Anthropic 用户报告 `oneOf`-related schema 拒绝时，立刻 ① 在官方 API 直连复测 ② 决定是否需要 Anthropic-specific runtime fallback validation 层。

## 看完之后

回主稿 `phase2-tau-design.md` 对应章节：第 #1 项 → §2.5；#2 → §2.1；#3 → §2.7+§4；#7 → §6 G4；#8 → §2.8；#9 → §4。