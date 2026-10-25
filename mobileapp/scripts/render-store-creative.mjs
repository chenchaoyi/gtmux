// Placement-specific Store art, using complete real synthetic-demo captures.
import {execFileSync} from 'node:child_process';
import {mkdirSync, mkdtempSync, readFileSync, writeFileSync, rmSync, existsSync} from 'node:fs';
import {dirname, join, resolve} from 'node:path';
import {tmpdir} from 'node:os';
import {fileURLToPath, pathToFileURL} from 'node:url';
import {CREATIVE_LAYOUTS, assertSafeGeometry} from './creative-layout.mjs';

const HERE = dirname(fileURLToPath(import.meta.url));
const CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const arg = (name, fallback) => {
  const index = process.argv.indexOf(`--${name}`);
  return index > 0 ? process.argv[index + 1] : fallback;
};
const lang = arg('lang', 'en');
const caption = JSON.parse(readFileSync(join(HERE, 'creative-captions.json')))[lang];
if (!caption) throw new Error(`Missing creative captions for ${lang}`);
const out = resolve(arg('out', 'fastlane/creative/en-US'));
const raw = (dir, name) => {
  const path = join(dir, name + '.png');
  if (!existsSync(path)) throw new Error(`Missing demo capture: ${path}`);
  return `data:image/png;base64,${readFileSync(path).toString('base64')}`;
};
const phone = raw(resolve(arg('phone', `.e2e-artifacts/appstore/${lang}`)), caption.phone);
const tablet = raw(resolve(arg('ipad', `.e2e-artifacts/appstore/ipad-${lang}`)), caption.ipad);
const escape = text => text.replace(/[&<>"']/g, ch => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'}[ch]));
const box = r => `left:${r.x}px;top:${r.y}px;width:${r.width}px;height:${r.height}px`;

function page(type, layout) {
  const text = type === 'header'
    ? `<div class="name" data-key="name">gtmux</div><div class="label" data-key="label">${escape(caption.header)}</div>`
    : `<div class="name" data-key="name">gtmux</div><div class="title" data-key="title">${escape(caption.title).replaceAll('\n', '<br>')}</div><div class="sub" data-key="sub">${escape(caption.sub)}</div>`;
  return `<!doctype html><html lang="${lang}"><meta charset="utf-8"><style>
html,body{margin:0;width:${layout.width}px;height:${layout.height}px;background:#EFEFF2;overflow:hidden}
body{font-family:-apple-system,"SF Pro Display","PingFang SC",system-ui,sans-serif;color:#1D1D1F}
.brand,.stage{position:absolute}.brand{${box(layout.brand)}}.stage{${box(layout.stage)}}
.name{font-size:${type === 'header' ? 148 : 64}px;font-weight:700;letter-spacing:-4px;line-height:1.1}
.label{font-size:46px;color:#55565E;margin-top:32px;line-height:1.3}
.title{font-size:90px;font-weight:700;letter-spacing:-2px;line-height:1.13;margin-top:28px;text-wrap:balance}
.sub{font-size:44px;line-height:1.4;color:#55565E;margin-top:32px;text-wrap:pretty}
.device{position:absolute;background:#101114;box-shadow:0 24px 70px #0002;box-sizing:content-box}
.tablet{left:${layout.tabletLeft}px;top:${layout.tabletTop}px;width:${layout.tablet}px;padding:16px;border-radius:42px}
.tablet img{border-radius:27px}.phone{left:0;bottom:0;width:${layout.phone}px;padding:10px;border-radius:36px}
.phone img{border-radius:26px}img{display:block;width:100%;height:auto}
</style><body class="${type}"><div class="brand">${text}</div><div class="stage">
<div class="device tablet" data-key="tablet"><img src="${tablet}"></div>
<div class="device phone" data-key="phone"><img src="${phone}"></div></div>
<script>Promise.all([document.fonts.ready,...[...document.images].map(i=>i.decode())]).then(()=>{
 const rectangles={};let overflow=false;
 for(const el of document.querySelectorAll('[data-key]')){const r=el.getBoundingClientRect();rectangles[el.dataset.key]={x:r.x,y:r.y,width:r.width,height:r.height};overflow ||= el.scrollWidth>el.clientWidth+1;}
 const data=document.createElement('script');data.type='application/json';data.id='geometry';data.textContent=JSON.stringify({rectangles,overflow});document.body.append(data);
});</script></body></html>`;
}

mkdirSync(out, {recursive: true});
const scratch = mkdtempSync(join(tmpdir(), 'gtmux-creative-'));
try {
  for (const [type, layout] of Object.entries(CREATIVE_LAYOUTS)) {
    const html = join(scratch, type + '.html');
    writeFileSync(html, page(type, layout));
    const common = ['--headless', '--disable-gpu', '--hide-scrollbars', '--force-device-scale-factor=1',
      `--user-data-dir=${join(scratch, 'chrome-' + type)}`, `--window-size=${layout.width},${layout.height + 128}`,
      '--virtual-time-budget=3000', pathToFileURL(html).href];
    const dom = execFileSync(CHROME, ['--dump-dom', ...common], {encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'], maxBuffer: 32 * 1024 * 1024});
    const match = dom.match(/<script type="application\/json" id="geometry">([^<]+)<\/script>/);
    if (!match) throw new Error('Creative geometry was not measured');
    const measured = JSON.parse(match[1]);
    const required = ['name', 'phone', 'tablet', ...(type === 'header' ? ['label'] : ['title', 'sub'])];
    if (required.some(key => !measured.rectangles[key])) throw new Error('Incomplete creative geometry');
    if (measured.overflow) throw new Error('Creative text overflows its container');
    assertSafeGeometry(layout, measured.rectangles);
    const image = join(out, layout.file);
    execFileSync(CHROME, [`--screenshot=${image}`, ...common], {stdio: 'ignore'});
    execFileSync('/usr/bin/sips', ['--cropOffset', '0', '0', '--cropToHeightWidth', String(layout.height), String(layout.width), image], {stdio: 'ignore'});
    const head = readFileSync(image).subarray(0, 26);
    if (head[25] !== 2) throw new Error('Store artwork must be an RGB PNG without alpha');
    writeFileSync(join(out, type + '-geometry.json'), JSON.stringify({canvas: layout, ...measured}, null, 2) + '\n');
    console.log(`${layout.file}: ${layout.width}×${layout.height}; all key content inside Apple's safe area`);
  }
} finally {
  rmSync(scratch, {recursive: true, force: true});
}
