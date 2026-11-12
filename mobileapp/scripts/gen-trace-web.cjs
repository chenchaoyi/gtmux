const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const ts = require('typescript');
const root = path.resolve(__dirname, '../..');
const inputs = require('../src/trace/shared-sources.json');
const sources = Object.fromEntries(
  Object.entries(inputs).map(([key, file]) => [
    key,
    fs.readFileSync(path.join(root, file), 'utf8'),
  ])
);
const hash = crypto
  .createHash('sha256')
  .update(JSON.stringify(sources))
  .digest('hex');
const modules = Object.entries(sources)
  .map(
    ([key, source]) =>
      JSON.stringify(key) +
      ':function(exports,require){\n' +
      ts.transpileModule(source, {
        compilerOptions: {
          target: ts.ScriptTarget.ES2020,
          module: ts.ModuleKind.CommonJS,
        },
      }).outputText +
      '\n}'
  )
  .join(',\n');
const body =
  '(function(root){var modules={' +
  modules +
  '},cache={};function load(key){if(cache[key])return cache[key];var exports={};cache[key]=exports;modules[key](exports,function(name){var id={"./model":"model","../ui/activity":"activity","./activity":"activity","./timeline":"timeline","../ui/toolPresentation":"toolPresentation"}[name];if(!id)throw new Error("Unknown shared trace module");return load(id);});return exports;}var api=Object.assign({},load("model"),load("presentation"),load("toolPresentation"));\nif(typeof module==="object"&&module.exports)module.exports=api;else root.GtmuxTrace=api;})(typeof window!=="undefined"?window:globalThis);\n';
const output =
  '// Generated from shared App trace analysis and presentation; SHA256 ' +
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

// Browser Activity uses the same preview/operation code as App and Trace.
const iconSource = fs.readFileSync(path.join(root, 'mobileapp/src/ui/ActivityIcon.tsx'), 'utf8');
const iconPaths = /const paths:[^=]+ = (\{[\s\S]*?\n\});/.exec(iconSource);
if (!iconPaths) throw Error('Activity icon table moved; review the shared generator');
const activityBody = ts.transpileModule(sources.activity, {compilerOptions: {target: ts.ScriptTarget.ES2020, module: ts.ModuleKind.CommonJS}}).outputText;
const activityOutput = '// Generated from shared Activity; SHA256 ' + crypto.createHash('sha256').update(sources.activity + iconPaths[1]).digest('hex') + '\n(function(root){var exports={};\n' + activityBody + '\nvar paths=' + iconPaths[1] + ';var api={operation:exports.activityOperation,label:exports.activityLabel,summary:exports.activitySummary,preview:exports.activityPreview,names:names,labels:labels,paths:paths};if(typeof module==="object"&&module.exports)module.exports=api;else root.GtmuxActivity=api;})(typeof window!=="undefined"?window:globalThis);\n';
const activityDest = path.join(root, 'internal/server/web/activity.js');
if (process.argv.includes('--check')) {
  if (fs.readFileSync(activityDest, 'utf8') !== activityOutput) throw Error('Run node mobileapp/scripts/gen-trace-web.cjs');
} else fs.writeFileSync(activityDest, activityOutput);
