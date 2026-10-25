# 私有存储写入入口

gtmux 管理 `state.Dir()`、`state.ConfigDir()` 两棵目录；App 对应
`Paths.dataDir`、`Paths.configDir`。`WriteFile` 会保留旧 inode 的 mode，chmod 不受
umask 约束，rename 则保留源 mode。因此每个自有 writer 都要在写入处保证权限。

| 入口 / 目标 | 期望策略 | 证据 / 处理 |
|---|---|---|
| CLI 附件、Mac 截图发送、HTTP 上传 → `uploads/` | 临时文件私有，原子发布 0600 | 已统一 `WritePrivateFile`；`TestUploadWritersKeepPrivateModes` 覆盖三种 umask 和旧宽权限副本 |
| `usage.saveDaily`、`radar.BuiltinIconPath` → 日账本、图标缓存 | 原子私有发布；缓存命中不反复写盘 | 修复固定 `.tmp` 带入旧 mode、同内容宽权限缓存直接复用；两个入口测试修前失败、修后通过 |
| marker、CPU/frame、tab order、运行戳 → data root | 新建私有；覆盖前收窄旧 owned inode | 接入 `state.WriteFile/OpenFile/MkdirAll`；`TestMarkerWritersAreLocallyPrivate` 修前失败 |
| 配置、设备/分享/推送名单、远程及隧道凭证、状态 → 两棵管理目录 | 数据 0600、目录链私有，序列化不变 | 接入共享 IO；`TestConfigurationAndCredentialsArePrivateOnRewrite` 检查新建、旧宽权限目标及 JSON |
| events/序号、advice、mine 历史、OpenCode 镜像、wake tally | 私有 append，保留 inode、O_APPEND、O_EXCL、flock | `TestJournalAndSequenceArePrivateBeforeAppend`、共享 IO 语义测试及原有 events 并发测试 |
| HQ 种子/备份、知识渲染、通知队列、dispatch 记录 | 写入和暂存私有；工具保留所有者执行位 | 接入共享 IO，保留原子系统测试及源码入口门禁；字节复制不带入源 mode，HQ 恢复仅保留所有者执行位 |
| native/resume/session-policy/client 存储及锁 | 私有暂存、覆盖、锁；格式与并发语义不变 | 仅做 IO 包装替换，保留原有存储、投递、scope、生命周期测试 |
| Go status/relay/profile/account、迁移及导出暂存 | 原有独占 0600 临时文件，父目录链也私有 | 保留原子算法；`state.CreateTemp/MkdirTemp` 补管理目录链检查 |
| Swift 日志 append → `logs/` | 目录链及 inode 在 append 前私有；拒绝链接 | `PrivateLogFile`；新增日志测试修前有七项断言失败 |
| gtmux DSH terminal 插件 → terminal clients/input | 独占 0600 JSON/claim，owned 0700 目录；拒绝不安全根目录 | 保留原有原子/独占叶子写入，补根目录检查；Node 夹具覆盖宽 umask 与宽根目录。虽由 agent 托管，仍算 gtmux 自有代码 |
| launchd/systemd 输出 → 管理目录 `logs/` | 服务管理器打开 stdout/stderr 前准备 0600 文件 | 生成服务描述时仅准备管理目录内捕获文件；测试确认旧输出不丢失，不启动服务 |
| 外部 agent/shell/tmux/Warp 配置、用户指定导出、可执行文件安装 | 沿用外部权限策略；需要时新建为常规 mode / 可执行 0755 | 管理目录外保留原 OS 语义；外部配置、共享 IO、安装及服务测试保留 |
| doctor 探针、生成源码、特权系统 guard | 只读探针不能顺手修复；源码和系统资产另有策略 | `TestRawWritersHaveAnExplicitBoundary` 列出每个原始 OS 调用例外；私有目录外的 root guard 脚本/plist 明确为 0755/0644 |
| agent 直接写 HQ notes/reports、OS 偏好/钥匙串、App 图片导出及个人系统临时目录 | 写入者或存储权限归属不同 | 不凭路径把 agent 文档归因于 gtmux；截图导出行为不变，经 `--attach` 导入时不带源 mode；不批量迁移 home |

共享机制只封装 `MkdirAll`、`OpenFile`、`WriteFile`、`CreateTemp`、`MkdirTemp`，
复用目录 fd 与 owner 检查。append/原地覆盖保留 inode；原子发布继续换 inode。
普通数据新文件 0600、新目录 0700，旧 owned 对象在写入前去掉 group/other 位，
原地收窄不增加 owner 位，原子发布的新数据 inode 使用 0600。管理目录外不改变原 OS 行为。管理目录内的符号链接、特殊文件和
其他 owner 明确报错。源码门禁阻止未分类的 Go 原始写入调用和宽权限 chmod 字面量新增；
它与运行时回归互补，不冒充对所有程序行为的证明。

本机 APFS 上，持有有效父目录 fd 的并发原始 `O_CREAT|O_NOFOLLOW` 仍复现了 `ENOENT`。
只对创建操作的 `ENOENT` 在同一 fd 上重试，最多八次；其他错误立即返回。
`TestPrivateCreateFailureBoundaries` 确认父目录被删、权限不足仍报错；原有并发序号测试验证成功路径。

此保证只针对 POSIX mode，不审计或移除继承 ACL；agent sandbox 的目录授权是另一套机制。
未被 writer 触及的旧文件不批量迁移。符号链接化的管理存储现在可能明确拒绝。
本变更不包含安装、服务重启或发布。
