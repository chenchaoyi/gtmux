#!/usr/bin/env python3
"""
README "surface" screenshot compositor for gtmux (learns from Rodi's
docs/appstore/generate_screenshots.py). Renders one polished, branded,
device-framed card per surface — CLI (terminal), menu-bar (popover under a
faux menu bar), mobile (phone bezel) — via headless Chrome.

Brand tokens mirror mobileapp/src/ui/theme.ts (status colors) and CLAUDE.md.
Inputs: docs/marketing/src/{radar,popover}.png (real captures).
Outputs: docs/assets/surface-{cli,menubar,mobile}.png.

    python3 docs/marketing/gen.py
"""
import os
import subprocess

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
SRC = os.path.join(HERE, "src")
OUT = os.path.join(ROOT, "docs", "assets")
CHROME = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

W, H = 1080, 1340                       # each surface card (kept identical so they row up)
RED, CYAN, GREEN, GRAY = "#EF4444", "#06B6D4", "#22C55E", "#8E8E93"


def furl(p):
    return "file://" + p


def page(label, accent, stage):
    return f"""<!doctype html><html><head><meta charset="utf-8"><style>
*{{margin:0;padding:0;box-sizing:border-box}}
html,body{{width:{W}px;height:{H}px}}
body{{font-family:-apple-system,'SF Pro Display',Helvetica,Arial,sans-serif;
 background:radial-gradient(115% 75% at 50% -8%, #FFFFFF 0%, #EEF1F4 46%, #E0E5EA 100%);
 display:flex;flex-direction:column;align-items:center;overflow:hidden}}
.cap{{margin-top:74px;font-size:50px;font-weight:700;color:#15171C;letter-spacing:-1px;
 display:flex;align-items:center;gap:18px}}
.cap .dot{{width:19px;height:19px;border-radius:50%;background:{accent}}}
.stage{{flex:1;width:100%;display:flex;align-items:center;justify-content:center;padding:24px 0 88px}}
</style></head><body>
<div class="cap"><span class="dot"></span>{label}</div>
<div class="stage">{stage}</div></body></html>"""


# ── terminal frame (CLI) ─────────────────────────────────────────────
def cli_stage():
    def cell(text, color, width=None, bold=False):
        pad = "" if width is None else "&nbsp;" * max(0, width - len(text))
        w = "font-weight:700;" if bold else ""
        return f'<span style="color:{color};{w}">{text}</span>{pad}'

    def row(glyph, gc, status, sc, agent, loc, task, pane, latest=""):
        return (
            cell(glyph, gc) + "&nbsp;&nbsp;"
            + cell(status, sc, 8)
            + cell(agent, "#ECECF0", 13, bold=True)
            + cell(loc, "#ECECF0", 19, bold=True)
            + cell(task, "#C7C7CC")
            + latest
            + cell(f"&nbsp;&nbsp;{pane}", "#6E6E74")
        )

    head = (cell("gtmux agents", "#FFFFFF", bold=True)
            + cell("&nbsp;— 6 agents · 1 waiting · 1 working · 4 idle", "#8A8A90"))
    rows = [
        row("⏸", RED, "waiting", RED, "Claude Code", "Pica:0.0", "permission to run tests", "%7"),
        row("⠿", CYAN, "working", CYAN, "Claude Code", "ccy-workspace:1.0", "gtmux.app dev", "%14"),
        row("✳", GREEN, "idle", GREEN, "Claude Code", "Hammer:0.0", "Add auto-update", "%8",
            latest=cell("&nbsp;&nbsp;✓ latest", GREEN)),
        row("✳", GREEN, "idle", GREEN, "Codex", "Diting:0.0", "BLE scan timeout", "%3"),
    ]
    foot = cell("jump: gtmux focus %7", "#6E6E74")
    body = head + "<br><br>" + "<br>".join(rows) + "<br><br>" + foot
    dots = "".join(
        f'<span style="width:16px;height:16px;border-radius:50%;background:{c};display:inline-block"></span>'
        for c in ("#FF5F57", "#FEBC2E", "#28C840"))
    return f"""<div style="width:1012px;border-radius:20px;overflow:hidden;background:#0C0C0E;
      box-shadow:0 50px 110px rgba(18,22,32,0.30),0 12px 32px rgba(18,22,32,0.18)">
      <div style="height:60px;background:#191A1E;display:flex;align-items:center;padding:0 26px;gap:12px;
        border-bottom:1px solid #000">{dots}
        <span style="margin-left:20px;color:#8E8E93;font:600 23px/1 -apple-system">gtmux — agents</span></div>
      <div style="padding:42px 34px 48px;font-family:'SF Mono','Menlo',monospace;font-size:20px;
        line-height:1.85;color:#D6D6DA;white-space:nowrap">{body}</div></div>"""


# ── menu-bar frame (popover under a faux menu bar) ───────────────────
def menubar_stage():
    img = furl(os.path.join(SRC, "popover.png"))
    return f"""<div style="width:740px">
      <div style="height:50px;background:rgba(248,248,250,0.95);border:1px solid #DCDCDF;border-radius:13px;
        display:flex;align-items:center;justify-content:flex-end;gap:22px;padding:0 24px;
        box-shadow:0 3px 10px rgba(0,0,0,0.06)">
        <span style="color:#6B6B70;font-size:24px;font-weight:600">⌘⌥G</span>
        <span style="display:inline-flex;align-items:center;gap:9px">
          <span style="width:17px;height:17px;border-radius:5px;background:{RED}"></span>
          <span style="color:#15171C;font-weight:700;font-size:24px">2</span></span></div>
      <div style="margin:20px auto 0;width:610px;border-radius:24px;overflow:hidden;
        box-shadow:0 50px 110px rgba(18,22,32,0.32),0 12px 32px rgba(18,22,32,0.20)">
        <img src="{img}" style="display:block;width:100%"></div></div>"""


# ── phone frame (mobile) ─────────────────────────────────────────────
def phone_stage():
    img = furl(os.path.join(SRC, "radar.png"))   # already has its own status bar + island
    return f"""<div style="background:#0A0A0C;border-radius:80px;padding:15px;
      box-shadow:0 50px 110px rgba(18,22,32,0.32),0 12px 32px rgba(18,22,32,0.20)">
      <div style="width:456px;height:991px;border-radius:66px;overflow:hidden;background:#000">
        <img src="{img}" style="width:100%;height:100%;object-fit:cover;display:block"></div></div>"""


def render(name, html):
    hp = os.path.join(HERE, f"_{name}.html")
    pp = os.path.join(OUT, f"surface-{name}.png")
    with open(hp, "w") as fh:
        fh.write(html)
    subprocess.run([
        CHROME, "--headless=new", "--disable-gpu", "--hide-scrollbars",
        "--force-device-scale-factor=2",
        f"--screenshot={pp}", f"--window-size={W},{H}", furl(hp),
    ], check=True)
    os.remove(hp)
    print("rendered", pp)


if __name__ == "__main__":
    render("cli", page("CLI", CYAN, cli_stage()))
    render("menubar", page("Menu bar", RED, menubar_stage()))
    render("mobile", page("Mobile", GREEN, phone_stage()))
