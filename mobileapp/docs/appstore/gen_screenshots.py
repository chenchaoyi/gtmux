#!/usr/bin/env python3
"""
App Store screenshot compositor for gtmux (learns from Rodi's
docs/appstore/generate_screenshots.py, and shares the house style of
docs/marketing/gen.py). Wraps each surface in a marketing card: a bold caption
headline + a status accent dot, a device-framed phone screen, on a light branded
gradient — so the store shots SELL the value instead of just showing the UI.

Five shots, mapped to the App Store description's pillars:
  01 SEE       — the radar               (real capture)
  02 STEER     — reply / terminal input  (real capture)
  03 SUPERVISE — the HQ                   (real capture)
  04 NOTIFY    — a push with 1·2·3 reply  (faithful lock-screen MOCKUP)
  05 LIVE      — Live Activity            (faithful lock-screen MOCKUP)

Shots 04/05 are OS-level UI (a banner / a Live Activity on the Lock Screen) that a
plain in-app capture can't show; Apple allows composed marketing images, so they are
rendered as mockups faithful to the real UI (colors from GtmuxWidget's ActivityAttributes,
the AGENT_WAITING quick-reply actions from relay-worker).

Inputs : mobileapp/.e2e-artifacts/appstore/<lang>/{01-radar,02-terminal-approval,03-hq}.png
Outputs: mobileapp/fastlane/screenshots/<locale>/{01..05}.png   (1320x2868, en-US + zh-Hans)

    python3 mobileapp/docs/appstore/gen_screenshots.py
"""
import os
import subprocess

HERE = os.path.dirname(os.path.abspath(__file__))
APP = os.path.abspath(os.path.join(HERE, "..", ".."))          # mobileapp/
CAPS = os.path.join(APP, ".e2e-artifacts", "appstore")         # real captures, per lang
OUT = os.path.join(APP, "fastlane", "screenshots")             # fastlane deliver source
CHROME = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

# App Store 6.9" iPhone canvas. Rendered from a half-size logical canvas at 2x for
# crisp text — 660*2 = 1320, 1434*2 = 2868.
CW, CH = 660, 1434
RED, CYAN, GREEN, GRAY = "#EF4444", "#06B6D4", "#22C55E", "#8E8E93"

LOCALES = {"en": "en-US", "zh": "zh-Hans"}


def furl(p):
    return "file://" + p


# ── the marketing card: caption + device-framed screen on a branded gradient ──
def card(caption, subtitle, accent, screen_html):
    return f"""<!doctype html><html><head><meta charset="utf-8"><style>
*{{margin:0;padding:0;box-sizing:border-box}}
html,body{{width:{CW}px;height:{CH}px}}
body{{font-family:-apple-system,'SF Pro Display',Helvetica,Arial,sans-serif;
 background:radial-gradient(120% 62% at 50% -6%, #FFFFFF 0%, #EEF1F4 44%, #DFE4EA 100%);
 display:flex;flex-direction:column;align-items:center;overflow:hidden}}
/* fixed-height header so the phone starts at the SAME y on every shot (1- or 2-line
   captions don't shift it) and never overlaps the copy */
.head{{height:312px;flex:none;display:flex;flex-direction:column;justify-content:center;
 align-items:center;padding-top:16px}}
.cap{{font-size:53px;font-weight:800;color:#14171C;letter-spacing:-1.2px;
 display:flex;align-items:center;gap:20px;text-align:center;padding:0 40px;line-height:1.06}}
.cap .dot{{width:20px;height:20px;border-radius:50%;background:{accent};flex:none}}
.sub{{margin-top:20px;font-size:26px;font-weight:500;color:#5C636E;text-align:center;
 padding:0 72px;line-height:1.32;max-width:604px}}
.stage{{flex:1;width:100%;display:flex;align-items:flex-end;justify-content:center;min-height:0}}
/* phone bezel — screen sits flush at the canvas bottom, large */
.phone{{width:548px;background:#0A0A0C;border-radius:82px 82px 0 0;padding:16px 16px 0;
 box-shadow:0 40px 90px rgba(20,24,34,0.32),0 12px 30px rgba(20,24,34,0.18)}}
.screen{{width:516px;height:1106px;border-radius:66px 66px 0 0;overflow:hidden;background:#000;
 position:relative}}
.screen>img{{width:100%;display:block;object-fit:cover;object-position:top}}
</style></head><body>
<div class="head">
  <div class="cap"><span class="dot"></span><span>{caption}</span></div>
  {f'<div class="sub">{subtitle}</div>' if subtitle else ''}
</div>
<div class="stage"><div class="phone"><div class="screen">{screen_html}</div></div></div>
</body></html>"""


