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

## 云端出包：配置完成后的跨机器默认流程

可选的 `iOS test delivery (maintainers)` 工作流使用 GitHub 的临时 macOS 环境构建、
签名并发布 Ad Hoc 测试包。配置完成后，维护者可以从另一台电脑或 GitHub 网页触发，
不必复制这台 Mac 的发布配置、Xcode 或签名钥匙串。目前流程默认关闭，只有设置
`IOS_TEST_DELIVERY_ENABLED=true` 后才会运行。合入代码不会迁移凭据、登记设备、上传
测试包或提交 App Store。首次真实出包验证通过前，继续使用下方的本地备用流程。

### 一次性配置（需审阅）

创建 `ios-test-delivery` 环境，将可部署分支限制为 `main`，要求维护者审核，按需禁止
发起者自行批准，并核查管理员绕过规则。公开仓库及 fork 的普通 PR 检查不需要这些
凭据。密钥放在环境 Secrets 中，不写进代码或 Variables。工作流只能从 `main` 手动
触发，还会检查 tag 已正式发布、提交属于主分支，以及该提交最近一次主分支 CI 已通过。UI 验证不要求发版，只接受最近一次 CI
已通过的主分支完整提交号；本流程不对未审阅的开发分支或 fork 提交签名。

| 环境配置 | 来源与范围 |
| --- | --- |
| Secret `IOS_CERT_P12`、`IOS_CERT_PASSWORD` | Base64 编码的 iOS **Apple Distribution** 证书及私钥，以及证书密码；不是 Mac 的 Developer ID 证书 |
| Secrets `IOS_PROFILE_APP`、`IOS_PROFILE_WIDGET`、`IOS_PROFILE_NOTIFICATION` | `com.gtmux.app`、`.widget`、`.notificationservice` 的 Base64 Ad Hoc 描述文件；团队一致、尚未过期、有共同登记的测试设备，主 App 使用 production APNs |
| Secret `IOS_PUBLISHER_SSH_KEY` | 专用 SSH 私钥，公钥仅作为私有 ccy.dev 仓库的**只读部署钥匙**；不使用个人 token，也不给跨仓库写权限 |
| 下方列出的五个 `BUILDS_*` Secrets | 经批准复用现有专用测试桶的发布配置，以及仅限 gtmux 的 API 密钥；每次出包无需新建桶 |
| Variable `IOS_TEAM_ID` | 签名团队的十位标识 |
| Variable `IOS_RESULT_RECIPIENT` | 维护者的 age **公钥**；解密私钥不上传 GitHub |

上传 `IOS_CERT_P12` 前，先用预期密码在 macOS 的独立临时钥匙串中验证导入。仅通过
OpenSSL 校验还不够：已复现 OpenSSL 3 默认导出的 PBES2/AES、SHA-256 MAC 格式
在密码正确时仍被 macOS 拒绝，提示 `MAC verification failed`；同一套证书与私钥
改用 macOS 的 `/usr/bin/openssl`（LibreSSL）导出后，导入验证通过。在其他环境导出时，
选择兼容 macOS 的 PKCS#12 格式并验证导入，不要为解决文件格式问题新建或吊销证书。
只有 P12 和描述文件需要 Base64 编码；密码保持原文。

启用流程前，将签名证书与私钥、密码、配套描述文件及其编号和有效期做加密恢复备份。
解密后与原文件核对，再保留一份异地加密副本，恢复密钥单独保存在批准的密钥管理工具
中。GitHub Secrets 无法读回，不能充当备份；工作流的结果产物也刻意排除了签名私钥和
描述文件。

审阅环境并通过 GitHub Secrets 页面或安全的标准输入命令配置密钥后，最后再将仓库
变量 `IOS_TEST_DELIVERY_ENABLED` 设置为 `true`。不要把密钥粘贴到对话或命令参数中。
本次实现没有迁移任何凭据。fork 的维护者如果需要云端发布，需配置自己的签名、私有
上传器访问权限及分发服务；普通构建和测试贡献不依赖这些配置。

在 **ccy.dev → Settings → Deploy keys** 登记上传器公钥，不勾选 **Allow write access**。
将专用私钥存入上述 gtmux 环境 Secret，不使用本机登录 SSH 私钥。普通 PR 与 fork
检查仍无法获得这些环境密钥。

