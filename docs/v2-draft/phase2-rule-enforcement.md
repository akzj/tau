# Phase 2 — R1-R6 铁律落地实施细节

> **What**：tau 6 条永久铁律（R1-R6）在 Go 工具链中的具体落地机制——哪个 lint / 哪个 vet analyzer / 哪条 CI 脚本 / 哪类 review checkpoint。
> **来自**：主稿 `phase2-tau-design.md` §3 拆出（保留主稿 §3 的速览表，本文是详细版）。
> **Phase 3 reader 为什么要看**：Phase 3 audit `docs/01..09` 发现潜在违反 R1-R6 时，本表是判断"该违反能否被 lint 拦截"的依据；Phase 4 PRODUCE 阶段必须 PoC 实现至少一条以自验 R3。

## R3 是元规则

**C2 强约束**：没有自动化检查的红线视为不存在。R3 = "每条红线必须 lint-enforced" 本身就是元规则——它要求 R1/R2/R4/R5/R6 的落地不能停留在文档承诺，必须有具体工具链实现。

## 6 条落地详细机制

### R1 — 类型 = 运行时真相

| 子项 | 工具 | 配置位置 | 行为 |
|---|---|---|---|
| (a) 禁未 ok-check 的 type assertion | `golangci-lint` `forcetypeassert` + 自定义 analyzer | `.golangci.yml` | `x.(T)` 不允许，必须 `x, ok := x.(T)` |
| (b) 禁 `interface{}` / `any` 在公开 API 签名 | 自定义 vet analyzer | `tools/vet/noany/` | 公开函数参数/返回值禁此类型；test 包白名单 |
| (c) 跨抽象边界禁泛型擦除 | 自定义 vet analyzer | `tools/vet/notypeassert/` | 对应 A1 §5 pi 的 `as any` 反例：`tool.Schema.(*concreteSchema)` 必须有 ok 分支 |

### R2 — 单一 canonical 抽象

| 子项 | 工具 | 配置位置 | 行为 |
|---|---|---|---|
| (a) 新增公开 type 触发架构 review | PR template | `.github/PULL_REQUEST_TEMPLATE.md` | PR 描述必须答"这个概念是否已有抽象"，reviewer 必须确认 |
| (b) 同一概念多 type 报警 | grep gate | `ci/check_canonical.sh` | 关键字"Agent" / "Tool" / "Session" 等核心概念公开 type 数 >1 报警，reviewer 必须 justify |

**已知妥协**：R2 "同义概念两 type 名"在自然语言层无法 100% 自动化（grep 只能粗筛同名前缀）。降级为 review checkpoint + grep heuristic 组合，不算完全自动化。

### R3 — 每条红线 lint-enforced（元规则）

| 子项 | 工具 | 配置位置 | 行为 |
|---|---|---|---|
| 文档与 lint 同步 | CI 脚本 | `ci/check_redline_coverage.sh` | parse `docs/v2/red-lines.md` 中每条红线的 `lint-rule:` 字段，比对 `.golangci.yml` + `tools/vet/` + `ci/` 脚本是否存在对应规则 |

**红线文档格式约束**：

```markdown
## R1 — 类型 = 运行时真相
lint-rule: forcetypeassert, tools/vet/noany, tools/vet/notypeassert
```

如果某条红线无 `lint-rule:` 字段或字段指向不存在的规则 → CI fail。

### R4 — Registry scoped

| 子项 | 工具 | 配置位置 | 行为 |
|---|---|---|---|
| (a) 禁包级 mutable global state | 自定义 vet analyzer | `tools/vet/noglobalmut/` | `core/` 包内 `var x = make(map[...])` / `var x sync.Map` 等→ fail |
| (b) 强制 `New*Registry()` 工厂模式 | grep gate | CI script | `var.*Registry.*=` in `core/` → fail |

### R5 — 抽象层禁知产品

| 子项 | 工具 | 配置位置 | 行为 |
|---|---|---|---|
| 禁产品名出现在 core | grep gate | `ci/check_no_product_names.sh` | `grep -r '\b(Claude\|GPT\|Anthropic\|OpenAI\|coding-agent\|TUI\|webui)\b' core/` → fail |
| 白名单（spec 文档引用等）| 配置文件 | `ci/product_name_allowlist.txt` | 白名单文件每行一个允许的字符串路径 + 注释 |

### R6 — 设计哲学 first-class doc

| 子项 | 工具 | 配置位置 | 行为 |
|---|---|---|---|
| (a) `docs/v2/PHILOSOPHY.md` 必须存在 | CI 脚本 | `ci/check_philosophy_freshness.sh` | 文件不存在 → fail |
| (b) 改动核心抽象必须更新 PHILOSOPHY 或 PR 描述 justify | CI + review | 同上 + PR template | core/ 公开 API 改动 PR 必须有 PHILOSOPHY 改动或 PR 描述 `no philosophy change` 标记 |
| (c) PHILOSOPHY mtime 不严重落后 | CI 脚本 | 同上 | PHILOSOPHY mtime 与 core/ 公开 API 改动 mtime 比较，超阈值告警 |

## Phase 4 PRODUCE 阶段任务

**所有上述 lint/vet/grep 规则必须 PoC 实现至少一条以验证可行性**——否则 R3 自身违反"lint-enforced"。建议第一条 PoC：R5（最简单的 grep gate，1 个 shell 脚本即可）。

## 看完之后

回主稿 §3 看 R1-R6 速览表与风险预警；本文是配 §3 的详细版。