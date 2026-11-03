# Android 上架准备

[English](store.md)

APK 内测交付与 Google Play 提审是两件事。编译、模拟器测试或加密产物都不代表已注册开发者账号或已上架。以下要求于 2026-11-03 根据官方资料核对。

| 需要完成的事 | 具体要求 |
| --- | --- |
| 注册或确认 Google Play 开发者账号 | 一次性注册费 25 美元，需核验身份和联系信息。使用真实的个人或组织主体；组织核验可能需要 D‑U‑N‑S。 |
| 完成账号／设备核验 | 新个人账号可能需要通过 Play Console app 使用真实 Android 设备核验，模拟器不能代替。 |
| 组织适用的封闭测试 | 2023-11-13 之后注册的个人账号，申请正式发布权限前需至少 12 名测试者连续加入封闭测试 14 天。仅完成内部测试不等于满足此要求。 |
| 配置可选 FCM 的 Firebase 项目 | 登记 `com.gtmux.app`，分别保存客户端配置与 relay 的服务账号凭据。FCM 依赖 Google 服务，不能代替所有厂商推送。 |
| 上传前确定 Play 签名身份 | 同一 application ID 如需与直接安装版兼容，应保留应用签名证书。上传密钥与应用签名密钥作用不同。 |
| 核对候选包对应的商店资料 | 准备手机／平板截图、支持与隐私链接、内容评级、审核访问／演示说明，并按实际 SDK 和行为填写 Data safety。不能照抄 iOS 隐私或无障碍声明。 |

安装包包含相机／ML Kit 和可选 Firebase Messaging 依赖。应结合候选包依赖图与启用情况核对 [ML Kit 披露说明](https://developers.google.com/ml-kit/android-data-disclosure)及 [Firebase Android 披露说明](https://firebase.google.com/docs/android/play-data-disclosure)。相机在扫码时申请权限，通知保持可选；未集成 Firebase Analytics，并不等于所有第三方 SDK 都不收集数据。

APK／AAB 需要验证自身及上游原生库的 16 KB 页支持。时间要求以 Google 的[当前说明](https://developer.android.com/guide/practices/page-sizes)为准；本地验包不能替代 Console 的校验。

Play 之外的分发也需关注 Android 开发者核验。2026 年首批范围为巴西、印度尼西亚、新加坡和泰国的认证设备，2027 年起扩大。应按目标国家及开源／有限分发选项查看[官方项目](https://developer.android.com/developer-verification)，不能理解为全球所有 APK 都必须有 Play 账号。

国内应用商店、厂商推送和无 Google 服务设备，需要单独核对渠道账号与真机表现。不能从 Apple 账号或已构建 APK 推导出厂商账号、法律主体、备案、付款或测试者名单已经就绪。

出处：[Play 注册](https://support.google.com/googleplay/android-developer/answer/6112435?hl=en)、[个人账号测试要求](https://support.google.com/googleplay/android-developer/answer/14151465?hl=en)、[账号类型与核验](https://support.google.com/googleplay/android-developer/answer/13634885)、[Firebase 配置](https://firebase.google.com/docs/android/setup)、[Android 应用签名](https://developer.android.com/studio/publish/app-signing)。