def screen_img(path):
    return f'<img src="{furl(path)}">'


# ── faithful lock-screen chrome (time + date), shared by the mockups ──────────
def lockscreen(inner, zh):
    date = "周二 7月28日" if zh else "Tuesday, July 28"
    return f"""<div style="width:100%;height:100%;
      background:linear-gradient(168deg,#1C2230 0%,#141821 52%,#0C0E14 100%);
      display:flex;flex-direction:column;align-items:center;color:#fff;
      font-family:-apple-system,'SF Pro Display',sans-serif">
      <div style="margin-top:104px;font-size:33px;font-weight:600;color:rgba(255,255,255,0.82)">{date}</div>
      <div style="font-size:142px;font-weight:600;letter-spacing:-3px;line-height:1.02;margin-top:2px">9:41</div>
      <div style="flex:1"></div>
      <div style="width:100%;padding:0 22px 40px">{inner}</div>
    </div>"""


# ── the gtmux brand mark (2x2 pane grid, top-right cyan) as inline SVG ────────
def brandmark(size, neutral="rgba(255,255,255,0.85)"):
    return f"""<svg width="{size}" height="{size}" viewBox="0 0 100 100">
      <rect x="8" y="8" width="40" height="40" rx="9" fill="{neutral}"/>
      <rect x="52" y="8" width="40" height="40" rx="9" fill="{CYAN}"/>
      <rect x="8" y="52" width="84" height="40" rx="9" fill="{neutral}"/></svg>"""


# ── 04 NOTIFY — a delivered push with the 1·2·3 quick-reply actions ──────────
def notify_mockup(zh):
    title = "api 在等你拍板" if zh else "api needs you"
    body = "permission to run tests"
    yes, always, no = ("1 好", "2 一律允许", "3 否") if zh else ("1 Yes", "2 Always", "3 No")
    now = "现在" if zh else "now"
    banner = f"""
    <div style="background:rgba(38,40,46,0.82);backdrop-filter:blur(30px);border-radius:34px;
      padding:26px 28px 22px;box-shadow:0 20px 50px rgba(0,0,0,0.35)">
      <div style="display:flex;align-items:center;gap:16px">
        <div style="width:64px;height:64px;border-radius:16px;background:#0C0E14;
          display:flex;align-items:center;justify-content:center">{brandmark(38)}</div>
        <div style="flex:1;min-width:0">
          <div style="display:flex;align-items:center;gap:10px">
            <span style="font-size:29px;font-weight:700">gtmux</span>
            <span style="width:14px;height:14px;border-radius:4px;background:{RED};display:inline-block"></span>
            <span style="margin-left:auto;font-size:24px;color:rgba(255,255,255,0.5)">{now}</span>
          </div>
          <div style="font-size:30px;font-weight:600;margin-top:3px">{title}</div>
          <div style="font-size:27px;color:rgba(255,255,255,0.72);margin-top:2px;white-space:nowrap;
            overflow:hidden;text-overflow:ellipsis">{body}</div>
        </div>
      </div>
      <div style="display:flex;gap:12px;margin-top:20px">
        {"".join(f'''<div style="flex:1;background:rgba(255,255,255,0.14);border-radius:16px;
          padding:15px 0;text-align:center;font-size:25px;font-weight:600">{a}</div>'''
          for a in (yes, always, no))}
      </div>
    </div>"""
    return lockscreen(banner, zh)


