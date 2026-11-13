const {test} = require('node:test');
const assert = require('node:assert/strict');
const activity = require('./activity.js');
const trace = require('./trace-model.js');

test('ASA004 browser Chat and Trace preserve factual action labels and one call per record', () => {
  const cases = [
    [{kind:'tool', title:'Read', operation:'read', action:'skill', detail:'design-kpis'},'Read skill','读取技能'],
    [{kind:'tool', title:'web.run', operation:'web', action:'web_search', detail:'official metrics'},'Search web','搜索网页'],
    [{kind:'tool', title:'web.run', operation:'web', action:'web_find', detail:'outcome'},'Find in page','页内查找'],
    [{kind:'tool', title:'web.run', operation:'web', action:'web_open', detail:'https://example.org'},'Open webpage','打开网页'],
    [{kind:'tool', title:'exec', operation:'read', subject_count:3, detail:'a.md, b.md, c.md', input:'cat a.md b.md c.md', input_ref:'same-ref'},'Read 3 files','读取 3 个文件'],
  ];
  for (const [step,en,zh] of cases) {
    const before=JSON.stringify(step);
    assert.equal(trace.callTitle(step,'en'),en);assert.equal(trace.callTitle(step,'zh'),zh);
    const index=trace.build([{agent:'claude',prompt:'q',segments:[{steps:[step]}]}]);
    const event=index.events.find(e=>e.kind==='tool');
    assert.equal(trace.traceEventLabel(event,'en'),en);assert.equal(trace.traceEventLabel(event,'zh'),zh);
    assert.equal(trace.traceCounts(index.events).calls,1);
    assert.equal(JSON.stringify(step),before);
  }
  assert.equal(activity.summary([cases[1][0]],'en'),'Search web');
});
test('ASA005 browser fallback ignores contradictory facts without guessing scripts or changing raw input', () => {
  const step={kind:'tool',title:'exec',operation:'command',action:'web_find',input:'find outcome on the web'};
  assert.equal(trace.callTitle(step,'en'),'Run command');
  assert.equal(trace.callTitle({kind:'tool',title:'Read',subject_count:2.5},'en'),'Read file');
  assert.equal(trace.callTitle({kind:'tool',title:'WebSearch'},'en'),'Search web');
  assert.equal(activity.preview({...step,detail:'a'.repeat(10000)}).length,160);
});
