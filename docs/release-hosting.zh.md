# 官方下载与发布托管

[English](release-hosting.md)

## 用户从哪里安装

短安装地址是 `https://ccy.dev/gtmux/install`，另一个入口为 `https://ccy.pub/gtmux/install`。两站保存 `bootstrap.sh` 的静态副本。它需要 Bash 与 OpenSSL／LibreSSL，用内置公钥验证签名清单、锁定版本、校验安装脚本的 SHA256 和字节数，再运行脚本。明确锁定较旧／尚未托管的版本时，可以使用已验证的当前安装脚本，保留所选 GitHub 产物版本。最初取得这份 bootstrap，仍信任对应网站的 HTTPS 和部署代码。

默认原生更新器与安装器依次尝试 `downloads.ccy.dev`、`downloads.ccy.pub`，读取带签名的 `gtmux/stable.json` 与不可覆盖的 `gtmux/vX.Y.Z/manifest.json`。只允许四种 CLI 压缩包、公证过的 Mac App、CLI 摘要与安装脚本。清单分别记录二进制源码与安装脚本源码，因此补托管旧版本时，可以换成已审查的新安装脚本，不重打、不冒充旧二进制的来源。

RSA-3072／SHA256 验证原始载荷，签名内的 SHA256 和字节数覆盖每个文件，包括 Mac ZIP。签名异常立即停下；普通 HTTP／网络不可用才尝试另一官方源及原有 GitHub 通道。已经验证清单后，任何兜底下载都必须匹配它。官方镜像请求不跟随重定向；原生更新器不运行远端脚本。明确设置仓库或镜像覆盖时，保留原有通道规则；短 bootstrap 始终验证官方通道。

直接下载可用 `https://downloads.ccy.dev/gtmux/latest/macos.zip` 或 `latest/darwin-arm64.tar.gz`，另有 `darwin-amd64`、`linux-arm64`、`linux-amd64`；短链接会转到固定版本路径。GitHub Releases 仍完整保存正式包和摘要，Homebrew 仍沿用 GitHub cask。

## 在哪打包、如何发布

GitHub Actions 继续构建不依赖 cgo 的 CLI，并为通用 Mac App 签名、公证、装订。两个精确源码任务都成功后，`release.yml` 才能发布同一份文件。上传器重新核 GitHub API 的文件大小／摘要和 CLI 摘要表，签名规范化清单，上传不可覆盖的对象，经两个公开下载域完整读回，最后才切 stable。缺文件、摘要错、签名／构建失败、已有版本被改、降级或 stable 并发变化都会停下，不把半套产物变成最新版本。

`release-mirror.yml` 仅能从 main 手动补托管已有正式 tag，核原发布任务，并记录当前受审安装脚本的源码。它不构建二进制、不重写 tag、不替换 GitHub 附件。如果切 stable 后的读回失败，要报告分发结果尚未确认，不能覆盖固定版本做隐式回滚；应按记录的源码排查、恢复服务，再重试。

独立 Pages 服务读取专用 R2 桶，支持 GET／HEAD、ETag 和单段字节范围；没有列目录、CORS、Cookie 或手机私有包路径。带鉴权的上传接口只收固定正式文件名，默认 Pages 域名不能公开下载。发布任务只拿独立上传 bearer，不持有 Cloudflare 账户 API 凭据；示例配置也不含账户标识或密钥。

## 运维配置与边界

1. 创建专用 R2 Standard 桶与 Pages 项目，绑定名为 `RELEASES`。从 `download-worker/wrangler.example.toml` 生成私有运维配置，不复用手机分发桶。
2. 设置准确下载主机白名单及单独上传主机；先在 Pages 关联域名，再给子域配置 CNAME。外部 DNS 的子域也可接入，不需要迁移根域。保留网站既有地区／鉴权规则，并禁止用默认 Pages 域名绕过公开下载策略。
3. 私钥保存在私有存储和恢复材料中，GitHub secret 名为 `GTMUX_RELEASE_SIGNING_KEY`。独立上传 bearer 分别设置为 Pages secret `PUBLISH_TOKEN` 与 GitHub secret `GTMUX_RELEASE_PUBLISH_TOKEN`。Git 只放公钥；换公钥要重建客户端并同步受审静态 bootstrap，不能静默换成老客户端不认识的密钥。
4. 运行 `make ci-test`、`python3 scripts/sync-release-feed.py --check`、`npm --prefix download-worker test` 和 `npm --prefix download-worker run build` 后，部署精确受审源码。不把凭据、私有恢复文件、运维配置放进 CI 附件。公开脚本、示例、workflow 的 secret 名、清单、公钥适合入库。
5. 在 Pages 路由中将 `/.well-known/acme-challenge/*` 留给平台处理，使 HTTP 域名验证和续期不被只接受 HTTPS 的下载 Worker 拦截。下载和发布路径继续经过 Worker；不要把验证文件或凭据放进静态资源。参见 [Pages 自定义域验证说明](https://developers.cloudflare.com/pages/platform/known-issues/#custom-domains)。
6. 宣传短地址之前，实际核 DNS／TLS、签名清单、两个源的完整文件摘要、短 bootstrap 字节、无鉴权上传拒绝、默认域下载拒绝和网页下载链接。代码或 CI 通过不代表这些入口已经可用。

当前每个上传文件限制 90 MiB，避免超过 Cloudflare 默认请求体上限；过大的包在切 stable 前失败。原生客户端另有 256 MiB 上限。R2 按官方价格免出站流量费，但存储／操作次数与 Pages Functions 请求分别计量，额度还与账户其他用量共享。按实测包体和实际访问量估算，不能把免费额度当成已核可用余量。这里用 Functions 读 R2，不声称拥有无限静态资源请求额度。

手机仍走 App Store／TestFlight／私有签名分发。商店、支持和隐私链接继续指向网站，不变成 Mac 下载或上传接口。纯托管变化不需要重打手机包，也不需要改正在审核的商店提交。网页安装／下载文案要对应已部署路径，生成文档仍保留独立、准确的源码来源记录。

参考：[Pages 自定义域名](https://developers.cloudflare.com/pages/configuration/custom-domains/)、[R2 定价](https://developers.cloudflare.com/r2/pricing/)、[Functions 定价](https://developers.cloudflare.com/pages/functions/pricing/)、[平台上限](https://developers.cloudflare.com/workers/platform/limits/)。
