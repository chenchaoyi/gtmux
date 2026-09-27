# Knowledge lint accuracy and mobile timer cleanup

## Why

HQ knowledge lint counted example wiki syntax as broken references and Chinese
explanatory text as a credential. A knowledge ledger append could then fail to
render before its event receipt was written. Mobile tests exposed callbacks left
after screens were unmounted, including a real copy-feedback timer and chat frames.

## What changes

- Ignore known placeholder links and bracketed prompt samples, while retaining
  checks for real wiki references. Match credential-shaped assignment values.
- Audit a knowledge operation immediately after its ledger append succeeds, even
  when refreshing its derived views later fails.
- Cancel pending share feedback and chat animation work on unmount. Unmount test
  renderers and await asynchronous React effects before Jest teardown.

## Boundary

The remaining broken links and possible duplicate lessons in the machine ledger
need fact-by-fact review through knowledge verbs. The linter never rewrites them.
An orphan count is a navigation hint, not a deletion instruction.

## Surfaces

- **Terminal / 终端** — more accurate knowledge lint and a durable audit receipt.
- **Phone / 手机 and iPad / 平板** — copy feedback and chat frames stop after leaving a view.
- **Menubar / 菜单栏 and Web / 网页** — no interface change.
