# 软件通用工程 Skill 实施计划

**目标：** 将本项目的需求收敛、契约保护、分层实施、真实验收和 Git 交付经验整理为可移植的 skill，并按用户授权添加到当前 GitHub 仓库。

**架构：** 使用一个精简入口，按需读取验收清单和报告模板。保留决策方法，不绑定业务域编号、技术栈、历史迁移号、账号或服务地址。

**格式：** Markdown、YAML、JSON。使用内置 skill 校验器，不改动产品代码或依赖清单。

## 已确认的边界

- 用户已选择通用工程方法，已确认前一轮设计。
- 用户已确定中文名称为“软件通用工程”，英文调用名为 `software-general-engineering`。
- GitHub 目标沿用已确认方案：当前仓库的 `.agents/skills/` 目录。
- 在当前会话内完成准备、校验、提交和推送；不再次要求用户重复确认已授权方案。
- 保留起始时的三个未跟踪计划文件，不打包、不暂存、不修改。

## 来源与抽象原则

抽取依据为当前提交 `45fb16355f21a3c8a52653e2f0a520c881780f3d` 的以下材料：

- `docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md`：业务归属、窄接口、分层调用和运行入口。
- `docs/DEVELOPMENT_DISCIPLINE.md`：状态推进、幂等、迁移、配置快照、证据和提交纪律。
- `scripts/test-backend-safe.cjs`、`package.json`：区分隔离测试入口与真实集成测试入口。
- `docs/P0-4_MEDIA_ASSETS_DRIFT_ROOT_CAUSE_ANALYSIS_v1.0.md`：启动时修改 schema、真实库漂移及最小修复边界。
- `docs/UI-21B-2C-TRADE-WRITES-ACCEPTANCE.md`、`docs/UI-21B-2C-TRADE-WRITES-RESIDUE.md`：跨端真实链路、无法验收项和资源残留。
- `docs/RELEASE-CANDIDATE-2026-08-14.md`：代码候选、构建产物、线上健康、真机验收和平台发布分别判断。

这些材料是流程来源，不作为当前线上状态或当前全量测试已经通过的证据。旧报告中的环境、账号、对象标识和具体业务规则不进入可分发 skill。

## 文件职责

| 文件 | 职责 |
| --- | --- |
| `.agents/skills/software-general-engineering/SKILL.md` | 触发范围、任务边界、契约、实现、验收、交付主流程 |
| `.agents/skills/software-general-engineering/agents/openai.yaml` | 可自定义的显示名称与默认调用提示 |
| `.agents/skills/software-general-engineering/references/acceptance-checklist.md` | 按改动风险选择检查，明确证据能证明什么 |
| `.agents/skills/software-general-engineering/references/report-template.md` | 轻量交付、跨端验收、阻断和残留报告模板 |
| `.agents/skills/software-general-engineering/evals/evals.json` | 三个自包含的行为测试场景，包含预期判断 |

## 执行清单

- [x] 核对当前分支、HEAD、工作树和原有未跟踪文件。
- [x] 读取来源材料，区分通用原则和本项目限定规则。
- [x] 核实官方 skill 格式支持独立的显示名称与调用名称。
- [x] 写入入口、显示配置、两个参考文件和三个测试场景。
- [x] 运行内置 `quick_validate.py`，核对 YAML、JSON、相对链接和名称一致性。
- [x] 检查分发文件中是否混入具体账号、地址、历史数据或未完成占位内容。
- [x] 由用户给出最终名称后，同步目录、frontmatter、显示名称、调用提示和测试元数据。
- [x] 再次校验，仅暂存本任务文件，审查暂存差异并提交。
- [x] 推送当前分支并比对远端提交，确认 GitHub 文件位置。

## 验证标准

使用 UTF-8 模式运行内置校验器：

```powershell
$env:PYTHONPATH = (Resolve-Path '.codex-tmp/skill-validator-deps').Path
python -X utf8 C:\Users\33141\.codex\skills\.system\skill-creator\scripts\quick_validate.py .agents/skills/software-general-engineering
```

预期输出 `Skill is valid!`。另外解析显示配置和测试 JSON，检查所有引用目标存在、名称一致且无占位内容。测试场景只准备输入及预期；未实际运行独立模型时，不宣称行为评估通过。

本任务没有产品代码变更，不重复运行产品全量构建，也不连接数据库、云服务或发布平台。GitHub 推送完成须以远端分支指向本次提交作为证据。

## 本轮实际结果

- 内置校验器：`Skill is valid!`。
- YAML、JSON、名称一致性、三个场景完整性、相对引用和文件空白检查通过。
- 可分发目录共 5 个文件，入口 101 行；项目标识、私人路径、明显凭证赋值及未完成占位扫描无匹配。
- 行为场景已准备，独立模型评估 `NOT_RUN`，不将预期结果称为已验证行为。
- 现有已跟踪产品文件无差异，三个原有未跟踪文件仍在。
- `PyYAML 6.0.3` 仅安装于被忽略的 `.codex-tmp/skill-validator-deps`，用于本地校验；未改产品依赖。
- Skill 提交：`ef7b21d6c4874b2705f23e20501c6b00a160ae2c`，提交说明为“技能：新增软件通用工程”。
- 已推送分支 `codex/rider-refresh-home-feed`，`git ls-remote` 返回的远端提交与本地 skill 提交一致。
- [GitHub 中的软件通用工程](https://github.com/YRZ-GUANGNAN/zhanke-shenghuoquan/tree/ef7b21d6c4874b2705f23e20501c6b00a160ae2c/.agents/skills/software-general-engineering)。
- 本记录单独提交，保留 skill 本体提交的五文件范围；不包含原有三个计划文件。
