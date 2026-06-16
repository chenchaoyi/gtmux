# gtmux 测试方案（Test Plan）

> 本文是 gtmux 的测试策略与测试设计。每次功能迭代都要**回看本方案**，按需更新测试设计、补/改用例，
> 并更新 [`DESIGN-TRACEABILITY.md`](DESIGN-TRACEABILITY.md)（设计完整跟随情况）。

## 0. 目标与分层

gtmux = cgo-free Go CLI + 原生 Swift 菜单栏 app，两者共用一套数据契约（`gtmux agents --json`）。
测试分四层，从「机器可判定」到「人工验收」：

| 层 | 测什么 | 在哪 | 何时跑 |
| --- | --- | --- | --- |
| L1 · Go 单元 | CLI 逻辑：agent 分类/排序、`agents --json` 契约、hook 状态机、ghostty 脚本、设置合并 | `internal/**/_test.go` | `make check` / CI（每 PR） |
| L2 · Swift 单元 | app 纯逻辑：**状态色=DESIGN 权威 hex**、相对时间、分组/过滤/搜索、JSON 解码、monogram | `macapp/Tests/` | `cd macapp && swift test` / CI（每 PR，macOS） |
| L3 · 一致性自检 | **设计跟随 + 架构不变量**：状态色与 DESIGN §9 一致、app 不引入 systray、app 只消费不自探测、CLI cgo-free | `scripts/check-design.sh` | CI（每 PR） |
| L4 · 人工验收 | 视觉与交互（无法机器判定）：DESIGN §13 矩阵、浅/深/着色菜单栏、键盘、i18n 即时切换、偏好设置 | 真机 macOS，对照 `docs/design/mockup/` | 发版前 + 收到设计变更时 |

L1–L3 在 CI 全自动；L4 是发版前的人工验收清单（见 §3）。

## 1. 测试设计原则

- **设计即断言**：把 DESIGN.md 里**可量化**的规范变成断言（颜色 hex、分区顺序、徽章=色+形+字形的存在性、
  相对时间格式、native/tmux 行为差异）。视觉细节（间距、材质观感）留给 L4。
- **纯函数优先**：把可测逻辑抽成纯函数（`relativeTime(_:now:)`、`AgentStore.fuzzy`、`sections(...)`、
  `Agent` 解码、`agentMonogram`），避开 AppKit/UI，单测稳定快。
- **契约锁定**：`agents --json` 的字段（含 §7 的 `source/project/terminal/tab/activity_at`）有契约测试，
  防止字段被悄悄改名/删除而打穿 app。
- **架构不变量自检**：见 L3，把「app 是纯消费方」「CLI cgo-free」「不回退 systray」做成 CI 闸门。

## 2. 持续迭代要求（每次功能迭代必做）

1. **回看测试设计**：本功能触及哪一层？是否需要新纯函数以便单测？
2. **更新用例**：新增/调整 L1/L2 用例；若引入新的 DESIGN 可量化点，加进 L3。
3. **更新跟随矩阵**：在 [`DESIGN-TRACEABILITY.md`](DESIGN-TRACEABILITY.md) 标注该 DESIGN 小节的实现/测试/状态。
4. **架构合理性 review**：确认未破坏不变量（消费方、cgo-free、终端耦合只在 `internal/ghostty`/未来 `Terminal`
   驱动里、Theme 是唯一 token 权威）。`scripts/check-design.sh` 守机器可判定的部分，其余在 PR 描述里自评。
5. **跑全闸门**：`make check` + `cd macapp && swift test` + `./scripts/check-design.sh`，CI 必绿。

## 3. 人工验收清单（L4，发版前）

对照 `docs/design/mockup/` 与 DESIGN §13 矩阵，在真机逐项确认：

- **状态项**：0（灰空心环/可隐藏）· 1 waiting（红方块+双竖线+计数，浅/深/**着色**菜单栏都可辨，非仅颜色）·
  working（青静态环，**不旋转**）· idle（绿✓）；三种显示模式（点/点+数字/空闲隐藏）切换正确。
- **popover**：分区 needs-you→working→idle→running、waiting 标题红+行淡红底；行=头像+状态徽章、session 主/
  window 次、task 省略号、相对时间、跳转记号；hover=选中；`↑↓⏎⎋`；超长 task 与 CJK 不破行/溢出。
- **跳转**：点行/⏎ → tmux 用 pane id 正确切；native 行（若有）聚焦 `terminal` app 中标题匹配 `tab` 的标签页。
- **空状态/首次运行**：文案平实无营销腔；权限卡步骤正确。
- **偏好**：语言三态**即时生效**（状态项/popover 跟随）；刷新间隔、开机自启、显示模式、通知开关生效。
- **动效**：仅 idle→waiting 一次脉冲；其余安静。

## 4. 怎么跑

```sh
make check                       # L1 Go: fmt + vet + staticcheck + race tests
cd macapp && swift test          # L2 Swift 单元
./scripts/check-design.sh        # L3 设计/架构一致性
# L4：构建 app 真机验收
make app                         # 产出 ~/Applications/Gtmux.app（或 build/Gtmux.app）
```

CI（`.github/workflows/ci.yml`）：Linux 跑 L1 + L3 的 cgo-free 断言；macOS 跑 `swift build`、`swift test`、
`scripts/check-design.sh`。
