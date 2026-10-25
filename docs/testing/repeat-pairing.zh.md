# 重复扫码配对审计（2026-10-25）

重复扫描新有效码可以安全刷新已保存连接；重复帧/点击只兑换一次。写入失败保留旧连接，支持直接重试保存；取消和旧凭据响应不能切换或污染新连接。代码基线为 `d5029932`，分支 `fix/repeat-pairing`。本次未发布、安装或修改版本。

## 实现与身份

iPhone/iPad 使用同一个 React Native `PairingScreen` 和 `ScanScreen`（camera-kit），并非 SwiftUI。扫码解析 v1 token、v2 一次性 owner code、pair URL 以及 guest share URL；owner code 在 Go `POST /api/enroll` 兑换为新的 device token。服务器显示名、手机设备名、device ID、Direct 地点 `route.id` 都不是服务器身份。

当前协议没有全局稳定的 server UUID。客户端以协议、主机、有效端口、路由路径组成的规范化端点为键，另接受此前经鉴权 `/api/addresses` 学到的备用地址。仅消除尾斜线、主机大小写、显式标准端口等价拼写。不同协议、非标准端口、路径及未经证明的 DNS/IP/主机别名保持独立；不会把旧 token 发往陌生扫码地址探测归属，也不按名字合并。同一实际服务器的陌生别名仍可能形成两个条目，这是现有身份协议的边界。

Go 当前实现确认：owner code 一次性、5 分钟 TTL、仅存内存，serve 重启失效；并发由锁保证只有一个兑换成功。每个新有效 owner code 创建新的 device ID/token，旧 token 不会自动撤销。设备列表可能保留多次配对的设备记录，不能用相同手机名称合并或撤销。客户端条目去重不改变服务器设备记录语义。

## 根因与场景结果

| 场景 | 修复前证据 / 根因 | 修复后与验证层级 |
| --- | --- | --- |
| 在线服务器重扫新码、同地址 | upsert 本来替换 URL，已有命名/通知测试正确；但覆盖对象丢备用地址和 route；成功兑换未清 busy，原地址不会触发根节点换页 | 保留配置和顺序；成功通知父页关闭添加页。组件与 store 回归 |
| 同一码再次扫描、已用/过期码 | 服务端已正确拒绝，不应客户端绕过 | 401 提示在服务器生成新码；中英文组件验证、真实 HTTP 验证 |
| 连续扫码帧/手动双击 | 相机已有 ref 防重；配对页各入口没有共同同步锁，React busy 尚未渲染时可并发 | 共用 ref admission，同帧只兑换/保存一次；扫描页重开重建，取消后旧帧无效。注入相机回调/组件验证 |
| URL 等价与 Direct 备用入口 | 精确字符串键生成多个 entry，重启也未去重 | 等价 URL 与已知 owner 备用入口去重，修复选中项；重启去重优先保留选中条目的凭据与配置，不把用户行序当成 token 新旧顺序；保留源配置；自动迁移到已存在地址也不重复。纯函数/provider 回归 |
| 同名不同服务、不同端口/路径/协议、陌生别名、guest 备用入口 | 无安全身份依据，不能根据名称合并 | 保留独立条目；不借 owner 的备用地址合并 guest。纯函数及另一服务拒绝 token 的真实 HTTP 验证 |
| 服务端已成功但响应丢失 | 一次性码已消费；旧 UI 的 5xx 文案误承诺配对码仍有效 | 不自动重复兑换，不放宽使用次数；新码恢复，旧有效凭据不撤销。真实 HTTP（提交后不读响应）及组件验证 |
| Keychain 拒绝或返回 false | pair 先 setState 后保存；基线断言旧 token 应保留却得到 fresh；false 被当成功 | 写完一个含列表和选中项的 blob 才应用；失败不切连接，当前页暂存已兑换凭据以「重试保存」，无需重复兑换。provider/组件验证，原生 Keychain 未实测 |
| 同一 render 并发保存、设置更新 | 闭包读旧 servers；基线新增 Studio 后被下一次 pair 丢掉 | 所有列表操作排队读取最新已提交快照，失败不毒化队列；迟到的已取消 pair 入队后再次核对来源。provider 回归 |
| 写入或重启中断 | 单个 Keychain blob 原本已包含服务器和选中项；内存先改导致失配 | native 写成功前保留旧内存；写完后重启读取同一个 blob。写入提交期间取消不可用。模拟持久化/重启验证；进程被杀及原生 Keychain 原子性未实测 |
| 网络断开/超时/502/无效正文 | 已有头部超时与错误分类正确；但收到响应头即清 timer，正文可永远挂起 | 总计 15 秒覆盖正文且 abort；坏正文/无 token 提示新码；5xx/传输失败提示检查服务与入口，不保证旧码未用。fake timer/fetch 回归 |
| 取消、关闭 sheet、切换目标、重进 | 缺少操作生命周期检查；隐藏 Modal 不保证卸载子页面 | 失效操作在保存前及队列执行时检查；关闭显式卸载；重进使用新 scanner session。注入回调/provider/组件回归 |
| 同地址换 token 的连接/重连 | SSE 本来会清理，但旧 HTTP 或排队 SSE 回调仍可改新状态；基线 live 被旧 401 改 unauthorized，反向也能错误清掉失败 | 按 client 对象（含 URL/token）和 mount 生命周期隔离；权限快照也绑定 client，新 guest 凭据查询失败时不继承旧 owner 控件；关闭旧流，迟到失败/成功/事件均无效。连接 provider 回归 |
| 手机/iPad、横竖屏反馈 | 小屏横屏 256pt 扫码框加控件所需空间超过 320pt 高度 | 响应式扫码框，短横屏保留标题/取消、隐藏次要说明；320×568、568×320、393×852、852×393、768×1024、1024×768 结构约束及中英文反馈组件验证。未做原生截图或相机识别验收 |