# ── 05 LIVE — the Live Activity card on the Lock Screen ──────────────────────
def liveactivity_mockup(zh):
    summ = "1 个在等你 · 1 个在跑" if zh else "1 waiting · 1 working"
    sess = "api"
    task = "permission to run tests"
    working = "web · refactor auth middleware"
    def row(dot_html, text, t):
        return f"""<div style="display:flex;align-items:center;gap:14px;padding:8px 0">
          {dot_html}
          <span style="font-size:27px;color:rgba(255,255,255,0.92);white-space:nowrap;
            overflow:hidden;text-overflow:ellipsis;flex:1">{text}</span>
          <span style="font-size:23px;color:rgba(255,255,255,0.5)">{t}</span></div>"""
    wait_badge = f'<span style="width:22px;height:22px;border-radius:6px;background:{RED};flex:none"></span>'
    work_badge = f'<span style="width:22px;height:22px;border-radius:50%;background:{CYAN};flex:none"></span>'
    card_html = f"""
    <div style="background:rgba(20,23,30,0.9);backdrop-filter:blur(30px);border-radius:34px;
      padding:26px 28px;box-shadow:0 20px 50px rgba(0,0,0,0.35)">
      <div style="display:flex;align-items:center;gap:14px;margin-bottom:6px">
        <div style="width:46px;height:46px;border-radius:12px;background:#0C0E14;
          display:flex;align-items:center;justify-content:center">{brandmark(28)}</div>
        <span style="font-size:31px;font-weight:800;letter-spacing:-0.5px">gtmux</span>
        <span style="margin-left:auto;font-size:28px;font-weight:700;color:{RED}">{summ}</span>
      </div>
      {row(wait_badge, f'<b>{sess}</b> — {task}', '4m')}
      <div style="height:1px;background:rgba(255,255,255,0.08)"></div>
      {row(work_badge, working, '1m')}
    </div>"""
    # a Dynamic Island pill floated at the very top, to say "and the Island too"
    island = f"""<div style="position:absolute;top:22px;left:50%;transform:translateX(-50%);
      background:#000;border-radius:20px;padding:9px 20px;display:flex;align-items:center;gap:11px;z-index:3">
      <span style="width:18px;height:18px;border-radius:5px;background:{RED}"></span>
      <span style="color:#fff;font-size:23px;font-weight:600">api needs you</span></div>"""
    return island + lockscreen(card_html, zh)


# ── the five shots × per-language caption copy ───────────────────────────────
SHOTS = [
    dict(name="01", accent=RED, cap=dict(en="See who needs you", zh="一眼看到谁在等你"),
         sub=dict(en="A live radar of every agent, color-coded by state.",
                  zh="每个 agent 的实时雷达，按状态着色。"),
         capture="01-radar"),
    dict(name="02", accent=CYAN, cap=dict(en="Reply from your phone", zh="在手机上回话"),
         sub=dict(en="Answer a prompt or send a command — without walking back.",
                  zh="回一句、或直接发命令——不用走回电脑。"),
         capture="02-terminal-approval"),
    dict(name="03", accent=GREEN, cap=dict(en="An HQ triages the rest", zh="中控替你分诊"),
         sub=dict(en="A supervisor watches the fleet and surfaces only what matters.",
                  zh="参谋长盯着全舰队，只把要紧的推给你。"),
         capture="03-hq"),
    dict(name="04", accent=RED, cap=dict(en="Pinged when it needs you", zh="该你出手时才提醒"),
         sub=dict(en="Reply 1·2·3 straight from the notification.",
                  zh="通知上直接 1·2·3 回应。"),
         mockup=notify_mockup),
    dict(name="05", accent=CYAN, cap=dict(en="Live on your Lock Screen", zh="锁屏实时可见"),
         sub=dict(en="The current agent's state on your Lock Screen and Dynamic Island.",
                  zh="当前 agent 状态，常驻锁屏与灵动岛。"),
         mockup=liveactivity_mockup),
]


def render(html, out_png):
    hp = out_png + ".html"
    with open(hp, "w") as fh:
        fh.write(html)
    subprocess.run([
        CHROME, "--headless=new", "--disable-gpu", "--hide-scrollbars",
        "--force-device-scale-factor=2",
        f"--screenshot={out_png}", f"--window-size={CW},{CH}", furl(hp),
    ], check=True, capture_output=True)
    os.remove(hp)


def main():
    for lang, locale in LOCALES.items():
        dest = os.path.join(OUT, locale)
        os.makedirs(dest, exist_ok=True)
        for s in SHOTS:
            if "capture" in s:
                cap_png = os.path.join(CAPS, lang, s["capture"] + ".png")
                screen = screen_img(cap_png)
            else:
                screen = s["mockup"](lang == "zh")
            html = card(s["cap"][lang], s["sub"][lang], s["accent"], screen)
            out = os.path.join(dest, f"{s['name']}.png")
            render(html, out)
            print("rendered", os.path.relpath(out, APP))


if __name__ == "__main__":
    main()
