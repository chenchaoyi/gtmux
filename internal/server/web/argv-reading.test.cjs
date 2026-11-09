const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),vm=require('node:vm'),crypto=require('node:crypto');
test('ARGV-001 browser uses the same generated argv decoder and compact formatter',()=>{
 const ctx=vm.createContext({});ctx.window=ctx;
 for(const file of ['vendor/prettier-standalone.js','vendor/prettier-parser-babel.js','vendor/prettier-parser-html.js','vendor/prettier-parser-postcss.js','vendor/prettier-parser-yaml.js','code-format.js'])vm.runInContext(fs.readFileSync(path.join(__dirname,file),'utf8'),ctx);
 const original=JSON.stringify(['/bin/bash','-c',"python3 - <<'PY'\nx={\"n\":1};print(x)\nPY\necho done"]);
 const parts=ctx.GtmuxCodeFormat.codeParts(original,'shell');
 assert.deepEqual(Array.from(parts,p=>p.hint),['shell','shell','python']);
 assert.equal(parts[0].source,original);assert.equal(parts[2].source,'x={"n":1};print(x)\n');
 assert.match(ctx.GtmuxCodeFormat.formatCode(parts[2].source,'python'),/x = \{"n": 1\}\nprint\(x\)/);
 const source=fs.readFileSync(path.resolve(__dirname,'../../../mobileapp/src/ui/codeFormatting.ts'));
 assert.ok(fs.readFileSync(path.join(__dirname,'code-format.js'),'utf8').includes(crypto.createHash('sha256').update(source).digest('hex')));
});
