#!/usr/bin/env node
// One reader stylesheet; platform Markdown parsers remain native adapters.
const fs = require('node:fs');
const path = require('node:path');
const base = path.resolve(__dirname, '..');
const css = fs.readFileSync(path.join(base, 'shared/documents/reader.css'), 'utf8');
const swift = path.join(base, 'ios/Documents/DocumentMarkdown.swift');
const source = fs.readFileSync(swift, 'utf8');
const begin = '  // gtmux:document-css:begin';
const end = '  // gtmux:document-css:end';
const a = source.indexOf(begin), z = source.indexOf(end, a);
if (a < 0 || z < 0 || !css.includes('__FONT_SIZE__')) throw new Error('Reader style markers are missing');
const replacement = `${begin}\n  static let readerCSS = """\n${css.trimEnd().split('\n').map(line => '  ' + line).join('\n')}\n  """\n${end}`;
const output = new Map([
  [swift, source.slice(0, a) + replacement + source.slice(z + end.length)],
  [path.join(base, 'android/app/src/main/res/raw/document_reader.css'), css],
]);
let drift = false;
for (const [file, value] of output) {
  if (fs.existsSync(file) && fs.readFileSync(file, 'utf8') === value) continue;
  if (process.argv.includes('--check')) { console.error(`Document stylesheet differs: ${path.relative(base, file)}`); drift = true; }
  else fs.writeFileSync(file, value);
}
if (drift) process.exitCode = 1;
