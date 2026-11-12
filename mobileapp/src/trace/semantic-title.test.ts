import {build} from './model';
import {traceEventLabel,traceCounts} from './presentation';
const browser=require('../../../internal/server/web/trace-model.js');

test('AOP001 Trace preserves every real call while labels follow normalized actions',()=>{
 const steps=[{title:'functions.exec',operation:'code',input_kind:'code',input:'text(await tools.exec_command(...))',call:{name:'exec'}},{title:'exec',operation:'read',detail:'one.md, two.md',call:{name:'exec'}}];
 const turns=[{prompt:'fixture',segments:[{steps}]}];
 const events=build(turns).events;const tools=events.filter(e=>e.kind==='tool');
 expect(tools).toHaveLength(2);expect(traceCounts(events).calls).toBe(2);
 for(const lang of ['en','zh'] as const){expect(tools.map(e=>traceEventLabel(e,lang))).toEqual(lang==='en'?['Run code','Read file']:['运行代码','读取文件']);expect(tools.map(e=>browser.traceEventLabel(e,lang))).toEqual(tools.map(e=>traceEventLabel(e,lang)));}
 expect(steps).toHaveLength(2);expect(steps[1].call.name).toBe('exec');
});
