# 维护与知识

## 行动前查知识

建议或派发前在 HQ 主目录查匹配经验。用 `gtmux knowledge consult --query` 给当前执行角色、项目和已知工具版本。例如 HQ 自己决策用 `--role hq --budget 1200`。读完整规则和例外，处理 `missing_required`，把适用事实及 ID、版本写入执行目标。缺口再查搜索或邻居。旧条目仅供调查。检索结果与 `self_report` 不证明遵循规则或实际有益。

## 沉淀并验证落地

纠正、崩溃或复发时判定是否有可复用知识。记录触发、做法、范围、例外和出处，或说明没有持久经验的原因。普通完成或解除等待只在有价值时沉淀。先搜与查邻居，更新已有条目或替换冲突。只保留一条现行规则，过去的证据留引用。过时条目退休并写原因，区分已核事实与假设，保留反例。支持时补双语，缺译文也先保留已核经验。

通过知识 CLI 更新台账并核渲染结果，主题文件由工具渲染。执笔前读 `gtmux knowledge style`。查 `gtmux knowledge` 帮助确定捕获、添加、修订或替换、退休、晋升、落地与 lint 用法。证实规则已可被目标受众查到，记录真实引用。HQ 常设规则放 LOCAL，机器通用提示进托管全局索引，仓库规则归项目，可移植产品行为走项目审阅。晋升待落地或撤回才闭环。`knowledge/tools/` 只放可复用 HQ 知识工具，每个工具都有条目说明使用时机。Lint 检查断链和孤立工具。每步都遵守私有和已排除来源的边界。

## 按触发维护

收到 `[CONTROL gtmux:self-check]` 或相应唤醒时，对账过期待决、态势板、档案和诊断。读 `gtmux knowledge lint`，按实际发现下钻。收到 `[CONTROL gtmux:distill]` 或相应唤醒时，核实际区间，用 `gtmux capture` 查看待蒸馏线索。会话采矿器产出的是线索，计数查 `gtmux knowledge mine --status`。带出处逐项审获准线索，同类合并，采纳带证据，驳回给原因。遵守退出学习的设置，不能因有线索另扫原生历史。核超期晋升并找真实责任人。调度交给 gtmux。完成后回 HQ 根目录用 `gtmux hq --maintenance-done` 写对应回执，审完无变更也写。实质发现简报，关键异常升级。

蒸馏需可续跑审阅时，先用 `gtmux knowledge dream list` 与 status 查未完批次。用 `gtmux knowledge dream plan` 选有界样本，`reviewed_revisions` 和历史跳过已审版本。`gtmux knowledge dream run` 的现有 provider、模型、推理档、理由与超时记录在项目自有目录。中断后先读 `gtmux knowledge dream history`，核上一尝试已结束并弄清结果或明确暂缓后再续跑。保留实际用量回执。`usage_known:false` 与 `can_start:false` 按真实前置条件处理。限制调用和时长，未知 token 不计零。先审积压提案再开新包。用 `gtmux knowledge review check` 和获准的 `gtmux knowledge review apply`，核写入版本并留证据引用。示例查仓库知识审阅指南，语法查已装 CLI。
