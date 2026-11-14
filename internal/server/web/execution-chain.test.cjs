const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const source = fs.readFileSync(__dirname+'/app.js','utf8');
const body=source.slice(source.indexOf('  function groupProfileChain('),source.indexOf('  function renderProfileChain('));
const scope={};vm.runInNewContext(body,scope);
test('CRX-005 adjacent equal process presentations retain count, order and distinct arguments',()=>{
 for(const name of ['codex','claude','opencode']){
  const p={name,path:'/fixture/'+name};const input=[p,p,{name:'launcher'},p,{...p,arguments:['visible-option']}];const before=JSON.stringify(input);
  const groups=JSON.parse(JSON.stringify(scope.groupProfileChain(input)));
  assert.equal(groups.length,4);assert.equal(groups[0].count,2);assert.equal(groups[2].count,1);assert.deepEqual(groups[3].arguments,['visible-option']);
  assert.equal(JSON.stringify(input),before);
 }
});
