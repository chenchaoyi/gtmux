# hq-verdict-single-source — the HQ verdict is decided once, in the core, and rendered by each surface

Origin: the commander's 2026-08-11 report — 「menubar 里的 hq 的展项形态，现在跟 app 里
差距太远了」 — and what auditing that complaint actually found.

## The complaint was about presentation; the defect underneath is worse

The design is already coherent ON PAPER. DESIGN §12 and MOBILE §17 both mandate that the
two surfaces share one identification token (the medallion / disc: brand mark + `HQ`
wordmark + status ring), one six-state priority model, and one headline synthesis rule —
「两屏读到同一个 HQ」. The two surfaces LOOK different on purpose: the menubar card is an
entry (it jumps to the pane), the command console lives on phone/web, and MOBILE §17 says
the phone radar uses a disc precisely because 「圆盘放不下那句合成头条」. DESIGN §12 draws
a red line against copying the phone's structure into the menubar: 「曾经复制导致漂移」.

**But the shared rule is implemented three times, and it has already diverged.** The
headline exists as `AgentStore.fleetHeadline` (Swift), `assessment()` (TS), and nowhere in
the core. Measured today:

| case | menubar (Swift) | phone (TS) |
|---|---|---|
| a worker waiting | `X needs you · N others normal` | same ✅ |
| **HQ itself waiting** | 「请你拍板」 | **not considered** — `workerRows()` filters the supervisor out, so it reads 「都正常」 |
| **machine at the red resource tier** | 「机器资源紧张」 | **no such branch** — `assessment()` takes no resource input at all, so it reads 「都正常」 |
| no other agent sessions | falls to 「都正常·无需你介入」 | 「暂无其它 agent 会话」 |

Rows 2 and 3 are the SAME defect already found and fixed on the menubar side. The Swift
resolver's own comment says so: *"Fixes the reported contradiction where a red resource
tier still read 'all normal'."* The fix never reached the phone, because the rule is
written twice and nothing connects them.

So: when the machine is under real pressure, the menu bar says 「机器资源紧张」 and the
phone's HQ page says 「都正常 · 无需你介入」 — **two contradictory conclusions about one
fleet at one moment.** That is a worse incoherence than any styling difference, and it is
the most likely true source of the reported feeling.

## What changes

The verdict moves into the core, which is the repo's own stated architecture (one Go
core, three screens, surfaces are pure consumers) applied to the one place it was not.

**Decide in the core; render at the edge.** The core SHALL NOT serve the rendered
sentence. The headline must be localized, and each surface owns its own language state —
the phone has a three-way follow-system/EN/中文 toggle while serve has a single
`GTMUX_LANG`, so shipping a pre-rendered string would silently override a reader's
language choice. **Corrected during implementation:** this proposal first said the digest would gain a
top-level `hq` object. It cannot — `GET /api/digest` is a bare ARRAY, so a sibling field
would break every existing consumer. The verdict hangs on the SUPERVISOR ROW instead
(`role:"supervisor"` already identifies it), which is additive, compatible in both
directions, and arguably where it belonged anyway. The row carries:

- `state` — the resolved six-state verdict (`absent` / `hq_call` / `needs_you` /
  `resource` / `working` / `normal`), priority-ordered in ONE place;
- the FACTS a headline needs: how many workers are waiting, the first waiter's name, how
  many others are normal, how many worker sessions exist at all.

Each surface renders its own sentence from that, in its own language. The judgment is
single-sourced; only the wording stays local — and a surface can no longer print
「都正常」 while the token beside it is red, because the state it is given says otherwise.

## Impact

- Specs: `supervisor-agent` (the verdict + its priority), `mobile-app` (the HQ page
  headline consumes it), and `api/contract.md` (the additive `hq` object).
- Code: `internal/radar/digest.go` (compute), `macapp` `AgentStore` (consume instead of
  re-deciding), `mobileapp` `hqZones.assessment` (consume instead of re-deciding).
- The additive `hq` object must be OPTIONAL, so an older surface against a newer core —
  and a newer surface against an older core — both keep working; each surface keeps its
  local resolver as the fallback for an absent `hq`.
- Risk: this is a behavior-visible unification. The phone will START saying 「请你拍板」
  and 「机器资源紧张」 where it used to say 「都正常」. That is the point, and it is worth
  a line in the release notes.
