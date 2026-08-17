# hq-first-person — the supervisor speaks to ITS user: neutral seeds, honest memory, a discoverable personalization slot

Origin: the commander's 2026-08-17 whole-of-hq review, criterion "anyone learns
personalized knowledge from their own usage". A fresh user's first day currently
opens on someone else's notebook, and the file where they could say who they are
is invisible.

## Why — the measured findings

1. **The seeds ship the author's knowledge, not the user's blank page.** The
   accounts/workflows/best-practices seeds and the playbook's topic list name
   Apple developer accounts, Cloudflare, iOS Appium, and gtmux's own
   openspec release flow — three times over. `best-practices.md` arrives
   pre-filled with five of the author's operating lessons, including an
   internal marker `(B2)` that is defined nowhere and reads as noise to
   everyone.
2. **The playbook manufactures false memories.** It tells every fresh HQ "a
   real dispatch OF YOURS died exactly this way … with the footgun already
   recorded in YOUR OWN knowledge base TWICE" — on a new machine both claims
   are factually false, and an HQ that believes them will recount history that
   never happened on that machine. Evidence must read as mechanism-plus-
   incident, never as the reader's biography.
3. **The personalization slot is undocumented — and the docs point at the wrong
   file.** README and `docs/cli.md` say "edit `AGENTS.md` to change its
   policy"; that file's first line says DO NOT EDIT and every playbook upgrade
   regenerates it, silently displacing the user's customizations to a backup.
   `LOCAL.md` — the real slot — appears in NO user document and ships as an
   example-free stub. Nothing ever ASKS the user who they are.
4. **The growth path has undiscoverable prerequisites.** README's hq section
   never mentions the knowledge base, `gtmux capture`, or `LOCAL.md`; the
   first-run output names only `AGENTS.md`; and the periodic distill ritual
   depends on `gtmux serve`, which README frames purely as the phone feature —
   a local-only user may never see a distill fire.
5. **Corrections seed drift**: the seeded `corrections.md` still teaches the
   pre-ledger ritual ("FLAG it for a seed/spec update", edit topic files by
   hand) that phases 2–3 replaced with the knowledge verbs.
6. **A board a non-Chinese-reader cannot read and is forbidden to translate.**
   The board seed hardcodes Chinese headings and the playbook orders "WRITE THE
   CHINESE, DON'T TRANSLATE IT" — right for the commander whose language that
   is, wrong as a shipped default for everyone.

## What changes

- **Seeds become domain-neutral templates** (what belongs here, not whose
  accounts), the pre-filled author lessons leave `best-practices.md` (the two
  genuinely portable budget rules fold into Policy #3 where operating rules
  live), `environment.md` shrinks to its purpose plus a one-line
  `agent-proxy` pointer, and `corrections.md` teaches the current verbs.
- **False-memory passages rewrite as mechanism + dated incident** ("a real
  dispatch died exactly this way during development…"), repo-specific example
  slugs neutralize, and a REGRESSION TEST bans the author-leak tokens from the
  seeds and playbook so they cannot return.
- **The board seeds in the user's language** (`GTMUX_LANG`), and the language
  rule becomes "keep the board in the language it was seeded in — never flip an
  existing board", preserving the real lesson (don't machine-translate a
  living document) without mandating anyone's language.
- **LOCAL.md becomes discoverable and asked-for**: the template gains three
  commented examples (reporting style · quiet hours · domains/repos) and a
  boundary note (it governs the supervisor only; notifications live in
  `config.json`); README and `docs/cli.md` point personalization at LOCAL.md;
  the first-run notice names `knowledge/`, `LOCAL.md`, and one
  `gtmux capture` example; and the STARTUP BRIEFING gains a first-launch step —
  when LOCAL.md is still the untouched template, HQ asks the commander three
  questions (what you mainly work on · how you want status reported · any
  quiet hours) and writes the answers into LOCAL.md.
- **README's hq section** gains the knowledge base in one breath and names
  `gtmux serve` as what drives the periodic rituals; small doc debts ride
  along (`docs/cli.md`'s dead archived-change link, the `⟣` signal-glyph
  legend users currently cannot look up, the `hqWake` keys joining the config
  section). "Local notes" is renamed to the literal `notes/` wherever the
  playbook meant HQ's own files, ending the collision with LOCAL.md.

Playbook v27 → v28.

## Non-goals

Full playbook i18n (a single mixed-language charter remains; only the BOARD's
seeded language follows the user), agent-vocabulary abstraction beyond a
clarifying parenthetical (the charter keeps Claude Code's command names with a
"your agent's equivalents" note), and any mechanism change — this is the
teaching layer catching up with the machinery.

## Impact / risk

Text-heavy; the behavior additions are the first-run notice lines, the
language-aware board seed, and the first-launch questions (whose trigger — an
untouched LOCAL.md template — is mechanical). The ban-list test is the piece
that keeps this fixed: neutrality regressions become red builds, not review
archaeology.
