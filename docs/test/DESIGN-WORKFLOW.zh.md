# 测试设计工作流

[English](DESIGN-WORKFLOW.md)

先完成测试设计与评审，再实现用例。本轮初始记录由实现负责人在修改测试前自审，不冒充独立评审或用户批准。

1. 在 `docs/test/designs/` 记录稳定用例 ID、层次、产品端、前置条件、操作、可观察预期、反例/边界、计划文件与证据限制。评审后将带摘要封印的设计先提交，再修改测试。Agent 评审需如实标明；不得补造用户或同事批准。
2. 按已评审设计实现，逐项关联用例 ID 与具体测试入口。范围或预期变化时先重新评审设计。
3. 运行相关测试、门禁反例和完整核心检查。设计状态与实际通过、跳过、真机未验分别记录。
4. CI 核对目录、封印、映射和 PR 提交顺序；它不能判断文字是否真实、评审是否认真。评审者仍须核查断言质量和设计/实现一致性。
5. 存量目录只描述已有源码，不代表历史上评审或执行通过；不批量补造历史评审。

## 命令与记录格式

具体样例见 `designs/test-system-20261105.json`。每例包含 `id`、`layer`、`description`、
`setup`、`action`、`expected`、`planned_files`；描述/前提写清产品端。
评审写明实际评审者、日期、结论、理由与限制。准备中可用 `draft`，CI 会拒绝未完成评审的记录；
完成后才标为 `reviewed`。

```sh
# 设计评审完成后先提交，此提交尚不包含测试实现：
python3 scripts/test_design.py --seal-plan docs/test/designs/<change>.json
git add docs/test/designs/<change>.json
git commit -m "test design: review <change>"
# 然后实现测试，在 implementations.json 登记用例 ID/具体入口。
python3 scripts/test_design.py --refresh
make test-design
# 推送前还可检查 PR 的精确 base/head 顺序：
python3 scripts/test_design.py --pr-history --base <base-sha> --head <head-sha>
```

`implementations.json` 以源码路径映射已评审的 `cases` 和实际可执行入口的字面 `selectors`
（使用完整测试名称）。删除路径用 `removed:true` 与理由，移动同时登记新旧路径。
刷新拒绝没有评审映射的源码变更，只更新源码摘要与已登记入口；新入口明确标为
`reviewed-selector`，不冒充框架自动发现的测试声明。复核 CSV 差异，保留存量声明，
退役条目写清理由。动态模板仍是模板，不补造运行数。

封印不是签名。PR CI 读取此前提交，设计与第一次测试修改同时出现或更晚出现都会失败；
CI 评估前保留这些提交。正常 squash 后 main/发版检查最终内容，开发顺序由成功的 PR 检查证明。
不能把 main 的快照检查写成独立验证了时序。设计有变化时，先提交新的评审封印再实现，
并保留旧用例预期的可追溯记录。

修改已有测试时，不能复用完全未更新的旧评审封印：须先记录本次复核并提交。稳定用例 ID 可保留，评审理由/预期及映射须反映本次变化。
