#!/usr/bin/env node
// frame-shots — turn raw simulator captures into App Store screenshots.
//
// The framed 1320×2868 PNGs under fastlane/screenshots had NO reproducible path in this
// repo: the raw captures came from the e2e demo harness, and the caption + device frame
// were added somewhere else. So the day the UI moved, nobody could re-make them without
// redoing that step by hand — and on 2026-09-10 they were a month and three redesigns old.
//
// This is that step, in the repo: an HTML page per shot, rendered headless at the exact
// store size. No dependency beyond a Chrome that is already on the machine.
//
//   node scripts/frame-shots.mjs --in .e2e-artifacts/appstore/en --lang en \
//     --out fastlane/screenshots/en-US
//   node scripts/frame-shots.mjs --slot ipad --in .e2e-artifacts/appstore/ipad-en --lang ipad-en \
//     --out fastlane/screenshots/en-US --prefix ipad-
//
// Captions live in shot-captions.json beside this script, keyed by locale and by the raw
// file's stem, so the words are reviewable in a diff rather than buried in a design file.

import {execFileSync} from 'child_process';
import {mkdirSync, readFileSync, writeFileSync, existsSync, rmSync} from 'fs';
import {basename, dirname, join, resolve} from 'path';
import {fileURLToPath} from 'url';

const HERE = dirname(fileURLToPath(import.meta.url));
const CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';

function arg(name, fallback) {
  const i = process.argv.indexOf(`--${name}`);
  return i > 0 ? process.argv[i + 1] : fallback;
}

// Two store slots, one script. `phone` is the 6.9" slot — Apple accepts one size for the
// whole iPhone family, and this is the largest, so it is the one to author. `ipad` is the
// 13" slot in landscape, which is how the iPad shell is used (change ipad-universal-app);
// its captures come from the iPad Pro 13" simulator and wear a tablet's bezel.
const SLOTS = {
  phone: {W: 1320, H: 2868, capW: 1080, capTop: 132, title: 76, sub: 40, devW: 1128, devTop: 96, bezel: 18, r: 108, rIn: 92},
  ipad: {W: 2752, H: 2064, capW: 2000, capTop: 84, title: 64, sub: 34, devW: 2420, devTop: 56, bezel: 22, r: 64, rIn: 44},
};
const slot = SLOTS[arg('slot', 'phone')];
if (!slot) throw new Error(`unknown slot; use --slot phone|ipad`);
const {W, H} = slot;

const inDir = resolve(arg('in', '.e2e-artifacts/appstore/en'));
const lang = arg('lang', 'en');
const outDir = resolve(arg('out', 'fastlane/screenshots/en-US'));
// A file-name prefix so two slots can share a locale folder (deliver tells them apart by size).
const prefix = arg('prefix', '');
const captions = JSON.parse(readFileSync(join(HERE, 'shot-captions.json'), 'utf8'));
const list = captions[lang];
if (!list) throw new Error(`no captions for locale ${lang} in shot-captions.json`);

mkdirSync(outDir, {recursive: true});

// The frame: a caption block, then the phone. The phone runs off the bottom edge on
// purpose — the screenshot is a window into the app, not a product photo of a handset.
const page = (title, sub, b64, dot) => `<!doctype html><html><head><meta charset="utf-8"><style>
  html, body { margin: 0; padding: 0; }
  body {
    width: ${W}px; height: ${H}px; overflow: hidden;
    background: #EFEFF2;
    font-family: -apple-system, "SF Pro Display", "PingFang SC", system-ui, sans-serif;
    display: flex; flex-direction: column; align-items: center;
  }
  .cap { width: ${slot.capW}px; padding-top: ${slot.capTop}px; text-align: center; }
  .t { font-size: ${slot.title}px; font-weight: 700; letter-spacing: -1.6px; color: #1D1D1F; line-height: 1.14; text-wrap: balance; }
  /* The status dot the app uses, carried onto the store page (DESIGN §9). */
  .d { display: inline-block; width: 22px; height: 22px; border-radius: 11px; margin-right: 20px; vertical-align: 13px; }
  .s { font-size: ${slot.sub}px; line-height: 1.42; color: rgba(60,60,67,0.62); margin-top: 26px; text-wrap: balance; }
  /* The device. Bezel and radius are the phone's, so the shot reads as the real thing. */
  .dev {
    margin-top: ${slot.devTop}px; width: ${slot.devW}px; border-radius: ${slot.r}px; background: #0A0A0C;
    padding: ${slot.bezel}px; box-shadow: 0 30px 90px rgba(0,0,0,0.18);
  }
  .scr { border-radius: ${slot.rIn}px; overflow: hidden; display: block; width: 100%; }
</style></head><body>
  <div class="cap"><div class="t"><span class="d" style="background:${dot}"></span>${title}</div><div class="s">${sub}</div></div>
  <div class="dev"><img class="scr" src="data:image/png;base64,${b64}"></div>
</body></html>`;

let n = 0;
for (const shot of list) {
  const src = join(inDir, `${shot.file}.png`);
  if (!existsSync(src)) throw new Error(`missing raw capture: ${src}`);
  const b64 = readFileSync(src).toString('base64');
  const html = join(inDir, `_frame-${shot.file}.html`);
  writeFileSync(html, page(shot.title, shot.sub, b64, shot.dot || '#8E8E93'));
  n += 1;
  const out = join(outDir, `${prefix}${String(n).padStart(2, '0')}.png`);
  execFileSync(CHROME, [
    '--headless', '--disable-gpu', '--hide-scrollbars',
    `--window-size=${W},${H}`, `--screenshot=${out}`,
    '--virtual-time-budget=3000', `file://${html}`,
  ], {stdio: 'ignore'});
  rmSync(html);
  // eslint-disable-next-line no-console
  console.log(`${basename(out)}  ←  ${shot.file}  “${shot.title}”`);
}
// eslint-disable-next-line no-console
console.log(`framed ${n} shots into ${outDir}`);
