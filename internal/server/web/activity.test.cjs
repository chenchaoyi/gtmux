const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const activity = require('./activity.js');

test('browser activity tables match the phone labels, aliases and icon family', () => {
  const ui = path.join(__dirname, '../../../mobileapp/src/ui');
  for (const [file, name, table] of [['activity.ts', 'names', activity.names], ['activity.ts', 'labels', activity.labels], ['ActivityIcon.tsx', 'paths', activity.paths]]) {
    const source = fs.readFileSync(path.join(ui, file), 'utf8');
    const literal = new RegExp('const ' + name + ':[^=]+ = (\\{[\\s\\S]*?\\n\\});').exec(source);
    assert.ok(literal, name + ' table moved; review cross-surface parity');
    assert.equal(JSON.stringify(table), JSON.stringify(vm.runInNewContext('(' + literal[1] + ')')));
  }
});

test('activity uses own known operations and qualified aliases without inventing outcomes', () => {
  assert.equal(activity.operation({operation:'wait',title:'exec_command'}), 'wait');
  assert.equal(activity.operation({title:'mcp__tools__spawn_agent'}), 'agent');
  assert.equal(activity.operation({title:'functions.exec_command'}), 'command');
  for (const title of ['toString', '__proto__', 'unknown']) assert.equal(activity.operation({title}), 'tool');
  assert.equal(activity.operation({operation:'constructor',title:'wait_agent'}), 'wait');
  assert.equal(activity.summary([{title:'exec'},{title:'bash'}], 'en'), 'Run command');
  assert.equal(activity.summary([{title:'exec'},{title:'wait'}], 'zh'), '执行过程');
});

test('delegation and wait previews explain only explicit recorded parameters', () => {
  const cases = [
    [{title:'spawn_agent',input_kind:'parameters',input:'{"message":"Review layout"}',detail:'{"message":"Review layout"}'},'Review layout','Review layout'],
    [{title:'delegate',input_kind:'arguments',input:'{"description":"Check behavior","message":"Other"}'},'Check behavior','Check behavior'],
    [{title:'wait_agent',input_kind:'parameters',input:'{"timeout_ms":30000}'},'Timeout setting 30s','超时设置 30 秒'],
    [{title:'wait',input_kind:'arguments',input:'{"yield_time_ms":0}'},'Up to 0s','最多 0 秒'],
    [{title:'wait',input_kind:'parameters',input:'{"duration_ms":2500}'},'Requested 2.5s','设定 2.5 秒'],
    [{title:'wait',input_kind:'parameters',input:'{"timeout":30}'},'Recorded parameters','已记录参数'],
    [{title:'wait',input_kind:'parameters',input:'{"timeout_ms":-1}'},'Recorded parameters','已记录参数'],
    [{title:'wait',input_kind:'parameters',input:'{"timeout_ms":"3000"}'},'Recorded parameters','已记录参数'],
    [{title:'wait',input_kind:'parameters',input:'{"timeout_ms":1e100}'},'Recorded parameters','已记录参数'],
    [{title:'spawn_agent',input_kind:'parameters',input:'{"message":'},'Recorded parameters','已记录参数'],
    [{title:'spawn_agent',input_kind:'parameters',input:'[]'},'Recorded parameters','已记录参数'],
    [{title:'wait',input_kind:'parameters',input:'{"timeout_ms":30000}',detail:'Wait for the reviewer'},'Wait for the reviewer','Wait for the reviewer'],
    [{title:'delegate',input_kind:'parameters',input:'{"message":"'+ 'x'.repeat(8200) +'"}'},'Recorded parameters','已记录参数'],
  ];
  for (const [step,en,zh] of cases) {
    const original = JSON.stringify(step);
    assert.equal(activity.preview(step,'en'),en);assert.equal(activity.preview(step,'zh'),zh);
    assert.equal(JSON.stringify(step),original,'preview must not rewrite the record');
  }
});

test('previews bound whitespace and spoken length without splitting a surrogate pair', () => {
  assert.equal(activity.preview({title:'exec',detail:'a\n   b'},'en'),'a b');
  const result=activity.preview({title:'read',detail:'a'.repeat(158)+'😀'+'x'.repeat(30)},'en');
  assert.equal(result,'a'.repeat(158)+'…');
});
