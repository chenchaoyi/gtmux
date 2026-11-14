# KB 修订的来源继承

[English](kb-source-lineage.md)

普通 supersede 与 review apply 都从当前仍存活的前项继承未提供的
`seq_range`、`seqs`、`capture`、`pane`、`task`、`legacy`、`migration`。
新操作在既有账本锁内持久化这些字段；重放遵循同一规则，不重写历史，
不回查已退休修订。后项显式非零值优先，legacy 来源标记继续保留。
继承的序号切片独立存储；原有 Sources 仍按候选 ID 合并并保留完整内容。

`seq` 仍是新操作事件水位，`candidate_result` 是当前操作收据，不继承。
审查证据、理由、时间和操作 ID 属于新修订。provenance、tags 与最新 hit
历史继续按原规则折叠。摘要、CAS、原子提交、幂等重试和只读 check 不变。
普通 supersede 仍重新判断分发；review 保留受众、状态和保护并重新打开
落地状态；继承行动卡转为 tentative。普通 supersede 仍重置译文；review 对
省略/null 的译文完整保留，并在同一事务中验证显式替换。新的 review 准备操作
显式保存保留的译文，重放读取已记录的值；历史省略值仍保持当时的清除语义，
尊重显式 ledger 清除和普通重置。保留内容不证明双语语义
一致，详见[审阅契约](../guides/knowledge-review.zh.md)。

测试只用隔离 HOME 的合成数据，经过实际 CLI、apply、show 与 render；
不把真实提案或私有会话正文加入测试。设计见
[KBL-001–004](../test/designs/kb-source-lineage-20261113.json)。
