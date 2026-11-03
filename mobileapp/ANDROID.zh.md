# Android

[English](ANDROID.md)

Android 与 iPhone、iPad 共用 React Native 页面、配对存储和 API 客户端，包含对话、终端、HQ、Trace、Diff 和代码阅读。Kotlin 负责文件传输与预览、系统分享、触感、硬件键盘、FCM 通知和桌面组件。会话身份、文件授权和访客权限仍由 Go 服务端判断。

## 安装与连接

签名 APK 支持 Android 7.0／API 24 及以上的 arm64、x86_64 设备。AAB 用于上传 Google Play，不能直接安装。

在 Android 浏览器中打开收到的私有链接，下载并打开 APK。系统如询问安装来源，只允许当前浏览器即可。更新时保留原 app 和配对数据；遇到签名冲突应联系发送者，不要先卸载，也不需要关闭 Play 保护机制。

配对沿用二维码、`gtmux://` 链接和地址／token 表单。已配对的局域网 HTTP 地址可继续使用；远程 HTTPS 使用正常的证书校验。只有打开扫码时才申请相机权限，拒绝后仍可手动配对。访客权限、输入保护和本地文件授权均沿用现有规则。

## 平台行为

- 手机和平板使用同一套响应式布局。Android 返回键关闭当前弹层或返回上一级；硬件 Ctrl 快捷键复用共用键表。输入框保留方向键、回车及输入法的正常行为。
- 文件保存在私有临时目录，校验会话、home、大小与 SHA-256。HTML 默认静态显示，仅在用户明确同意后运行当前文件的脚本；页面仍不能联网、读取任意文件、嵌套网页或调用原生 JS 接口。本地图片沿用 iOS 的父文档资源接口。分享只授予单个 `content://` 文件的读取权；关闭预览后清理该目录。
- HQ 副本下载完成后原子替换旧副本，可通过系统分享导出。Android 系统／云备份处于关闭状态；本机有副本不代表已有云端备份。导出的文件未加密，请妥善存放。
- 用量组件保存所选自有连接的“上次查看”快照，不含凭据。前台 app 负责刷新，点击组件打开用量详情。切换或移除连接、查看访客连接时会清除旧快照。
- 后台通知采用可选的 FCM，需要安装包中的 Firebase 客户端配置、relay 单独保存的服务账号、设备上的 Google Play 服务，以及系统通知权限。设置页会说明具体缺哪一项。没有 Google 服务时，其余功能仍可使用。通知点击后打开当前会话；Android 不会从可能过期的通知中直接发送审批数字，应在当前问题里操作。真实 FCM、厂商后台限制和厂商推送需单独进行真机验证。

## 构建与自动化

使用 Node 22.12+、JDK 17、SDK／Build Tools 36、NDK 27.1.12297006。Gradle 9.3.1 的分发校验和已固定，可直接复用已有 SDK，无需再安装一套 IDE。

```sh
cd mobileapp
npm ci
npm run check -- --runInBand
cd android
./gradlew :app:assembleQa :app:testQaUnitTest :app:lintQa --no-daemon
```

`com.gtmux.app.qa` 自带 JS，不依赖 Metro。测试 fixture 模块仅注册在 QA／debug 包中，正式 `com.gtmux.app` 不注册此模块。正式构建缺少签名输入或精确源码 SHA 时直接失败，不会使用公开的 debug keystore 兜底。

仅对自己创建、名称为 `gtmux-qa-*` 或 `gtmux-ci*` 的 AVD 执行：

```sh
export APPIUM_HOME="$(mktemp -d)"
npx appium driver install --source=local "$PWD/node_modules/appium-uiautomator2-driver"
export ANDROID_SERIAL=emulator-5554
npm run test:e2e:android
```

执行器拒绝真机和正式包，使用独立 Appium 与合成服务，核对实际发送请求，保存失败截图，结束时只关闭自己启动的进程。单测、编译、模拟器操作、TalkBack、真机／厂商行为、真实 FCM 和商店验收分别记录。Android 不会调用 iOS 的模拟器启动或回收流程。

