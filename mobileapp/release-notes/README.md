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

## 怎么写这段文案（2026-09-11 学的）

商店文案和提交信息是两种东西。这个仓库的提交信息讲来龙去脉，商店文案不讲 —— 读它的人是在
决定要不要点「更新」，不是在做代码考古。

**中文照中文商店的通行写法写，不要从英文译过来。** 参考一个写得好的真实例子（滴答清单 8.2.00）：

```
编辑体验优化
- 书写更简洁：编辑时自动隐藏格式符号，专注于内容本身。
- 重点更清晰：支持更多高亮颜色标记内容，关键信息一目了然。
- 查看更方便：标题支持展开折叠，轻松浏览内容。
```

形状是：**分组小标题 + 每条「短标签：一句陈述句。」** 标签说这条对用户意味着什么，句子说它
具体做了什么，一条一行。不用破折号，不讲为什么会有这个改动，不写「不是 X 而是 Y」。
另一个极端是微信那种「解决了一些已知问题」，什么都没说，也不学。

**英文不套这个标签格式**（英文商店不这么写），但分组和长度保持一致 —— 两份是同一份文案的
两种母语写法，不是原文和译文。

**共同的规矩**：挑用户真能感觉到的能力，其余收成最后一句「其余为大量问题修复与体验打磨」。
不要把 changelog 全倒进去。
