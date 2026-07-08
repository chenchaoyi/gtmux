# Task for the ccy.dev site: add gtmux's App Store pages

Add gtmux's App Store URLs to the ccy.dev / ccy.pub site, **mirroring the existing rodi
setup exactly**. gtmux is a companion iOS app for monitoring/steering coding-agent
sessions on your own Mac (repo: `github.com/chenchaoyi/gtmux`).

## What to create

Under `src/pages/projects/gtmux/`, three Astro pages that mirror
`src/pages/projects/rodi/{privacy,support,index}.astro` — same `BaseLayout`, same
bilingual `data-i18n="en"` + `data-i18n="zh"` article structure (so **ccy.dev leads
with English, ccy.pub / SITE_TARGET=cn leads with Chinese**, exactly like rodi).

1. **`privacy.astro`** — the privacy policy. Use the content in
   `/Users/ccy/meituan/chenchaoyi/gtmux/docs/appstore/privacy-policy.md` **verbatim**:
   the "## English" section → the `data-i18n="en"` article, the "## 中文" section →
   the `data-i18n="zh"` article. Keep the headings/structure; convert Markdown to the
   same HTML markup rodi's privacy.astro uses. Title like "gtmux · privacy · ccy.dev".

2. **`support.astro`** — a short support page (mirror rodi's support.astro):
   - EN: "gtmux is a command center for tmux + coding agents. Questions or issues:
     open an issue at github.com/chenchaoyi/gtmux/issues, or email ccy.chenchaoyi@gmail.com."
   - ZH: "gtmux 是 tmux + coding agent 的指挥中心。有问题或反馈：在
     github.com/chenchaoyi/gtmux/issues 提 issue，或发邮件到 ccy.chenchaoyi@gmail.com。"

3. **`index.astro`** — a brief marketing/landing page (mirror rodi's index.astro):
   one-liner "gtmux — see which coding agent needs you across your tmux sessions; jump,
   reply, and get a push the moment one's blocked" (zh: "gtmux —— 跨所有 tmux 会话，一眼
   看清哪个 coding agent 在等你；跳转、回复、有人卡住立刻推送"), a link to the GitHub repo,
   and (optional) a link to the App Store once live.

## Resulting URLs (what I'll paste into App Store Connect)

- Privacy (en): `https://ccy.dev/projects/gtmux/privacy`
- Privacy (zh / CN storefront): `https://ccy.pub/projects/gtmux/privacy`
- Support: `https://ccy.dev/projects/gtmux/support` (+ ccy.pub)
- Marketing: `https://ccy.dev/projects/gtmux`

## Finish

Build + deploy to **both** ccy.dev and ccy.pub the same way rodi's pages ship, then
reply with the final live URLs (and flag anything you changed from this spec).
