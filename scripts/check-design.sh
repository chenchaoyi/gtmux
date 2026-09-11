#!/usr/bin/env bash
# Design + architecture conformance check (run in CI).
#
# This is the automated half of the "did we actually follow the design?" /
# "is the architecture still sound?" review (see docs/test/TEST-PLAN.md). It can't
# judge visuals — that's manual acceptance against docs/design/mockup/ — but it
# guards the machine-checkable invariants so they can't silently drift.
set -euo pipefail
cd "$(dirname "$0")/.."

DESIGN=docs/design/DESIGN.md
THEME=macapp/Sources/GtmuxBar/Theme.swift
fail=0
note() { echo "design-check: $*"; }

# 1. Status palette: the app's colors MUST equal DESIGN §9's authoritative hex.
for hex in EF4444 06B6D4 22C55E 8E8E93; do
  grep -qi "$hex" "$DESIGN" || { note "#$hex not in $DESIGN (spec changed?)"; fail=1; }
  grep -qi "$hex" "$THEME"  || { note "status color #$hex from DESIGN missing in $THEME"; fail=1; }
done

# 2. Architecture: the menu-bar app stays native — no systray regression.
if grep -rqi "systray" macapp/Sources 2>/dev/null; then
  note "macapp must stay native AppKit (no systray dependency)"; fail=1
fi

# 3. Architecture: the app stays a CONSUMER — it must not re-implement agent
#    detection; it only reads `gtmux agents --json` and shells `gtmux`.
if grep -rqi "list-panes\|gatherAgents\|classifyAgent" macapp/Sources 2>/dev/null; then
  note "macapp must consume 'gtmux agents --json', not re-detect agents"; fail=1
fi

# 4. The CLI must stay cgo-free (only the Swift app is native). Skipped where
#    there's no Go toolchain (the macOS CI job is Swift-only; Linux CI enforces it).
if command -v go >/dev/null 2>&1; then
  if CGO_ENABLED=0 go build -o /dev/null ./cmd/gtmux 2>/dev/null; then :; else
    note "CLI failed to build cgo-free (CGO_ENABLED=0 ./cmd/gtmux)"; fail=1
  fi
fi

# 5. Spec validity: the OpenSpec capability specs must stay well-formed, so a
#    behaviour change can't land with a broken/missing spec. This is the automated
#    half of CLAUDE.md's "spec ⇄ code ⇄ test consistency" rule (the spec-matches-code
#    and archive-hygiene halves are a review-gate checklist item). Needs node; both
#    CI runners (ubuntu + macOS) have it preinstalled, so a missing npx = a real gap.
if command -v npx >/dev/null 2>&1; then
  # PINNED, not floating: 1.11.0 (2026-08-26) turned "the Purpose section is still the
  # placeholder" into a WARNING, and --strict fails on warnings — so twelve specs written
  # long before it went red on a repo nobody had touched. Same shape as the staticcheck
  # break a week earlier: a gate that turns red on someone else's release schedule is not
  # testing this repo. Raise this deliberately, with the spec cleanup it then demands.
  if npx --yes @fission-ai/openspec@1.10.0 validate --specs --strict >/tmp/openspec-check.log 2>&1; then :; else
    note "openspec spec validation FAILED (specs malformed / drifted):"; cat /tmp/openspec-check.log; fail=1
  fi
else
  note "npx not found — skipping openspec validation (install node to enforce it)"
fi

