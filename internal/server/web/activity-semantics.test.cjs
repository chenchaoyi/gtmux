const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs'),path=require('node:path');
const artifact=process.env.GTMUX_ACTIVITY_ARTIFACT||__dirname;
const activity=require(path.join(artifact,'activity.js'));
const trace=require(path.join(artifact,'trace-model.js'));

test('AOP001 generated browser presenter shows read/search while retaining actual calls',()=>{
 for(const [operation,title] of [['read','Read file'],['search','Search'],['list','List files'],['command','Run command'],['code','Run code']]){
  const step={title:'exec',operation,call:{name:'exec',provider:'synthetic'},input:'raw input'};const original=JSON.stringify(step);
  assert.equal(trace.callTitle(step,'en'),title);assert.equal(JSON.stringify(step),original);assert.equal(trace.callFacts(step,'en')[0].value,'exec');
 }
 const events=trace.build([{segments:[{steps:[{title:'functions.exec',operation:'code'},{title:'exec',operation:'read',detail:'one.md, two.md'}]}]}]).events;
 assert.equal(events.filter(e=>e.kind==='tool').length,2);
});
test('AOP002 browser command preview unwraps complete shell argv only for display',()=>{
 const input=JSON.stringify(['/bin/bash','-c','python3 - <<\'PY\'\nprint(1)\nPY']);
 const step={title:'exec',operation:'command',input_kind:'arguments',input};
 assert.equal(activity.preview(step,'en'),"python3 - <<'PY' print(1) PY");assert.equal(step.input,input);
 assert.equal(activity.preview({...step,input:'["bash",'},'en'),'["bash",');
 assert.equal(activity.preview({...step,operation:'search',detail:'needle · src'},'en'),'needle · src');
});
test('AOP003 compact density is pointer-scoped and focus/overflow affordances remain',()=>{
 const css=fs.readFileSync(path.join(artifact,'style.css'),'utf8');
 assert.ok(/@media\s*\(hover:\s*hover\)\s*and\s*\(pointer:\s*fine\)\s*\{\s*\.tool-steps \.step-toggle\s*\{[^}]*min-height:\s*30px/.test(css), 'fine-pointer density override missing');
 assert.match(css,/\.step-toggle[^}]*min-height:\s*44px/);
 assert.match(css,/\.step-toggle:focus-visible/);assert.match(css,/\.step-detail[^}]*text-overflow:\s*ellipsis/);
});
