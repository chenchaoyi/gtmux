import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {useDesktopTranscript} from './useDesktopTranscript';
import type {GtmuxClient} from '../api/client';

test('CRX-002 external reload retains content, enforces parent deadline and retires old scope',async()=>{
 jest.useFakeTimers();let tree!:renderer.ReactTestRenderer;
 const read=jest.fn().mockResolvedValue({turns:[{prompt:'kept',response:'kept answer'}],dropped:0,etag:'old'});
 const client={desktopTranscript:read} as unknown as GtmuxClient;
 let value!:ReturnType<typeof useDesktopTranscript>;
 function Probe({id}:{id:string}){value=useDesktopTranscript(client,id,true,'home');return null;}
 const flush=async()=>act(async()=>{for(let i=0;i<12;i++)await Promise.resolve();});
 await act(async()=>{tree=renderer.create(<Probe id="one"/>);});
 try{
  let late!:(v:any)=>void;read.mockReturnValueOnce(new Promise(r=>{late=r;}));
  act(()=>value.retry());expect(value.retrying).toBe(true);expect(value.turns[0].response).toBe('kept answer');await flush();
  expect(read.mock.calls.at(-1)[1]).toBeUndefined();
  await act(async()=>jest.advanceTimersByTime(15000));await flush();expect(value.retrying).toBe(false);expect(value.retryFailed).toBe(true);
  read.mockResolvedValue({turns:[{prompt:'new',response:'new scope'}],dropped:0});
  await act(async()=>tree.update(<Probe id="two"/>));
  await act(async()=>late({turns:[{prompt:'PRIVATE OLD',response:'PRIVATE OLD'}],dropped:0}));
  expect(value.turns[0].response).toBe('new scope');
 }finally{act(()=>tree.unmount());jest.clearAllTimers();jest.useRealTimers();}
});