使用 [age 官方工具](https://github.com/FiloSottile/age) 创建一次解密身份：

```sh
umask 077
age-keygen -o "$PRIVATE_DELIVERY_IDENTITY"
# 工具输出公钥，将公钥填入 IOS_RESULT_RECIPIENT。
```

私钥保存在维护者批准的密钥管理工具中，并保留恢复副本。它只能解密工作流结果，
不能用于签名或发布。另一台电脑触发工作流不需要私钥，取回结果时才需要。
丢失私钥就无法打开已有的加密回执。不要上传私钥到 GitHub；按需要轮换公钥。

### 触发与获取结果

在 GitHub 中打开 **Actions → iOS test delivery (maintainers) → Run workflow → main**，
选择用途，再批准环境部署：

- `ui-preview`（默认）：`preview_commit` 留空，测试当前主分支；也可填写某次主分支
  提交的完整 SHA，要求 CI 已通过。`release_tag` 留空。不打 tag、不创建 GitHub
  Release、不更新商店版本说明，也不提交 ASC。包沿用源码中的 App 版本，使用独立的
  `run_number.run_attempt` 构建号，安装页面和说明会标记为 UI 验证包，并注明提交号。
- `release-test`：在 `release_tag` 填入已发布的 `vX.Y.Z`，App 版本应已同步；
  使用该提交中的中英文更新说明。

也可使用：

```sh
# 不发版，进行 UI 验证：
gh workflow run ios-test-delivery.yml --ref main -f purpose=ui-preview
# 测试已发布版本：
gh workflow run ios-test-delivery.yml --ref main -f purpose=release-test -f release_tag=vX.Y.Z
gh run list --workflow ios-test-delivery.yml
# 选择本次实际 run ID，避免下载另一次发版的结果。
gh run download "$DELIVERY_RUN_ID" -n "$DELIVERY_ARTIFACT_NAME" -D "$PRIVATE_RESULT_DIR"
age --decrypt -i "$PRIVATE_DELIVERY_IDENTITY" \
  -o "$PRIVATE_RESULT_DIR/result.tar.gz" "$PRIVATE_RESULT_DIR/ios-delivery.tar.gz.age"
tar -xzf "$PRIVATE_RESULT_DIR/result.tar.gz" -C "$PRIVATE_RESULT_DIR"
```

先创建新的私有结果目录，并使用 `umask 077`。产物名称包含 run ID 和重试次数，保留
三天；公开产物仅有加密文件。解密后的 `publication.json` 中有仅限该版本、有效期
24 小时的 Safari 安装链接，私下发给测试者即可。结果也包含 IPA；可信 Mac 能连接到
已配对手机时，仍可无线直装。GitHub 的构建机器没有本机的设备配对，也不能直接连接手机。

公开日志只显示执行阶段和结果，不展示安装链接、IPA、描述文件或构建日志。失败时
保留加密日志，以及已导出的 IPA、已创建的发布回执；凭据和签名配置文件不包含在内。

导出校验先验证每个包的签名，再通过 `codesign --der` 读取签名中的权限。默认输出是
可读文本，并非 plist；较新的 iOS 导出包可能没有可用的旧 XML 权限数据。目前读取器
支持第 1 版 DER 字典，未知或损坏的格式会阻止发布。不要为绕过格式错误而跳过应用
标识、调试权限或生产推送权限的检查。

若上传成功但未生成安装链接，任务会明确失败；维护者可使用回执补发链接，无需重建。
网络中断时对象可能已经上传，应复用同一回执和 IPA，不要盲目再发一份。
若任务在加密前取消，可能没有可取回的产物，需要明确报告这个限制。

工作流采用手动描述文件签名、固定版本的私有上传器和固定提交的 Action。它不登记
Apple 设备，不提交 ASC，也不部署 ccy.dev。构建机器的 Xcode/Ruby 兼容性及真实签名、
导出流程仍需首次实跑确认，合成测试不能代替。云端发布成功也不代表 Safari 安装、
配对保留、通知/实时活动、iPad 布局或 VoiceOver 已验收。

## 2. 本地备用流程：复用配置，先检查磁盘空间

先看实际可用空间。能复用依赖和已验签的同版 IPA，就不用重建。Archive、导出目录和
DerivedData 使用本任务专用路径。仅清理本任务确认可移除的中间产物，保留 IPA 和上传回执；
不要清理其他会话的工作区、依赖缓存或签名配置。

分发服务已部署在 `https://builds.ccy.dev`，使用独立私有测试桶。每轮发版不必重新建桶、建库
或索要新密钥；音频桶和官网评论库不属于这条流程。

发布配置是**每台机器自己的私有配置**，不会随 Git 克隆或 HQ 信息同步自动出现。
默认位置为 `$XDG_CONFIG_HOME/ccy-builds/gtmux-publisher.json`；未设置 XDG 时使用
`~/.config/ccy-builds/gtmux-publisher.json`。这个路径约定不代表新 Mac 已有配置。
可以通过 `--config` 或 `BUILDS_PUBLISHER_CONFIG` 指定位置；完整的本地／CI 环境变量也可
直接使用，并优先于文件中的对应字段。`BUILDS_ORIGIN` 可指定 HTTPS 服务地址。
工具只加载发布流程规定的五个字段。密钥不进入命令参数、Git、回执、截图或公开 CI 产物；
手机输入框会保留草稿和输入历史，不用于录入密码。

### 换一台 Mac：先检查，再做一次性配置

大构建前先确定安装路径。能直接安装时，不需要上传凭据；需要网页安装或网络安装的
后备方案时，先检查发布配置，再归档。Xcode、仓库 Ruby 版本、Apple 签名授权和测试设备
登记也要分别检查；上传配置不能代替 Apple 签名配置。

在 gtmux 仓库运行以下预检，不显示密钥，不访问远端：

```sh
python3 mobileapp/scripts/test-publisher.py check
# 配置不在默认位置时：
python3 mobileapp/scripts/test-publisher.py --config "$PRIVATE_PUBLISHER_CONFIG" check
```

缺配置时，通过**已验证的 SSH 通道**或经授权的密码管理器／凭据渠道取得既有配置。
文件只交给可信的发布执行者：其中 S3 凭据能访问整个测试桶，不是仅限 gtmux。普通测试者
只需要受限安装链接。这份流程不自动开通远程登录、不新建凭据，也不授权向新的人提供密钥。

下面的命令在**新 Mac**执行。`PUBLISHER_HOST` 要填实际可访问的发布 Mac 地址或别名。
HQ 尚无已确认的发布 Mac SSH 地址记录；示例不代表两台机器已能连接。
密钥通过 SSH 标准输出直接导入，不经过命令参数或剪贴板：

```sh
# 在新 Mac 的 gtmux 仓库执行；前提是已有、已验证的 SSH 访问通道。
set -o pipefail
ssh "${PUBLISHER_HOST:?set the verified source release Mac}" \
  'cat "$HOME/.config/ccy-builds/gtmux-publisher.json"' | \
  python3 mobileapp/scripts/test-publisher.py import-config
python3 mobileapp/scripts/test-publisher.py check
```

源路径也是默认示例；原机器使用其他位置时，应填写核实后的私有路径。
导入工具会校验 JSON，以 `0600` 权限新建文件，拒绝覆盖已有配置、空输入、不完整字段和
符号链接。已授权的密码管理器也可以将 JSON 交给该命令的标准输入，或注入 `BUILDS_*`
环境变量。不要拿 VPS 凭据代替 Mac 的访问方式，也不要通过公开下载或聊天传输密钥。
没有安全通道时，明确报告这一项阻碍，保留已验签 IPA；不能因为本机缺文件就重新索要 R2
密钥或重部署 ccy.dev。

在新 Mac 选择位置准备最新 ccy.dev 仓库。上传仍使用它的既有脚本，不需要每个包改代码或
部署。将 `CCY_DEV_CHECKOUT` 设为本机仓库路径，再准备 Python 环境：

```sh
# 路径由当前机器决定，不复制别的机器上的绝对目录或 venv 路径。
BUILDS_VENV="${XDG_DATA_HOME:-$HOME/.local/share}/ccy-builds/uploader-venv"
python3 -m venv "$BUILDS_VENV"
"$BUILDS_VENV/bin/python" -m pip install \
  -r "${CCY_DEV_CHECKOUT:?set the local ccy.dev checkout}/distribution/scripts/requirements.txt"
BUILDS_UPLOADER="$CCY_DEV_CHECKOUT/distribution/scripts/builds.py"
BUILDS_PYTHON="$BUILDS_VENV/bin/python"
"$BUILDS_PYTHON" mobileapp/scripts/test-publisher.py check --uploader "$BUILDS_UPLOADER"
```

预检只证明本地字段、上传脚本和 boto3 齐备，不代表远端密钥有效、签名可用或安装成功。
这些结果分开记录；一次配置完成后，后续发版复用即可。

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

上传程序仍属于 **ccy.dev 仓库**。gtmux 的维护脚本只加载配置并保存私有结果，
不另写上传协议，也不增加 App 或 CLI 产品功能。五个 `BUILDS_*` 字段见
[发布者流程](https://github.com/chenchaoyi/ccy.dev/blob/main/distribution/README.md#发布者使用--publisher-workflow)。
下面使用预检中设定的本机 `BUILDS_PYTHON`、`BUILDS_UPLOADER`；非默认配置可通过子命令前的
`--config` 或 `BUILDS_PUBLISHER_CONFIG` 指定。

```sh
# 原上传脚本先检查包，不需要凭据，不写远端。
"$BUILDS_PYTHON" "$BUILDS_UPLOADER" --app gtmux publish "$RELEASE_IPA" \
  --receipt "$RELEASE_RECEIPT" --dry-run

# 在 gtmux 仓库执行。结果／日志路径必须尚不存在，父目录应已准备好。
# 文件权限固定为 0600，不依赖 shell 的 umask。
"$BUILDS_PYTHON" mobileapp/scripts/test-publisher.py run \
  --uploader "$BUILDS_UPLOADER" --output "$RELEASE_PUBLICATION_RESULT" \
  --log "$RELEASE_PUBLICATION_LOG" -- \
  publish "$RELEASE_IPA" --name gtmux \
  --notes-zh "$RELEASE_NOTES_ZH" --notes-en "$RELEASE_NOTES_EN" \
  --receipt "$RELEASE_RECEIPT"
```

上传脚本的标准输出和错误输出都存为私有文件，不直接进入 agent／CI 日志。
执行者本地读取结果，将安装链接仅交给目标测试者。维护脚本保留上传程序的退出码；
失败时保留输出、IPA 和回执，不把上传状态误报为成功。重试时结果和日志换新路径，IPA 和
回执继续使用原来的。版本已发布、只是链接过期时，可用同一入口执行
`link --build <id> --hours 24`。

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

### 恢复备份与续期维护

加密恢复包应包含：兼容 macOS 的签名 P12 及密码、证书与私钥、三份 Ad Hoc 描述文件及
有效期台账、上传器只读 deploy key、App 专用上传/R2 配置和交付结果解密私钥。
外层恢复钥匙单独存入批准的密码管理器；不能只把解密钥匙放在它自己加密的包里。
账户登录和 MFA 恢复凭据仍由账户持有人保管，不包含在这份发布备份中。

清理本任务的明文临时副本前，先解密备份并逐文件核对哈希，再保留异地加密副本。
包内 README 应写清首次配置、UI/正式版本出包、结果解密、安装、Secrets/Variables
对照及续期。换机时由操作者选择实际私有目录，不能把原 Mac 的绝对路径当成前提。

每次发版前及每月检查证书和**每一份描述文件**的有效期。按最早到期日提前 60 天开始
续期检查，提前 30 天完成替换验证。签名证书或登记设备变化后，重建三个 bundle 的描述
文件，验证 macOS P12 导入，更新受保护环境，跑通 UI 包并验收安装，再处理旧凭据。
上传/API 密钥按服务方的真实有效期维护，不假设统一到期日。轮换结果公钥后，旧私钥仍
需保留，以解密历史回执。

签名、描述文件、上传凭据、deploy key、结果公钥或工作流参数变更时，同步更新本中英文
文档、私有台账、恢复 README、校验清单、提醒和加密备份。回执中区分“已导出提醒”与
“已设置提醒”；当前流程不会自动续证。GitHub Secrets 无法读回，不能替代备份。
