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
//
// Captions live in shot-captions.json beside this script, keyed by locale and by the raw
// file's stem, so the words are reviewable in a diff rather than buried in a design file.

import {execFileSync} from 'child_process';
import {mkdirSync, readFileSync, writeFileSync, existsSync, rmSync} from 'fs';
import {basename, dirname, join, resolve} from 'path';
import {fileURLToPath} from 'url';

const HERE = dirname(fileURLToPath(import.meta.url));
const CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';

// The 6.9" App Store slot. Apple accepts one size for the whole iPhone family, and this
// is the largest — so it is the one to author.
const W = 1320;
const H = 2868;

function arg(name, fallback) {
  const i = process.argv.indexOf(`--${name}`);
  return i > 0 ? process.argv[i + 1] : fallback;
}

const inDir = resolve(arg('in', '.e2e-artifacts/appstore/en'));
const lang = arg('lang', 'en');
const outDir = resolve(arg('out', 'fastlane/screenshots/en-US'));
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
  .cap { width: 1080px; padding-top: 132px; text-align: center; }
  .t { font-size: 76px; font-weight: 700; letter-spacing: -1.6px; color: #1D1D1F; line-height: 1.14; text-wrap: balance; }
  /* The status dot the app uses, carried onto the store page (DESIGN §9). */
  .d { display: inline-block; width: 22px; height: 22px; border-radius: 11px; margin-right: 20px; vertical-align: 13px; }
  .s { font-size: 40px; line-height: 1.42; color: rgba(60,60,67,0.62); margin-top: 26px; text-wrap: balance; }
  /* The device. Bezel and radius are the phone's, so the shot reads as the real thing. */
  .dev {
    margin-top: 96px; width: 1128px; border-radius: 108px; background: #0A0A0C;
    padding: 18px; box-shadow: 0 30px 90px rgba(0,0,0,0.18);
  }
  .scr { border-radius: 92px; overflow: hidden; display: block; width: 100%; }
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
  const out = join(outDir, `${String(n).padStart(2, '0')}.png`);
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
