// Copies the xterm.js runtime files the browser mirror needs into
// internal/server/web/vendor/ (committed, so the Go //go:embed has no build-time
// node_modules dependency). Re-run after bumping @xterm/* in mobileapp/.
//   node mobileapp/scripts/gen-web-assets.mjs
import {readFileSync, writeFileSync, mkdirSync} from 'node:fs';
import {dirname, resolve} from 'node:path';
import {fileURLToPath} from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const nm = resolve(here, '..', 'node_modules', '@xterm');
const out = resolve(here, '..', '..', 'internal', 'server', 'web', 'vendor');
mkdirSync(out, {recursive: true});

const files = [
  ['xterm/lib/xterm.js', 'xterm.js'],
  ['xterm/css/xterm.css', 'xterm.css'],
  ['addon-fit/lib/addon-fit.js', 'addon-fit.js'],
  ['addon-unicode11/lib/addon-unicode11.js', 'addon-unicode11.js'],
];
for (const [src, dst] of files) {
  writeFileSync(resolve(out, dst), readFileSync(resolve(nm, src)));
  console.log('vendored', dst);
}
