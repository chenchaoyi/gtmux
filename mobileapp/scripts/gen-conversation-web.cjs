// The browser consumes the App's document validation contract without a runtime build.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const ts = require('typescript');
const root = path.resolve(__dirname, '../..');
const source = fs.readFileSync(path.join(root, 'mobileapp/src/documents/references.ts'), 'utf8');
const hash = crypto.createHash('sha256').update(source).digest('hex');
const js = ts.transpileModule(source, {compilerOptions: {target: ts.ScriptTarget.ES2020, module: ts.ModuleKind.CommonJS}}).outputText;
const output = '// Generated from mobileapp/src/documents/references.ts; SHA256 ' + hash + '\n(function(root){var exports={};\n' + js + '\nif(typeof module==="object"&&module.exports)module.exports=exports;else root.GtmuxDocuments=exports;})(typeof window!=="undefined"?window:globalThis);\n';
const dest = path.join(root, 'internal/server/web/documents.js');
if (process.argv.includes('--check')) {
  if (fs.readFileSync(dest,'utf8') !== output) {console.error('Run node mobileapp/scripts/gen-conversation-web.cjs');process.exit(1);}
} else fs.writeFileSync(dest, output);
