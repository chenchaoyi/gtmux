import {callTitle,callFacts} from './toolPresentation';
import {activityPreview} from './activity';
import type {TranscriptStep} from '../api/client';
const context = {module: {exports: {} as any}};
require('node:vm').runInNewContext(require('node:fs').readFileSync(require.resolve('../../../internal/server/web/activity.js'), 'utf8'), context);
const browser = context.module.exports;
const trace=require('../../../internal/server/web/trace-model.js');

test('AOP001 semantic actions override raw transport names without rewriting tool facts',()=>{
 for(const agent of ['codex','claude','dsh','opencode','kimi']) for(const [operation,en,zh] of [['read','Read file','读取文件'],['search','Search','搜索'],['list','List files','浏览目录'],['command','Run command','运行命令'],['code','Run code','运行代码'],['edit','Edit file','编辑文件']]){
  const step={kind:'tool',title:'exec',operation,input:'unchanged raw',call:{name:'exec',provider:agent,status:'completed'}} as TranscriptStep;
  const original=JSON.stringify(step);
  expect(callTitle(step,'en')).toBe(en);expect(callTitle(step,'zh')).toBe(zh);
  expect(trace.callTitle(step,'en')).toBe(en);expect(trace.callTitle(step,'zh')).toBe(zh);
  expect(callFacts(step,'en')[0]).toEqual({label:'Tool',value:'exec'});
  expect(JSON.stringify(step)).toBe(original);
 }
 expect(callTitle({title:'future_method',call:{name:'future_method',provider:'sample'}} as TranscriptStep)).toBe('sample · Future method');
});

test('AOP002 argv preview is bounded, cross-surface and leaves ambiguous input verbatim',()=>{
 const cases:[Partial<TranscriptStep>,string][]=[
  [{input:JSON.stringify(['/bin/bash','-c','python3 - <<\'PY\'\nprint(1)\nPY'])},"python3 - <<'PY' print(1) PY"],
  [{input:JSON.stringify(['/bin/zsh','-lc','rg needle src'])},'rg needle src'],
  [{input:JSON.stringify(['printf','a b','$HOME'])},'printf "a b" "$HOME"'],
  [{operation:'read',detail:'BUDGET.md, REVIEW.md',input:JSON.stringify(['/bin/bash','-c','cat BUDGET.md REVIEW.md'])},'BUDGET.md, REVIEW.md'],
  [{operation:'read',detail:'[notes].md',input:JSON.stringify(['/bin/bash','-c','cat "[notes].md"'])},'[notes].md'],
  [{input:'["bash",'},'["bash",'],
  [{input:'["bash",1]'},'["bash",1]'],
  [{input:JSON.stringify(['/bin/bash','-c','printf one']),details_truncated:true},JSON.stringify(['/bin/bash','-c','printf one'])],
 ];
 for(const [fields,expected] of cases){const step={title:'exec',operation:'command',input_kind:'arguments',...fields} as TranscriptStep;const raw=JSON.stringify(step);expect(activityPreview(step)).toBe(expected);expect(browser.preview(step,'en')).toBe(expected);expect(JSON.stringify(step)).toBe(raw);}
 const big={title:'exec',input_kind:'arguments',input:JSON.stringify(['bash','-c','x'.repeat(9000)])} as TranscriptStep;
 expect(activityPreview(big).length).toBeLessThanOrEqual(160);expect(browser.preview(big,'en')).toBe(activityPreview(big));
});
