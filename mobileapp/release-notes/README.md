# release-notes — the per-version archive the in-app What's New reads

The App Store metadata (`fastlane/metadata/*/release_notes.txt`) holds only the CURRENT
submission's notes: fastlane overwrites it every release. That is fine for the store, which
shows one version, but the in-app popup has to answer a harder question — **a user who
skipped three versions must see all three** — so the notes are ARCHIVED here, one pair of
files per shipped version, and `src/releaseNotes.ts` is generated from the whole directory.

- `<version>.en.txt` / `<version>.zh.txt` — one bullet per line, `•` optional.
- `mobileapp/scripts/set-version.sh` copies the current fastlane notes in under the version
  being stamped, then regenerates `src/releaseNotes.ts`. So the normal release flow needs no
  extra step: write the store notes as always, stamp the version, and the archive grows.
- `scripts/check-design.sh` fails when the generated file is stale, by regenerating it and
  diffing — byte-exact, no guesswork.

History before this archive existed lives only in App Store Connect; a user upgrading from
one of those versions sees the notes from here onward, which is everything the app can
honestly claim to know.

## When a submission crosses several versions

Most device builds are not submitted, so the App Store can sit several versions behind the
archive — 0.50.0 was submitted while the store still showed 0.45.13, with 0.47.0, 0.48.1
and 0.48.4 never having reached a store reader. In that case the STORE notes should cover
everything since the last SUBMITTED version (a reader is deciding whether to update from
what they have), while the archive keeps one entry per version, because the in-app popup
already replays each skipped version and would otherwise say everything twice.

So the two deliberately diverge for that release: write the multi-version text into
`fastlane/metadata/*/release_notes.txt` and **do not re-run `set-version.sh`** — it would
archive that text under the stamped version and duplicate it in the popup.

⚠️ **The next stamp must overwrite the store notes first.** `set-version.sh` archives
whatever is in that file at the time, so a stamp that reuses a multi-version text would
file it under the wrong version. Writing the store notes before stamping is the normal
flow anyway; this is only a reason not to skip it.

## `store/` — what the App Store showed, when it differs

`fastlane/metadata/*/release_notes.txt` feeds TWO consumers: the App Store listing and
(via `set-version.sh`) the in-app What's New archive. They are the same text in the normal
case, because every version ships to both.

They diverge when the store falls behind. 0.55.1 was submitted after the store had sat at
0.50.0 for six versions, so its "What's New" had to cover everything a user crossed — while
the in-app popup still needs the per-version entries, one per version skipped. The submitted
store text lives here so the difference is recorded rather than left as a modified file
somebody has to remember to put back.

**When this happens again:** write the consolidated text, submit it, then put a copy under
`store/<version>.{en,zh}.txt` and restore `fastlane/metadata/*/release_notes.txt` to the
per-version text BEFORE the next stamp. Otherwise `set-version.sh` archives the
consolidated text as some later version's per-version entry, and the in-app popup tells a
reader about changes they already saw.

## 怎么写这段文案（2026-09-11 学的，2026-09-17 改）

商店文案和提交信息是两种东西。这个仓库的提交信息讲来龙去脉，商店文案不讲，读它的人是在
决定要不要点「更新」，不是在做代码考古。

**每条直接说做了什么，不加讨巧的短标签。** 2026-09-11 定的格式是「分组小标题 + 每条「短标签：一句陈述句。」」，
照着写出来的是「翻得回去」「拍板更快」「额度条好认」「一个 app 两台设备」这类标签，标签本身不带信息，
读着像口号。司令 2026-09-17 看了 1.0.30 的商店文案说「太恶心了，说人话」，这个格式作废。

现在的写法：

```
用量
- 额度条按用量变色：平时是蓝色，周额度超过 85% 变成琥珀色，用完变成红色。
- 额度窗口名称、重置时间和机器告警跟随手机语言显示。
```

- 小标题用普通名词（「支持 iPad」「用量」「其他改进」），不用「一目了然」「处处更清楚」这种评价。
- 每条一句话讲一件具体的事：用户看到什么、能做什么。冒号只用来引出后面真实的列举。
- 不用破折号，不写「不是 X 而是 Y」，不用「一眼」「好认」「记得更久」这类渲染词，不讲为什么会有这个改动。
- 内部叫法换成界面上看得到的名字，界面里的名字加引号（「所有 pane」「告诉 HQ」）。
- 挑用户真能感觉到的变化，其余收成最后一句「另外修复了大量问题。」，不要把 changelog 全倒进去。
- 写完用 humanizer skill 过一遍再推。

英文分组和条数与中文保持一致，按英文的母语习惯写，不是中文的译文。