基线先运行新增 store/QR/provider 回归：7 失败、75 通过。旧凭据来源隔离单独回归：2 失败；同地址 owner→guest 且权限查询失败的补充基线回归也失败。另以基线的原始 PairingScreen/ScanScreen 运行新 UI 回归，确实失败；只以独立业务断言作为缺陷证据，未把基线测试的后续级联错误算作额外缺陷。临时还原仅发生在本 worktree，以 `command cp -f` 覆盖并逐次 `cmp` 核对，随后恢复修复版。

## 可重复验证

```sh
cd mobileapp
npm run check -- --runInBand
npm test -- --runInBand src/pairing src/screens/PairingScreen src/screens/ScanScreen.repeat.test.tsx src/state/AppContext.order.test.tsx src/state/AgentsContext.repair.test.tsx
cd ..
go test -race ./internal/server -run 'TestRepeatPairing|TestEnroll' -count=1
GOFLAGS=-p=2 make check
./scripts/check-design.sh --source-only
```

最终 `npm run check -- --runInBand` 通过：189 个套件、1928 个测试通过，15 项外部条件测试跳过；包含 TypeScript 和 ESLint（0 error，仓库允许既有 warnings）。最终 source gate 通过。首次 mobile 完整门禁 188 suites / 1911 tests 通过，15 个外部条件测试跳过；之后补充回归再跑最终版本。完整 `GOFLAGS=-p=2 make check` 已通过，包含 fmt、vet、staticcheck、全仓 race、75 项 publisher 测试与 44 项 CI/策略测试。首次无限制并发 core 门禁的三个现有严格计时用例曾失败（brew 5s、native hook 5s、version probe 2s），限制包并发后的完整复验通过，未修改阈值。source gate 的私有精确词表未提供，报告相应检查跳过；其他源码、双语、规格检查通过。

所有服务、码和 token 均为测试合成数据。HTTP 测试使用生产 Go handler/EnrollManager 和随机 loopback 端口，不运行用户 serve/tunnel，不操作用户设备、凭据或其他 checkout。RN 组件测试 mock 了原生 Keychain、摄像头和 fetch；HTTP 测试与组件测试是两个层级，不构成真机端到端证据。没有执行摄像头真实扫描、真实 iPhone/iPad UI、原生 Keychain 重启/中断、系统级断网/502、原生 SSE 或 APNs 验收；没有安装客户端，也没有碰其他执行者模拟器。
