import {GtmuxClient} from './client';

test('SID009 ordinary reads stay compatible while UI history explicitly opts in',async()=>{
 const original=globalThis.fetch,fetch=jest.fn();globalThis.fetch=fetch;
 const client=new GtmuxClient('https://synthetic.invalid','fixture');
 const response=(body:unknown=[])=>({ok:true,status:200,headers:{get:()=>null},json:async()=>body}) as unknown as Response;
 try{
  fetch.mockResolvedValueOnce(response());await client.transcript('%1');
  expect(fetch.mock.calls.at(-1)[0]).toBe('https://synthetic.invalid/api/transcript?id=%251&context=1');
  fetch.mockResolvedValueOnce(response());await client.transcript('%1',undefined,0,undefined,true);
  expect(fetch.mock.calls.at(-1)[0]).toBe('https://synthetic.invalid/api/transcript?id=%251&recent=1&context=1');
  fetch.mockResolvedValueOnce(response());await client.transcript('%1',undefined,0,undefined,true,true);
  expect(fetch.mock.calls.at(-1)[0]).toBe('https://synthetic.invalid/api/transcript?id=%251&recent=1&context=1&observation=1');
  fetch.mockResolvedValueOnce(response({invalid:'shape'}));
  expect(await client.transcript('%1')).toEqual({turns:[],dropped:0});
  const page=()=>({ok:true,status:200,headers:{get:(key:string)=>key==='X-Gtmux-History-Page'?'1':null},json:async()=>[{record_id:'fixture'}]}) as unknown as Response;
  fetch.mockResolvedValueOnce(page());await client.traceHistory({pane_id:'%1'},'cursor');
  expect(fetch.mock.calls.at(-1)[0]).toBe('https://synthetic.invalid/api/transcript?id=%251&context=1&before=cursor');
  fetch.mockResolvedValueOnce(page());await client.traceHistory({pane_id:'%1',observed_conversation:{session_id:'history',home_id:'h',state:'last-known'}},'cursor');
  expect(fetch.mock.calls.at(-1)[0]).toBe('https://synthetic.invalid/api/transcript?id=%251&context=1&before=cursor&observation=1');
  fetch.mockResolvedValueOnce(page());await client.traceHistory({pane_id:'native:codex:s',session_id:'s',home_id:'h',observed_conversation:{session_id:'history',home_id:'h',state:'last-known'}},'cursor');
  expect(fetch.mock.calls.at(-1)[0]).toBe('https://synthetic.invalid/api/session/transcript?session_id=s&home_id=h&context=1&before=cursor');
 }finally{globalThis.fetch=original;}
});
