const fs=require('node:fs'),path=require('node:path'),ts=require('typescript'),crypto=require('node:crypto');
const root=path.resolve(__dirname,'../..');
for(const [name,global] of [['codeReading','GtmuxCodeReading'],['diffModel','GtmuxDiffModel']]){
 const source=fs.readFileSync(path.join(root,'mobileapp/src/ui/'+name+'.ts'),'utf8');
 const output='// Generated from mobileapp/src/ui/'+name+'.ts; SHA256 '+crypto.createHash('sha256').update(source).digest('hex')+'\n(function(root){var exports={};\n'+ts.transpileModule(source,{compilerOptions:{target:ts.ScriptTarget.ES2020,module:ts.ModuleKind.CommonJS}}).outputText+'\nif(typeof module==="object"&&module.exports)module.exports=exports;else root.'+global+'=exports;})(typeof window!=="undefined"?window:globalThis);\n';
 const dest=path.join(root,'internal/server/web/'+(name==='codeReading'?'code-reading':'diff-model')+'.js');
 if(process.argv.includes('--check')){if(fs.readFileSync(dest,'utf8')!==output)throw Error('Run node mobileapp/scripts/gen-reading-web.cjs');}else fs.writeFileSync(dest,output);
}
