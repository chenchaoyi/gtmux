const {logTail, automationDisconnected, captureIndependently} = require('./diagnostics.cjs');

test('emulator logs retain a bounded tail across chunks and a single oversized write',()=>{
 const tail=logTail(8);tail.push('1234');tail.push('567890');expect(tail.text()).toBe('34567890');expect(tail.bytes).toBe(8);
 tail.push('abcdefghijkl');expect(tail.text()).toBe('efghijkl');expect(tail.bytes).toBe(8);
 const unicode=logTail(32);const bytes=Buffer.from('设备🙂');unicode.push(bytes.subarray(0,4));unicode.push(bytes.subarray(4));expect(unicode.text()).toBe('设备🙂');
});
test('driver loss is distinct from an application assertion or a missing UI element',()=>{
 for(const text of ['instrumentation process is not running (probably crashed)','adb: device offline','Original error: socket hang up','invalid session id'])expect(automationDisconnected(new Error(text))).toBe(true);
 for(const text of ['Radar did not load','expected 2 pages','element was not displayed'])expect(automationDisconnected(new Error(text))).toBe(false);
});
test('a broken screenshot still collects source and late independent device diagnostics',async()=>{
 const calls:string[]=[];const original=new Error('socket hang up');
 await expect(captureIndependently([
  ()=>calls.push('log-before'),()=>{calls.push('screenshot');throw original;},
  ()=>{calls.push('source');throw new Error('source unavailable');},()=>calls.push('log-after'),
 ])).rejects.toBe(original);
 expect(calls).toEqual(['log-before','screenshot','source','log-after']);
});
