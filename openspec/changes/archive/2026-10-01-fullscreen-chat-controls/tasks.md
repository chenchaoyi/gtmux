- [x] Anchor fixed full-screen chat controls to the top safe area without shrinking the scrolling canvas.
- [x] Add rendered control/action regression tests, bilingual design docs and spec.
- [x] Run mobile and repository gates and submit PR; device acceptance remains pending.

## Results
- Rendered full-screen control and existing safe-area tests: 6 passed.
- Full mobile gate (TypeScript, ESLint, Jest): 1,176 passed, 6 skipped; existing warnings remain nonblocking.
- `GOFLAGS=-p=2 make check`, Web Node tests (7) and design/OpenSpec conformance passed.
- Physical iPhone/iPad pixel and screen-reading acceptance remains pending an unlocked connected device. No native mobile build, release or install performed.