## 签名与交付

**Android private delivery build** 只接受 main 上的精确已审提交，该提交的最新 main CI 和安卓模拟器任务必须通过。云端构建生产模式的未签名 APK／AAB，核对 JS／原生库字节、两个 64 位架构、源码和 16 KB ELF 对齐，再只上传 age 加密产物。构建任务只接收公开的加密接收方，不接触应用或上传签名私钥。

下载精确的成功流水线产物，独立核对整体摘要、源码回执和每个文件的 SHA。在本机的空输出目录中执行 `scripts/android-delivery.py sign --input UNSIGNED --output SIGNED --source SHA --sdk SDK`，签名输入只从私有环境读取：`GTMUX_ANDROID_KEYSTORE`、`GTMUX_ANDROID_STORE_PASSWORD`、`GTMUX_ANDROID_KEY_ALIAS`、`GTMUX_ANDROID_KEY_PASSWORD`。然后执行 `verify`，将 APK／AAB 与固定应用证书核对，包含签名、源码／版本、JS／原生字节、双架构、16 KB ELF／ZIP 和 Google bundle 校验。构建或下载成功不能当作安装回执。R8 映射、JS source map、原生符号和源码包随加密构建保存。上架准备见[商店清单](../docs/android/store.zh.md)。

可选 Firebase 客户端使用公开仓库变量 `ANDROID_FIREBASE_APP_ID`、`ANDROID_FIREBASE_SENDER_ID`、`ANDROID_FIREBASE_API_KEY`、`ANDROID_FIREBASE_PROJECT_ID`；服务端推送账号私钥不进入客户端或构建任务。本机已有签名输入时，也可不带 `--unsigned` 直接构建；缺少私钥时不会回退使用 QA 签名。

版本名来自 `src/version.ts`。每个语义版本的构建序号为 1–999；versionCode 为 `major × 100000000 + minor × 1000000 + patch × 1000 + ordinal`，minor ≤99、patch ≤999，且不超过 Play 的 2100000000 上限。不同包不能复用已发布的 versionCode。

直接安装的更新必须保留应用签名身份。未来使用同一包名进入 Play 时，需要明确保留该签名；否则应使用独立 application ID。Play 的上传密钥不是设备用来识别更新的应用签名密钥。

中间构建和临时验包副本留在任务／构建缓存。只有唯一的签名、符号、源码恢复和关键验收证据，才在应用运行目录之外归档，并登记负责人、容量和复核日期。CI 产物会过期，不代表已经有异地恢复保障；不得删除唯一的签名或恢复材料。

Android 图片粘贴仅在点击“粘贴”后读取剪贴板，使用有大小和像素上限的私有临时文件；图片标注复用共享绘制与导出，Android 适配双指缩放和移动。自动化使用 Node 24.12+ 与本任务拥有的模拟器，按实际屏幕宽高核验横竖屏。

使用既有私有下载服务时，发布器读取 Git 之外、仅本用户可读的 Android 配置，使用独立安装包目录。凭据不进入命令参数，私有安装链接只写入私有结果文件：

```sh
python3 scripts/test-publisher.py --platform android run \
  --uploader /path/to/website/distribution/scripts/builds.py \
  --output /private/path/published.json --log /private/path/published.log -- \
  publish /private/path/gtmux.apk \
  --android-verification /private/path/VERIFICATION.json \
  --available-days 90 --receipt /private/path/upload-receipt.json
```

`--platform android` 默认选择 `gtmux-android` 及其独立配置，拒绝非 APK 或缺少验签回执的发布。访问链接仍为私有、默认 24 小时；版本可下载期限与签名有效性是两件事。

模拟器测试独立保留最近 2 MiB 的设备日志，不依赖 Appium 截图或控件树读取成功。自动化连接丢失时保留首个失败，后续流程记为未执行，并以失败退出；不会静默重启会话或把后续流程记为通过。