# 6. CLI command ↔ docs drift. Every user-facing command wired into the dispatch
#    (internal/app/app.go) MUST be listed in CLAUDE.md's command registry — this is the
#    exhaustive reference, so a new/renamed command that isn't there is a drift (exactly
#    how `attach` once shipped undocumented). Internal/plumbing commands are exempt via
#    HIDDEN. The curated `gtmux --help` usage + docs/cli.md are a deliberate SUBSET, so
#    we can't require completeness there — instead we check them in REVERSE (a doc must
#    not reference a command that no longer exists). Spec↔behavior + "is this command
#    worth documenting in the curated usage/cli.md" stay REVIEW-GATE items (see below).
APP=internal/app/app.go
HIDDEN=" tunnel-client save-tab-order options app oneshot-run server-mode install-hooks uninstall-hooks uninstall-app "   # internal plumbing + back-compat aliases, not user commands
cmds=$(grep -oE 'case "[a-z][a-z0-9-]*"' "$APP" | sed 's/case "//;s/"$//' | sort -u)
# The command list is "Commands: `a`, `b`, … `uninstall-app`." — grab exactly that,
# truncating the prose that follows `uninstall-app` on the same line (else a command
# named in the description would falsely count as "listed").
listblock=$(awk '/Commands:/{f=1} f{print} /uninstall-app`\./{exit}' CLAUDE.md | sed -E 's/`uninstall-app`\..*/`uninstall-app`./')
for cmd in $cmds; do
  case "$HIDDEN" in *" $cmd "*) continue ;; esac
  needle=$(printf '`%s`' "$cmd")
  # Here-string, not `printf | grep -q`: grep -q closes the pipe on the first match,
  # SIGPIPE-ing printf ("write error: Broken pipe") — a timing-dependent flake that
  # failed CI at random. `<<<` feeds grep without a pipe, so there is nothing to break.
  grep -qF "$needle" <<<"$listblock" || {
    note "CLI command '$cmd' (dispatched in $APP) is NOT in the CLAUDE.md command list — add it there (and weigh 'gtmux --help' usage + a docs/cli.md section)"; fail=1
  }
done
for cmd in $(grep -oE '^## `gtmux [a-z][a-z0-9-]*`' docs/cli.md | sed -E 's/^## `gtmux ([a-z0-9-]+)`/\1/'); do
  grep -qxF "$cmd" <<<"$cmds" || {
    note "docs/cli.md documents 'gtmux $cmd' but it is not a dispatched command (renamed/removed → stale doc)"; fail=1
  }
done

# 7. Wake vocabulary ↔ docs drift. Every class the code can emit MUST be taught in BOTH
#    places a reader learns it: docs/cli.md's class table (the human) and the seeded
#    playbook (HQ itself). A class missing from the playbook means HQ receives a knock its
#    own charter never mentions — which is exactly what `usage·warn` and `stuck·waiting`
#    did for months. The needle is the BACKTICKED form: a bare grep for "done" or "tick"
#    matches ordinary prose and passes for the wrong reason.
#
#    BOTH means both LANGUAGES too. This checked docs/cli.md and internal/hq/hq.go — the
#    English doc and the English charter — while docs/cli.zh.md carries the same class
#    table and internal/hq/playbook_zh.go carries the Chinese charter a home seeded in
#    Chinese actually receives. Neither was read. The failure the paragraph above
#    describes, a knock whose charter never mentions it, was still fully available in the
#    other language, and the operator's own HQ runs in that one.
#
#    The files are FOUND, not listed: any file defining an hqInstructions<LANG> constant
#    is a charter, and docs/cli*.md is the doc pair. A third language is covered the day
#    it lands, without anyone remembering this script exists.
WAKE=internal/hqwake/wake.go
PLAYBOOKS="$(grep -rlE 'const hqInstructions[A-Z]+ = ' internal/hq/*.go | sort)"
CLASS_DOCS="$(ls docs/cli.md docs/cli.*.md 2>/dev/null | sort -u)"
DOC_HIDDEN=" "   # classes deliberately not surfaced (none today)
[ -n "$PLAYBOOKS" ] || { note "no seeded charter found — the wake-vocabulary check just stopped checking"; fail=1; }
[ -n "$CLASS_DOCS" ] || { note "no cli doc found — the wake-vocabulary check just stopped checking"; fail=1; }
for c in $(grep -oE 'Class[A-Za-z]+ += +"[^"]+"' "$WAKE" | sed -E 's/.*"([^"]+)"/\1/' | sort -u); do
  case "$DOC_HIDDEN" in *" $c "*) continue ;; esac
  for d in $CLASS_DOCS; do
    grep -qE '`'"$c"'(`|·)' "$d" || {
      note "wake class '$c' is not in $d's class table (a reader meets a glyph nothing explains)"; fail=1
    }
  done
  for pb in $PLAYBOOKS; do
    grep -qE '`'"$c"'(`|·)' "$pb" || {
      note "wake class '$c' is not taught in the seeded playbook ($pb) — an HQ home seeded from that charter would get a knock it never mentions"; fail=1
    }
  done
done

