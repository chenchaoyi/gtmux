// Pinned, offline read-only formatters shared by the native and browser readers.
const fs=require('node:fs'),path=require('node:path'),ts=require('typescript'),crypto=require('node:crypto');
const root=path.resolve(__dirname,'../..'), dest=path.join(root,'internal/server/web');
const files=['standalone','parser-babel','parser-html','parser-postcss','parser-yaml'];
function output(file,body){if(process.argv.includes('--check')){if(!fs.existsSync(file)||!fs.readFileSync(file).equals(Buffer.from(body)))throw Error('Run node mobileapp/scripts/gen-code-format.cjs: '+file);}else fs.writeFileSync(file,body);}
for(const file of files)output(path.join(dest,'vendor/prettier-'+file+'.js'),fs.readFileSync(require.resolve('prettier/'+file)));
output(path.join(dest,'vendor/prettier.LICENSE'),fs.readFileSync(path.join(path.dirname(require.resolve('prettier/package.json')),'LICENSE')));
const source=fs.readFileSync(path.join(root,'mobileapp/src/ui/codeFormatting.ts'),'utf8');
const browser=source.replace(/const prettier:[\s\S]*?require\('prettier\/standalone'\);/,'const prettier = window.prettier;').replace(/const plugins = \[[\s\S]*?\];/,'const plugins = Object.values(window.prettierPlugins);');
const modules=['@lezer/common','@lezer/lr','@lezer/highlight','@lezer/python'];
let licenses='', bundled='var factories={},cache={};\nfunction require(name){if(cache[name])return cache[name].exports;var module={exports:{}};cache[name]=module;factories[name](require,module,module.exports);return module.exports;}\n';
for(const name of modules){
  const entry=require.resolve(name), folder=path.dirname(path.dirname(entry));
  const body=ts.transpileModule(fs.readFileSync(entry,'utf8'),{compilerOptions:{target:ts.ScriptTarget.ES2020,module:ts.ModuleKind.CommonJS,removeComments:true}}).outputText;
  bundled+='factories['+JSON.stringify(name)+']=function(require,module,exports){\n'+body+'\n};\n';
  licenses+='\n'+name+'\n'+fs.readFileSync(path.join(folder,'LICENSE'),'utf8');
}
output(path.join(dest,'vendor/lezer.LICENSE'),licenses);
output(path.join(dest,'code-format.js'),'// Generated code formatter; source SHA256 '+crypto.createHash('sha256').update(source).digest('hex')+'\n// Includes @lezer/*, MIT; see vendor/lezer.LICENSE.\n(function(){\n'+bundled+'var exports={};\n'+ts.transpileModule(browser,{compilerOptions:{target:ts.ScriptTarget.ES2020,module:ts.ModuleKind.CommonJS}}).outputText+'\nwindow.GtmuxCodeFormat=exports;})();\n');
