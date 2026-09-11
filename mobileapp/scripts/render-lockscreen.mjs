#!/usr/bin/env node
// render-lockscreen — the one App Store shot that cannot be captured.
//
// The Live Activity and the push notification are two of the product's core moves and
// neither reached the store page, because neither can be photographed here: ActivityKit
// refuses to create an activity in a simulator build (no aps-environment — simulator
// builds carry no entitlements, even signed ad-hoc), and `simctl` cannot reach the lock
// screen at all. Capturing it on a real phone would show the operator's own session
// names, which is exactly what the demo exists to avoid.
//
// So this one is DRAWN — and it READS the card rather than copying it: every size and
// colour below comes out of ios/GtmuxWidget/GtmuxWidget.swift at render time (see
// widget-tokens.mjs). Change a size in Swift and the next render moves with it; refactor
// the line out of recognition and this throws instead of drawing yesterday's card.
//
// The first version of this file copied those values by hand and the note here said drift
// could not be caught automatically. It could: they are all literals, in shapes a regex
// can find. What still cannot be linked is what is INSIDE a band beyond its literals —
// `widget-tokens --check` pins the ORDER of the bands, and the rest is a human's job.
//
//   node scripts/render-lockscreen.mjs --lang en --out .e2e-artifacts/appstore/en
//
// It writes `02-lockscreen.png` at the same size as a Pro Max capture, so the framing
// step downstream treats it exactly like the real ones.

import {execFileSync} from 'child_process';
import {mkdirSync, writeFileSync, rmSync} from 'fs';
import {join, resolve} from 'path';
import {widgetTokens} from './widget-tokens.mjs';

const CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const W = 1320;
const H = 2868;
const S = 3; // the widget's points → these pixels

function arg(name, fallback) {
  const i = process.argv.indexOf(`--${name}`);
  return i > 0 ? process.argv[i + 1] : fallback;
}
const lang = arg('lang', 'en');
const outDir = resolve(arg('out', '.e2e-artifacts/appstore/en'));
const zh = lang === 'zh';
const t = (en, cn) => (zh ? cn : en);

// Read the card, do not copy it.
const K = widgetTokens();
const WAITING = K.waiting;
const WORKING = K.working;
const IDLE = K.idle;

const px = n => `${n * S}px`;

// One session row: badge, title, and the elapsed time SwiftUI renders locally.
const row = (color, title, time, ring) => `
  <div style="display:flex;align-items:center;gap:${px(K.rowGap)};">
    <div style="width:${px(K.badgeRow)};height:${px(K.badgeRow)};border-radius:${px(K.badgeRow / 2)};background:${color};display:flex;align-items:center;justify-content:center;flex-shrink:0;">
      ${ring ? `<div style="width:${px(6)};height:${px(6)};border-radius:${px(3)};border:${px(1.4)} solid #fff;border-right-color:transparent;box-sizing:border-box;"></div>` : ''}
    </div>
    <div style="flex-grow:1;min-width:0;font-size:${px(15)};color:rgba(255,255,255,0.92);white-space:nowrap;overflow:hidden;text-overflow:ellipsis;">${title}</div>
    <div style="font-size:${px(12)};color:rgba(255,255,255,0.5);font-variant-numeric:tabular-nums;">${time}</div>
  </div>`;