# 8. Retired vocabulary must stay retired. Each entry names the change that retired it, so
#    the list is an audit trail rather than a pile of greps nobody dares delete from — and
#    an entry can itself be retired once its change is forgotten. Scoped per entry: the
#    file that DOCUMENTS a retirement legitimately contains the token (CLAUDE.md's
#    代码位置对照 table maps the Swift migration's deleted paths on purpose).
#
#    token · retired-by · files exempt (space-separated, "" = none)
#    Searched: the docs that describe CURRENT behavior. openspec/changes/** is excluded on
#    purpose — a proposal must be able to QUOTE the thing it is retiring (this very change
#    does), and the same goes for the archive's audit trail.
DOC_TREE="docs CLAUDE.md README.md README.zh.md api openspec/specs"
retired_check() {  # $1=token  $2=retired-by  $3=exempt files
  for f in $(grep -rlF "$1" --include=*.md $DOC_TREE 2>/dev/null | sed 's|^\./||'); do
    case " $3 " in *" $f "*) continue ;; esac
    note "$f reintroduces '$1' — retired by $2"; fail=1
  done
}
retired_check '[gtmux] '        'hq-perception-v2 (the wake format is now `» gtmux·<class>`)' \
  'openspec/specs/chat-transcript/spec.md docs/design/DESIGN.md docs/design/HANDOFF.md'
retired_check 'internal/menubar/' 'the Swift migration v0.0.11 (the package is gone)' \
  'CLAUDE.md docs/design/DESIGN.md docs/design/HANDOFF.md'
retired_check 'hq-feed' 'retire-perception-spool (the spool daemon and its command are gone)' ''
retired_check 'feed-degraded' 'retire-perception-spool (the wake class retired with its raiser)' ''

# ── the mobile What's New notes are GENERATED, not authored twice ─────────────
#
# mobileapp/src/releaseNotes.ts is produced by mobileapp/scripts/gen-release-notes.sh from
# the per-version archive in mobileapp/release-notes/. Nothing enforces that at build time,
# and re-running a *version* script is not an obvious part of "edit the release notes" — so
# the drift is checked by REGENERATING and diffing. Byte-exact, and none of the un-escaping
# guesswork that comparing a TS string literal back to its source would need.
#
# Not a jest test on purpose: reading the archive needs Node's fs, and the mobile tsconfig
# deliberately limits itself to jest types so APP code cannot reach for Node APIs.
NOTES_TS="mobileapp/src/releaseNotes.ts"
NOTES_GEN="mobileapp/scripts/gen-release-notes.sh"
if [ -f "$NOTES_TS" ] && [ -f "$NOTES_GEN" ]; then
  if ! bash "$NOTES_GEN" | diff -q - "$NOTES_TS" >/dev/null 2>&1; then
    note "$NOTES_TS is stale vs mobileapp/release-notes/ — run mobileapp/scripts/set-version.sh"
    fail=1
  fi
fi

# ── …and a stamped version that CHANGED the app must say what changed ─────────
#
# The check above enforces CONSISTENCY (the generated file matches the archive). It was
# green on 2026-08-09 while the app shipped 0.48.0 with three user-visible changes and no
# notes for any of them: the commander updated, opened What's New, and saw 0.47.0 and
# 0.45.13 — every version except the one being run. Consistency is not COVERAGE.
#
# The rule: the stamped version may legitimately have no notes of its own — the app version
# follows the gtmux tag, so most stamps cross a CLI-only release the app never shipped, and
# repeating the previous bullets under a second heading is worse than saying nothing. That
# is true ONLY while the app itself is unchanged. `<newest>.srchash` records the app the
# newest notes describe; if the live sources no longer hash to it, the skip is not a
# CLI-only crossing, it is a release with nothing to say for itself.
#
# Fails OPEN on what it cannot establish (no archive, no recorded hash — every entry
# written before this gate existed): a gate that guesses is worse than one that abstains.
APP_VER_TS="mobileapp/src/version.ts"
HASH_SH="mobileapp/scripts/appsrc-hash.sh"
if [ -f "$APP_VER_TS" ] && [ -f "$HASH_SH" ]; then
  stamped="$(sed -nE "s/.*APP_VERSION = '([^']*)'.*/\1/p" "$APP_VER_TS")"
  newest="$(ls mobileapp/release-notes/*.en.txt 2>/dev/null |
    sed -e 's|.*/||' -e 's|\.en\.txt$||' | sort -Vr | head -1)"
  recorded="mobileapp/release-notes/${newest}.srchash"
  if [ -n "$stamped" ] && [ -n "$newest" ] && [ "$stamped" != "$newest" ] && [ -f "$recorded" ]; then
    if [ "$(bash "$HASH_SH")" != "$(cat "$recorded")" ]; then
      note "mobileapp is stamped $stamped but the newest release notes are $newest, and the app has CHANGED since those notes — write mobileapp/fastlane/metadata/*/release_notes.txt for $stamped, then re-run mobileapp/scripts/set-version.sh"
      fail=1
    fi
  fi
