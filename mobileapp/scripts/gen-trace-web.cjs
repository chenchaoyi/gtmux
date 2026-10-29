const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const ts = require('typescript');
const root = path.resolve(__dirname, '../..');
const source = fs.readFileSync(
  path.join(root, 'mobileapp/src/trace/model.ts'),
  'utf8',
);
const hash = crypto.createHash('sha256').update(source).digest('hex');
const js = ts.transpileModule(source, {
  compilerOptions: {
    target: ts.ScriptTarget.ES2020,
    module: ts.ModuleKind.CommonJS,
  },
}).outputText;
const body =
  '(function(root){var exports={};\n' +
  js +
  '\nif(typeof module==="object"&&module.exports)module.exports=exports;else root.GtmuxTrace=exports;})(typeof window!=="undefined"?window:globalThis);\n';
const output =
  '// Generated from mobileapp/src/trace/model.ts; SHA256 ' +
  hash +
  '\n// Body SHA256 ' +
  crypto.createHash('sha256').update(body).digest('hex') +
  '\n' +
  body;
const dest = path.join(root, 'internal/server/web/trace-model.js');
if (process.argv.includes('--check')) {
  if (fs.readFileSync(dest, 'utf8') !== output) {
    console.error('Run node mobileapp/scripts/gen-trace-web.cjs');
    process.exit(1);
  }
} else fs.writeFileSync(dest, output);