const html = `<!doctype html><html><head><meta charset="utf-8"><style>
  html,body{margin:0;padding:0;}
  body{width:${W}px;height:${H}px;overflow:hidden;
    font-family:-apple-system,"SF Pro Display","PingFang SC",system-ui,sans-serif;
    /* A plain graded ground, not a stock photo: the card is the subject. */
    background:radial-gradient(120% 90% at 50% 0%, #23252C 0%, #14151A 46%, #0B0C0F 100%);
    color:#fff;display:flex;flex-direction:column;align-items:center;}
  .num{font-variant-numeric:tabular-nums;}
</style></head><body>

  <div style="margin-top:${px(58)};text-align:center;">
    <div style="font-size:${px(20)};color:rgba(255,255,255,0.72);letter-spacing:${px(0.3)};">${t('Tuesday, 10 September', '9 月 10 日 星期二')}</div>
    <div class="num" style="font-size:${px(78)};font-weight:270;letter-spacing:${px(-2)};line-height:1.05;margin-top:${px(2)};">21:47</div>
  </div>

  <!-- The push: title = agent + needs you, subtitle = which Mac, body = the task. -->
  <div style="width:${px(370)};margin-top:${px(34)};border-radius:${px(22)};background:rgba(48,48,52,0.72);backdrop-filter:blur(${px(20)});padding:${px(13)} ${px(15)};display:flex;gap:${px(11)};align-items:flex-start;">
    <img src="ICON" style="width:${px(38)};height:${px(38)};border-radius:${px(9)};display:block;flex-shrink:0;">
    <div style="flex-grow:1;min-width:0;">
      <div style="display:flex;align-items:baseline;gap:${px(8)};">
        <div style="flex-grow:1;font-size:${px(15)};font-weight:600;">${t('Claude Code needs you', 'Claude Code 在等你')}</div>
        <div style="font-size:${px(12)};color:rgba(255,255,255,0.5);">${t('now', '现在')}</div>
      </div>
      <div style="font-size:${px(14)};color:rgba(255,255,255,0.62);margin-top:${px(1)};">MacBook Pro</div>
      <div style="font-size:${px(15)};color:rgba(255,255,255,0.92);margin-top:${px(3)};line-height:1.3;">${t('run the test suite?', '要跑一遍测试吗？')}</div>
    </div>
  </div>

  <!-- The Live Activity, at the widget's own geometry. -->
  <div style="width:${px(370)};margin-top:${px(20)};border-radius:${px(22)};overflow:hidden;background:rgba(0,0,0,0.55);">
    <div style="background:linear-gradient(180deg, rgba(239,68,68,${K.tintTop}) 0%, rgba(239,68,68,0.03) 46%, rgba(255,255,255,0.02) 100%);padding:${px(K.padV)} ${px(K.padH)};display:flex;flex-direction:column;gap:${px(K.cardGap)};">

      <div style="display:flex;align-items:center;gap:${px(K.serverGap)};">
        <div style="width:${px(K.serverDot)};height:${px(K.serverDot)};border-radius:${px(K.serverDot / 2)};background:${WAITING};"></div>
        <div style="font-size:${px(12)};font-weight:600;color:rgba(255,255,255,0.6);">MacBook Pro</div>
        <div style="flex-grow:1;"></div>
        <div class="num" style="display:flex;align-items:center;gap:${px(10)};font-size:${px(11)};font-weight:600;color:rgba(255,255,255,0.85);">
          <span style="display:flex;align-items:center;gap:${px(4)};"><span style="width:${px(12)};height:${px(12)};border-radius:${px(3.4)};background:${WAITING};"></span>1</span>
          <span style="display:flex;align-items:center;gap:${px(4)};"><span style="width:${px(12)};height:${px(12)};border-radius:${px(6)};background:${WORKING};"></span>3</span>
          <span style="display:flex;align-items:center;gap:${px(4)};opacity:0.3;"><span style="width:${px(12)};height:${px(12)};border-radius:${px(6)};background:${IDLE};"></span>6</span>
        </div>
        <img src="ICON" style="width:${px(K.brand)};height:${px(K.brand)};border-radius:${px(K.brand * 0.22)};display:block;margin-left:${px(9)};">
      </div>

      <div style="display:flex;align-items:center;gap:${px(K.bandGap)};">
        <div style="width:${px(K.badgeBig)};height:${px(K.badgeBig)};border-radius:${px(K.badgeBig * 0.28)};background:${WAITING};display:flex;align-items:center;justify-content:center;gap:${px(4)};flex-shrink:0;">
          <div style="width:${px(2.9)};height:${px(11.4)};border-radius:${px(1.4)};background:#fff;"></div>
          <div style="width:${px(2.9)};height:${px(11.4)};border-radius:${px(1.4)};background:#fff;"></div>
        </div>
        <div style="flex-grow:1;min-width:0;">
          <div style="display:flex;align-items:baseline;gap:${px(10)};">
            <div style="flex-grow:1;min-width:0;font-size:${px(K.titleSize)};font-weight:700;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;">api</div>
            <div class="num" style="font-size:${px(K.timerSize)};font-weight:700;color:${WAITING};">4:12</div>
          </div>
          <div style="font-size:${px(K.detailSize)};color:rgba(255,255,255,0.7);margin-top:${px(1)};white-space:nowrap;overflow:hidden;text-overflow:ellipsis;">${t('run the test suite?', '要跑一遍测试吗？')}</div>
        </div>
      </div>

      <div style="height:1px;background:rgba(255,255,255,0.09);"></div>

      <div style="display:flex;flex-direction:column;gap:${px(K.rowGap)};">
        ${row(WORKING, 'web', '12:31', true)}
        ${row(WORKING, 'worker', '3:04', true)}
      </div>

    </div>
  </div>

</body></html>`;

mkdirSync(outDir, {recursive: true});
const iconB64 = execFileSync('base64', ['-i', resolve('ios/GtmuxWidget/Assets.xcassets/BrandIcon.imageset/brand-192.png')])
  .toString().replace(/\n/g, '');
const page = join(outDir, '_lockscreen.html');
writeFileSync(page, html.replace(/ICON/g, `data:image/png;base64,${iconB64}`));
const out = join(outDir, '02-lockscreen.png');
execFileSync(CHROME, ['--headless', '--disable-gpu', '--hide-scrollbars',
  `--window-size=${W},${H}`, `--screenshot=${out}`, '--virtual-time-budget=3000', `file://${page}`],
  {stdio: 'ignore'});
rmSync(page);
// eslint-disable-next-line no-console
console.log(`drawn ${out}  (${lang})`);