fi

# N. Icon size floor (DESIGN §16). Icons kept coming out smaller than the text beside
#    them — twenty-two sites at 8–11pt when this was written — so the floor is checked
#    rather than remembered. It reads the DECLARED point size only: it cannot judge
#    optical size, or whether an icon is large enough for its context. That stays a
#    reviewer's call; this just stops the floor being crossed by habit.
#    StatusBadge is exempt by design (§16.5) — it is a drawn shape, not an icon, and it
#    is not an Image(systemName:), so it never matches here.
#    It follows the MODIFIER CHAIN, not the line: a first version grepped single lines
#    and so read `Image(systemName: "magnifyingglass")` followed by `.font(…size: 9…)`
#    on the next line as compliant — a gate that is green whatever the code does.
small_icons="$(find macapp/Sources -name '*.swift' -print0 2>/dev/null | xargs -0 awk '
  # A SwiftUI modifier chain is the declaration plus the following lines that begin
  # with a dot. Track one from each Image(systemName:) and report a size below 12.
  /Image\(systemName:/ { chain = 1; start = FNR; buf = $0; next }
  chain && /^[[:space:]]*\./ { buf = buf " " $0
                               if (match(buf, /\.font\(\.system\(size: ([0-9]|1[01])[,)]/)) {
                                 print FILENAME ":" start ":" buf; chain = 0
                               }
                               next }
  { chain = 0 }
' || true)"
if [ -n "$small_icons" ]; then
  note "icons below the DESIGN §16 floor of 12pt:"
  echo "$small_icons" | sed 's/^/  /'
  fail=1
fi

# N+1. A text CHARACTER used as an icon (DESIGN §16.1). Its ink has nothing to do with
#      its point size — "⌕" at 13pt read smaller than the 12pt placeholder next to it.
#      Each entry names what it should be instead.
for pair in "⌕:magnifyingglass" "✕:xmark" "⚙:gearshape"; do
  ch="${pair%%:*}"; sym="${pair##*:}"
  if grep -rn "Text(\"$ch\")" macapp/Sources 2>/dev/null | grep -q .; then
    note "DESIGN §16.1: '$ch' is a text character used as an icon — use Image(systemName: \"$sym\")"
    fail=1
  fi
done

# N+2. Every path that types into a pane and submits must be a KNOWN one. The draft
#      guard has been written three times — for `gtmux send`, for the wake nudge — and
#      each time a NEW writer appeared later without it. HQ's session rotation typed
#      `/clear` straight into the composer, landed on a half-written `%11 `, and
#      submitted `%11 /clear` as an instruction (2026-08-29). A guard that has to be
#      remembered gets forgotten, so a new writer now has to be declared here, which is
#      a reviewer asking "does this one ask dispatch.BoxEmpty first?".
writers=$(grep -rn "tmux\.Paste(\|tmux\.SendText(\|tmux\.SendKey(" internal --include='*.go' \
  | grep -v '_test\.go:' | grep -v '^internal/tmux/' \
  | grep -v ':[0-9]*:[[:space:]]*//' \
  | cut -d: -f1 | sort -u)
known="internal/app/adopt.go
internal/app/agent_resume.go
internal/app/oneshot.go
internal/app/plainsend.go
internal/app/send.go
internal/app/serve.go
internal/app/spawn.go
internal/dispatchbridge/dispatchbridge.go
internal/hq/hq.go
internal/hq/selfrotate.go
internal/hqnudge/hqnudge.go"
new_writers=$(comm -23 <(echo "$writers") <(echo "$known" | sort))
if [ -n "$new_writers" ]; then
  note "a new path types into a pane — it must ask dispatch.BoxEmpty (or go through dispatch.PasteAndSubmit) before submitting, then be added to the list in this script:"
  echo "$new_writers" | sed 's/^/  /'
  fail=1
fi


# N+3. Every gtmux path must resolve $HOME through internal/state, which is where the
#      guard lives that keeps a test off the operator's real home.
#
#      That guard shipped on 2026-09-09 with a comment calling state the "one chokepoint
#      every gtmux path goes through". It was not: twenty-three files read $HOME on their
#      own, including the ones that install hooks into ~/.claude/settings.json and write
#      serve's device roster and the pairing credentials. The claim was never checked
#      because nothing could check it, so the guard covered the two paths its author had
#      in mind and the rest kept the door open.
#
#      This is that check. A new resolver has to come here to get past it, which is a
#      reviewer asking "should this one be reachable from a test?".
resolvers=$(grep -rln 'os\.Getenv("HOME")\|os\.UserHomeDir()' --include='*.go' . 2>/dev/null \
  | grep -v '_test\.go$' | grep -v '/node_modules/' | sed 's|^\./||' | sort -u)
if [ "$resolvers" != "internal/state/state.go" ]; then
  note "these resolve \$HOME outside internal/state, so the test guard there cannot see them — route them through state.Home():"
  echo "$resolvers" | grep -v '^internal/state/state.go$' | sed 's/^/  /'
  fail=1
fi


# N+4. Bilingual user docs ship as PAIRS.
#
#      CLAUDE.md: "USER DOCS ARE BILINGUAL, and both halves ship in the same PR… If you
#      add a new USER doc, it is born as a pair." That lived only in prose, and a doc born
#      single would have read as fine to every gate here.
#
#      It is the same hole this file kept finding on 2026-09-09 from the other side: the
#      wake-vocabulary check above, and internal/docs, each opened one half of a pair and
#      called it the docs. Enforcing the pairing is what stops the next checker from
#      having only one half to open.
#
#      SUBJECTS ARE FOUND, exceptions are written down. Every docs/*.md and README*.md is
#      a user doc unless it is named below; the list is maintainer logs, which change
#      constantly and which nobody reads to learn the product (CLAUDE.md states this
#      boundary). An entry here shrinks coverage deliberately and visibly, which is the
#      whole difference between an exception list and a list of things to check.
# The store's lock-screen screenshot is DRAWN, and it reads the card's own numbers rather
# than copying them (mobileapp/scripts/widget-tokens.mjs). This is the half a drawing
# cannot do for itself: if a band is added, removed or reordered in the widget, every
# number still parses and the drawing is quietly out of date — so the ORDER is pinned here.
if [ -f mobileapp/scripts/widget-tokens.mjs ]; then
  node mobileapp/scripts/widget-tokens.mjs --check >/dev/null || {
    node mobileapp/scripts/widget-tokens.mjs --check 2>&1 | head -4
    fail "the Live Activity card moved but the store's drawn screenshot did not"
  }
fi

SOLO="docs/TROUBLESHOOTING.md docs/release-signing.md docs/appstore-shots.md"
for f in README.md docs/*.md; do
  case "$f" in *.zh.md) continue ;; esac
  case " $SOLO " in *" $f "*) continue ;; esac
  base="${f%.md}"
  [ -f "${base}.zh.md" ] || {
    note "$f has no Chinese twin (${base}.zh.md) — a user doc is born as a pair; if this is a maintainer log, add it to SOLO in this script"
    fail=1
  }
done
# And the other direction: a translation whose original was renamed or deleted is a doc
# nobody will ever update again.
for f in README.zh.md docs/*.zh.md; do
  [ -e "$f" ] || continue
  base="${f%.zh.md}"
  [ -f "${base}.md" ] || { note "$f has no English original (${base}.md)"; fail=1; }
done
for f in $SOLO; do
  [ -f "$f" ] || { note "SOLO names $f, which does not exist — the exception outlived its file"; fail=1; }
done


if [ "$fail" = 0 ]; then
  note "OK — status palette matches DESIGN §9; architecture invariants hold; icons meet the §16 size floor; specs valid; CLI commands documented; wake vocabulary taught; retired vocabulary stays retired; pane writers declared; \$HOME resolves through state; user docs are paired; mobile release notes generated"
else
  exit 1
fi
