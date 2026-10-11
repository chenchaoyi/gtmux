# iOS 测试包安装

维护者流程 · [English](./DISTRIBUTION.md)

用户授权发版并需要安装手机测试包时，先检查能否直接通过网络连接已配对的手机，能连接就
无线安装。无法直连时，再通过
[ccy.dev 独立分发服务](https://github.com/chenchaoyi/ccy.dev/tree/main/distribution)
上传 IPA、生成受限安装链接。上传完成后，安装不依赖 Mac 在线，也不用经过 gtmux 隧道。

## 1. 优先无线安装，无法直连时提供安装链接

- 只改 CLI 或 Mac 的版本，不一定需要新手机包。核对手机代码和 API 兼容性，按用户要求交付。
- 网络里能找到并连接已配对的目标手机，就直接无线安装并核对版本；否则上传 Ad Hoc IPA，
  交付网页安装链接。不要求用户每轮自己选择。
- 两条路径复用同一份已验签的 Ad Hoc IPA。用户明确要求验收网页安装流程时，直接上传；
  明确要求开发包时，使用 [README.md](./README.md) 的开发构建，推送走 sandbox。
- App Store/TestFlight：遵循 [submit.md](../docs/appstore/submit.md)。上传 Ad Hoc 包不等于上传 ASC 或提交审核。

沿用分支 → PR → CI 通过 → 合入 → 带用户更新说明的 tag；同步版本号、更新说明和官网文档。
CLI 与公证后的 Mac App 两项 release job 都完成后再运行 `gtmux update`，分别核对安装版本。
文档流程更新本身不触发新发版。

## 2. 复用配置，构建前检查磁盘

先看实际可用空间。能复用依赖和已验签的同版 IPA，就不用重建。Archive、导出目录和
DerivedData 使用本任务专用路径。仅清理本任务确认可移除的中间产物，保留 IPA 和上传回执；
不要清理其他会话的工作区、依赖缓存或签名配置。

分发服务已部署在 `https://builds.ccy.dev`，使用独立私有测试桶。每轮发版不必重新建桶、建库
或索要新密钥；音频桶和官网评论库不属于这条流程。

本机发布配置位于 `~/.config/ccy-builds/gtmux-publisher.json`，权限为 `0600`，只有文件所有者
可读写。配置包含 App 范围的发布密钥与测试桶 S3 凭据。由执行者在本机读入上传脚本的环境，
不打印，不写入命令参数、PR、回执、截图或公开 CI 产物。配置可用时，不再要求用户提供密钥。
新密钥优先通过安全渠道保存；手机 App 输入框会保留草稿和输入历史，不适合录入密码。

首次缺少 S3 凭据时，在 Cloudflare 的 R2 token 管理页创建 **Account API token**，权限选
**Object Read & Write**，仅开放专用测试桶。Wrangler OAuth 不是这把 S3 密钥；首次使用 REST
上传时未保留所需 SHA 元数据，因此不能绕过校验强行发布。密钥到期和替换方式见分发服务手册。

## 3. 归档并导出

从目标 release commit 构建。使用 `scripts/set-version.sh` 同步版本与中英文更新说明，再跑
仓库要求的检查。遵循仓库 Ruby/CocoaPods 配置，导出时让 `/usr/bin/rsync` 排在 Homebrew
版本之前；签名及 PATH 的常见问题见 [ASC 流程](../docs/appstore/submit.md)。

目标使用通用 iOS 平台，归档不需要手机连接或解锁。分发包使用 production APNs，不沿用
开发包的 `APS_ENVIRONMENT=development`。

```sh
# 在 mobileapp 目录执行；变量和路径使用本次已授权发版的值。
xcodebuild -workspace ios/GtmuxMobile.xcworkspace -scheme GtmuxMobile \
  -configuration Release -destination 'generic/platform=iOS' \
  -derivedDataPath "$RELEASE_DERIVED_DATA" -archivePath "$RELEASE_ARCHIVE" \
  -jobs 2 DEVELOPMENT_TEAM="$APPLE_TEAM_ID" CODE_SIGN_STYLE=Automatic \
  MARKETING_VERSION="$RELEASE_VERSION" CURRENT_PROJECT_VERSION="$RELEASE_BUILD" \
  APS_ENVIRONMENT=production -allowProvisioningUpdates archive

xcodebuild -exportArchive -archivePath "$RELEASE_ARCHIVE" \
  -exportPath "$RELEASE_EXPORT" -exportOptionsPlist "$RELEASE_EXPORT_OPTIONS" \
  -allowProvisioningUpdates
```

无界面自动签名需要 ASC 身份时，**归档与导出两条命令都要带上**既有认证参数。
本机导出配置使用 `method = release-testing`（当前 Xcode 的 Ad Hoc 类型）、
`signingStyle = automatic`、已有 team ID、`destination = export`，以及
`manageAppVersionAndBuildNumber = false`。保留已设定的 build 号，不拿 App Store IPA 做直接安装。

上传前，核对主 App **和两个扩展**的 bundle ID、版本/build、严格验签、分发描述文件有效期、
测试设备登记、`get-task-allow = false`，以及主 App 的 `aps-environment = production`。
上传脚本只检查主 App 描述文件，不能替代扩展验签。记录 IPA 大小与 SHA-256；不把 UDID 或
完整描述文件写入仓库和公开回执。

## 4. 能直连时无线安装

设备发现与连接检查应有超时。`idevice_id -n` 枚举网络设备，`idevice_id -l` 枚举 USB 设备；
Xcode/CoreDevice 记住了设备，不代表此刻能连接。核对已配对的目标设备，不把多个 UDID
一起传给安装命令，也不自动给所有发现的设备安装。

```sh
idevice_id -n
# 核对目标设备并确认它能够响应后：
ideviceinstaller -n -u "${TEST_DEVICE_UDID:?select the paired network test device}" \
  install "$RELEASE_IPA"
ideviceinstaller -n -u "$TEST_DEVICE_UDID" list -b com.gtmux.app \
  -a CFBundleShortVersionString -a CFBundleVersion --xml
```

通过执行工具设置超时，不编造安装命令不存在的超时参数。发现或连接失败时，直接走下文
上传流程，不等待 USB、不反复轮询、不要求用户到 Mac 旁边。安装已开始但命令超时时，
先核对一次结果与设备上的版本，不能把结果未知直接当成失败；核对仍失败时，注明状态
未知，并提供同一 IPA 的安装链接供恢复。枚举到设备或发出命令都不等于安装成功。
布局和设备能力验收可能需要解锁，应与安装分开记录。

## 5. 无法直连时通过 ccy.dev 上传

日常发版复用已部署的分发服务，运行现有上传工具即可。每个测试包不需要联动修改 ccy.dev
代码、提交 PR、部署官网或重新部署 Worker；只有分发服务自身变更才需要更新该仓库。

在 **ccy.dev 仓库**运行既有上传脚本。环境变量和私有配置加载示例见
[发布者流程](https://github.com/chenchaoyi/ccy.dev/blob/main/distribution/README.md#发布者使用--publisher-workflow)。
这不是 gtmux 命令，也不通过 gtmux 隧道上传。

```sh
# 先检查包，不需要凭据，不写远端。
python3 distribution/scripts/builds.py --app gtmux publish "$RELEASE_IPA" \
  --receipt "$RELEASE_RECEIPT" --dry-run

# 已在本机加载发布环境。标准输出含受限安装链接，保存为 0600 私有结果文件。
umask 077
set -C  # 使用新的私有结果路径，拒绝覆盖已有文件。
python3 distribution/scripts/builds.py --app gtmux publish "$RELEASE_IPA" \
  --name gtmux --notes-zh "$RELEASE_NOTES_ZH" --notes-en "$RELEASE_NOTES_EN" \
  --receipt "$RELEASE_RECEIPT" > "$RELEASE_PUBLICATION_RESULT"
```

脚本先保存回执，再上传，核对 R2 大小和 SHA 元数据，确认版本发布后生成**仅限这一版、
24 小时有效**的链接。结果文件保持私有，链接只交给目标测试者；不放进 GitHub、官网导航
或 HQ 报告。

上传或发布中断时，使用**相同 IPA 和回执**重试。版本已发布但链接创建失败或过期时，运行
`link --build <id>`，不为续发链接重建或重传包。回执对应对象的大小或 SHA 不一致时，不覆盖
对象；不同产物或元数据使用新回执。`--all-builds` 会开放同一 App 的其他及后续版本，需明确要求。

## 6. 验证并交给测试者

检查线上中英文页面、同源 HTTPS 安装清单、bundle/build、IPA HEAD 与大小、完整下载 SHA、
Range/If-Range；无授权发布请求应被拒绝，单版本链接不能选择其他版本。改动授权或下载实现
后，用独立测试链接验证撤销：页面、清单、IPA 的新请求都应被拒绝，保留用户安装链接。
没有改协议的日常出包，不必每轮重跑完整协议测试。

交付链接时说明版本/build、到期时间和时区。测试者在**已登记的 iPhone/iPad 的 Safari**里
打开链接，点击「安装测试版」，确认系统提示后打开 App。不要先删除旧 App；验收远程流程时，
不要改用 USB 安装代替。

分别记录以下证据：

| 证据 | 能证明什么 |
| --- | --- |
| CI、导出签名和描述文件检查 | 构建与签名检查通过 |
| 安装页、清单、下载检查 | 发布与下载协议可用 |
| 用户确认安装成功 | 该设备可安装；未单独核对版本时要注明 |
| App 内版本、原有配对与数据 | 更新版本正确、数据保留 |
| 实际通知和 Live Activity | 本次签名包的对应设备能力 |
| iPad 布局、VoiceOver | 需分别进行的平台和读屏验收 |

2026-10-11，v1.0.104（104）完成 S3 发布和线上协议验证，用户确认 iPhone 安装成功。
这不代表数据保留、APNs、Live Activity、iPad、VoiceOver 或完整 zone/CDN/Logpush 日志检查
已通过。后续回执继续区分这些结果，并报告磁盘剩余空间。完整 zone 日志核实仍是运维待办；
Worker 配置已关闭 observability，应用代码不记录安装链接。
