#!/usr/bin/env node
// frame-creative — render the App Store creative asset: one 16:9 image per locale that
// serves both the product page header and the search results (Apple calls a 5244×2950
// PNG "universal": it is valid for both placements). Same method as frame-shots.mjs: an
// HTML page rendered by headless Chrome at the exact size, from the demo harness's raw
// captures, so the image can be re-made the day the UI moves.
//
//   node scripts/frame-creative.mjs --lang en --ipad .e2e-artifacts/appstore/ipad-en \
//     --phone .e2e-artifacts/appstore/en --out fastlane/creative/en-US
//
// It says the whole product in one picture: the iPad shows every tmux pane on the Mac,
// agents among plain shells, and the phone in front shows HQ, which watches all of them
// (the user's review, 2026-10-08: two phones of radar and reply said "coding agents" and
// left HQ out). Layout follows Apple's guidance: one idea, the product in use, short
// text, nothing that matters outside the centre. The header crops this image to 21:9 on
// some devices, about 350px off the top and bottom, so the devices stay inside that band.
// Words live in creative-captions.json beside this script.

import {execFileSync} from 'child_process';
import {mkdirSync, readFileSync, writeFileSync, existsSync, rmSync} from 'fs';
import {dirname, join, resolve} from 'path';
import {fileURLToPath} from 'url';

const HERE = dirname(fileURLToPath(import.meta.url));
const CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';

function arg(name, fallback) {
  const i = process.argv.indexOf(`--${name}`);
  return i > 0 ? process.argv[i + 1] : fallback;
}

const W = 5244;
const H = 2950;
// The 21:9 crop of a 16:9 frame keeps the middle 2247px; the devices stay inside it.
const SAFE = Math.round((H - (W * 9) / 21) / 2);
// iPadOS 26 draws a window-resize grip in the screenshot's bottom-right corner; a bottom
// strip this tall hides it without touching the composer row above it.
const IPAD_CROP = 0.045;

const lang = arg('lang', 'en');
const ipadDir = resolve(arg('ipad', `.e2e-artifacts/appstore/ipad-${lang}`));
const phoneDir = resolve(arg('phone', `.e2e-artifacts/appstore/${lang}`));
const outDir = resolve(arg('out', 'fastlane/creative/en-US'));
const all = JSON.parse(readFileSync(join(HERE, 'creative-captions.json'), 'utf8'));
const c = all[lang];
if (!c) throw new Error(`no creative caption for locale ${lang} in creative-captions.json`);

const raw = (dir, stem) => {
  const src = join(dir, `${stem}.png`);
  if (!existsSync(src)) throw new Error(`missing raw capture: ${src}`);
  return readFileSync(src).toString('base64');
};

// The iPad sits inside the safe band; the phone stands in front of it and covers the
// iPad's sidebar exactly (it ends at 306/1400 of the screen), so no slivers of the radar
// peek out beside the phone.
const padH = 1880;
const padBezel = 32;
const padScrH = padH - 2 * padBezel;
const padScrW = Math.round(((padScrH / (1 - IPAD_CROP)) * 2752) / 2064);
const padW = padScrW + 2 * padBezel;
const SIDEBAR = 306 / 1400;
const phoneH = Math.round(padH * 0.88);
const phoneW = Math.round((phoneH * 1320) / 2868);
const ps = phoneW / 1128;
const padLeft = phoneW - (padBezel + Math.round(SIDEBAR * padScrW) + 36);

const page = (ipadB64, phoneB64) => `<!doctype html><html><head><meta charset="utf-8"><style>
  html, body { margin: 0; padding: 0; }
  body {
    width: ${W}px; height: ${H}px; overflow: hidden; background: #EFEFF2;
    font-family: -apple-system, "SF Pro Display", "PingFang SC", system-ui, sans-serif;
    display: flex; align-items: center; justify-content: center; gap: 220px;
  }
  .cap { width: 1780px; }
  .t { font-size: 140px; font-weight: 700; letter-spacing: -3.5px; color: #1D1D1F; line-height: 1.1; text-wrap: balance; }
  .d { display: inline-block; width: 42px; height: 42px; border-radius: 21px; margin-right: 36px; vertical-align: 24px; }
  .s { font-size: 76px; line-height: 1.38; color: rgba(60,60,67,0.62); margin-top: 60px; text-wrap: pretty; }
  .stage { position: relative; width: ${padLeft + padW}px; height: ${padH}px; }
  .pad {
    position: absolute; left: ${padLeft}px; top: 0; border-radius: 88px; background: #0A0A0C; padding: ${padBezel}px;
    box-shadow: 0 40px 120px rgba(0,0,0,0.16);
  }
  .padscr { width: ${padScrW}px; height: ${padScrH}px; border-radius: 58px; overflow: hidden; }
  .padscr img { display: block; width: 100%; }
  .phone {
    position: absolute; left: 0; bottom: -60px; width: ${phoneW}px; border-radius: ${Math.round(108 * ps)}px;
    background: #0A0A0C; padding: ${Math.round(18 * ps)}px; box-shadow: 0 50px 140px rgba(0,0,0,0.28);
  }
  .phone img { display: block; width: 100%; border-radius: ${Math.round(92 * ps)}px; }
</style></head><body>
  <div class="cap"><div class="t"><span class="d" style="background:${c.dot}"></span>${c.title.split('\n').join('<br>')}</div><div class="s">${c.sub}</div></div>
  <div class="stage">
    <div class="pad"><div class="padscr"><img src="data:image/png;base64,${ipadB64}"></div></div>
    <div class="phone"><img src="data:image/png;base64,${phoneB64}"></div>
  </div>
</body></html>`;

mkdirSync(outDir, {recursive: true});
const html = join(outDir, '_creative.html');
writeFileSync(html, page(raw(ipadDir, c.ipad), raw(phoneDir, c.phone)));
const out = join(outDir, 'creative-16x9.png');
execFileSync(CHROME, [
  '--headless', '--disable-gpu', '--hide-scrollbars', '--force-device-scale-factor=1',
  `--window-size=${W},${H}`, `--screenshot=${out}`,
  '--virtual-time-budget=3000', `file://${html}`,
], {stdio: 'ignore'});
rmSync(html);
// eslint-disable-next-line no-console
console.log(`${out}  (${W}×${H}, 21:9 safe band ${SAFE}px top and bottom)  “${c.title.replace('\n', ' ')}”`);
