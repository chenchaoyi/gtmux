import {GtmuxClient} from './client';
import {toAgent, nativeEvidenceLabel} from './types';

test('SID006 observed history stays separate from current session authority and survives 304', async () => {
 const original = globalThis.fetch;
 const fetch = jest.fn();globalThis.fetch=fetch;
 const response = (status: number, state: string) => ({status,ok:status===200,headers:{get:(key:string)=>key==='X-Gtmux-Conversation-State'?state:null},json:async()=>[]}) as unknown as Response;
 try {
  const a=toAgent({pane_id:'%1',agent:'Codex',observed_conversation:{session_id:'history',home_id:'home',state:'last-known'}});
  expect(a.session_id).toBeUndefined();expect(nativeEvidenceLabel(a,'en')).toContain('Association pending');
  const client=new GtmuxClient('https://synthetic.invalid','fixture');
  fetch.mockResolvedValueOnce(response(200,'last-known'));
  expect((await client.transcript('%1')).readOnlyHistory).toBe(true);
  expect(fetch.mock.calls[0][0]).toContain('&observation=1');
  fetch.mockResolvedValueOnce(response(304,'last-known'));
  expect(await client.transcript('%1','old')).toMatchObject({unchanged:true,readOnlyHistory:true});
  fetch.mockResolvedValueOnce(response(304,'verified'));
  expect((await client.transcript('%1','new')).readOnlyHistory).toBe(false);
 } finally {globalThis.fetch=original;}
});
