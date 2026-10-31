const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto'),vm=require('node:vm');
for(const [name,file] of [['codeReading','code-reading'],['diffModel','diff-model']])test(file+' follows its native model',()=>{
 const source=fs.readFileSync(path.resolve(__dirname,'../../../mobileapp/src/ui/'+name+'.ts'));
 assert.ok(fs.readFileSync(path.join(__dirname,file+'.js'),'utf8').includes(crypto.createHash('sha256').update(source).digest('hex')));
});
test('offline browser formatter and syntax reader retain complete inputs',()=>{
 const context=vm.createContext({});context.window=context;
 for(const file of ['vendor/prettier-standalone.js','vendor/prettier-parser-babel.js','vendor/prettier-parser-html.js','vendor/prettier-parser-postcss.js','vendor/prettier-parser-yaml.js','code-format.js','code-reading.js'])vm.runInContext(fs.readFileSync(path.join(__dirname,file),'utf8'),context);
 const source='const value={text:"'+ '鲸🙂'.repeat(8000)+'",count:1};text(value);';
 assert.equal(context.GtmuxCodeFormat.formatCode('plain',null),'plain');
 assert.equal(context.GtmuxCodeReading.codeLanguage('text',null),'text');
 const formatted=context.GtmuxCodeFormat.formatCode(source,'javascript');
 assert.ok(formatted.includes('鲸🙂'.repeat(8000)));assert.ok(formatted.includes('\n'));
 assert.equal(context.GtmuxCodeReading.codeTokens(formatted,'javascript').map(t=>t.text).join(''),formatted);
 assert.equal(context.GtmuxCodeFormat.formatCode('echo " x "\n','shell'),'echo " x "\n');
});
