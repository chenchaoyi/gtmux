const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');
const source=fs.readFileSync(path.join(__dirname,'app.js'),'utf8');
const formattingContext={};
formattingContext.window=formattingContext;
vm.createContext(formattingContext);
for(const file of ['vendor/prettier-standalone.js','vendor/prettier-parser-babel.js','vendor/prettier-parser-html.js','vendor/prettier-parser-postcss.js','vendor/prettier-parser-yaml.js','code-format.js'])vm.runInContext(fs.readFileSync(path.join(__dirname,file),'utf8'),formattingContext);
// Reuse the existing VM surface, without importing/registering its test cases.
const shared=fs.readFileSync(path.join(__dirname,'app.test.cjs'),'utf8');
const context={source,formattingContext,fs,path,vm,assert,require,setTimeout,clearTimeout,setInterval,clearInterval,console,AbortController};
vm.runInNewContext(shared.slice(shared.indexOf('function harness('),shared.indexOf('\nasync function submit('))+';globalThis.harness=harness;',context);

test('SID007 focused and tile history cannot dispatch input or become an external row',()=>{
 const h=context.harness({ok:true,json:async()=>[],headers:{get:()=>null}});
 const a={pane_id:'%fixture',agent:'Codex',source:'tmux',observed_conversation:{session_id:'old',home_id:'home',state:'last-known'}};
 // Set the real share state by the existing fixture's accessor.
 Object.assign(h.api.share(),{input:true,all:true,known:true});
 h.api.setPin(a.pane_id,a,'chat',[]);
 assert.equal(h.api.paneCanInput(a.pane_id),false);
 assert.match(h.api.nativeEvidenceLabel(a),/Association pending/);
 assert.equal(h.api.readableConversation(a),false);
 h.api.setPin(a.pane_id,a,'term',[]);assert.equal(h.api.paneCanInput(a.pane_id),true);
 h.api.setTiles([{id:a.pane_id,agent:a,mode:'chat',removed:false}]);assert.equal(h.api.paneCanInput(a.pane_id),false);
 h.api.setTiles([]);h.api.setPin(a.pane_id,{...a,observed_conversation:undefined},'chat',[]);assert.equal(h.api.paneCanInput(a.pane_id),true);
});
