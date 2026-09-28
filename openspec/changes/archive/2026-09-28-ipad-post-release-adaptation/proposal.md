# Adapt post-1.0.30 mobile features to iPad

## Why

The iPad uses the shared split shell but opens HQ through `HQView`, not the generic
`DetailView`. The background-task row added after 1.0.30 was wired only into
`DetailView`, leaving the normal HQ entrance without it on both devices. The demo
tasks also named panes absent from the sample fleet. New task and share delivery
sheets stretched across the full iPad width.
The split shell also defaulted to the first roster row, which can be HQ, and then
showed HQ as a generic pane detail until the user tapped its dedicated sidebar card.

## What changes

- Fetch and show dispatched work in the actual HQ page on both shells. Task rows
  select the matching pane through workspace state.
- Point demonstration tasks at sample panes so their navigation can be tested.
- Keep demonstration task states aligned with the sample fleet after an approval.
- Limit task and share delivery sheets to readable widths on regular iPad canvases.
- Use the active palette for the secondary text in the task control.
- Prefer a worker at split-shell startup; if HQ is alone, use the dedicated HQ page.

## Surfaces

- **Phone / 手机** — the normal HQ entrance now has its task control.
- **iPad / 平板** — the HQ task path works in the split shell; task and share
  sheets use a centred reading width.
- **Terminal / 终端, Menubar / 菜单栏, Web / 网页** — no interface change.
