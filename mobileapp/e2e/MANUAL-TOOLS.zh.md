# 手工截图与诊断工具

[English](MANUAL-TOOLS.md)

`npm run test:e2e` 只收集 `e2e/__tests__`。RN Jest 排除这些设备用例，也排除
`capture/` 与 `diagnostics/`。后两个目录保留截图和探针能力，截图/日志不算自动回归通过。

沿用自有模拟器设置，并显式选择工具类别：

```sh
GTMUX_E2E_MANUAL=1 GTMUX_E2E_TOOL=capture npm run e2e:manual -- --testPathPatterns=hq-header-shot
```

原有 URL/token/UDID 要求仍适用。截图可能包含私有屏幕内容，证据保持私有。
知识页截图不会采纳知识条目。

发送复现要求 `GTMUX_E2E_TOOL=diagnostics`、`GTMUX_E2E_OWNED_FIXTURE=1`、
**准确的** `GTMUX_E2E_PANE`、假服务 URL/token 和自有模拟器。地址须为 HTTP 回环 origin，
health 必须标识 `gtmux-e2e-fake-serve`，带认证的 agent 列表须恰好包含一个目标 pane。
这些检查在访问设备前完成。假服务只在内存记录发送，不向真实 tmux 输入；
故意不支持把此诊断指向真实服务，也不会选列表第一行。

键盘探针用于诊断驱动输入并记录观察，不是键盘/快捷键验收断言。
执行者可核观察结果后记录人工结论，并注明源码、构建、设备和限制。
