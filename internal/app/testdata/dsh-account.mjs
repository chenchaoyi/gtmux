import assert from 'node:assert/strict';
import {pathToFileURL} from 'node:url';
const {observeAccountUsage} = await import(pathToFileURL(process.argv[2]).href);
let tick, expire;
const setTimer = globalThis.setTimeout, clearTimer = globalThis.clearTimeout;
globalThis.setInterval = fn => {tick=fn;return {unref(){}};};
globalThis.clearInterval = () => {};
globalThis.setTimeout = (fn,ms) => ms===8000 ? (expire=fn,{unref(){}}) : setTimer(fn,ms);
globalThis.clearTimeout = timer => {if (typeof timer==='number' || timer?.hasRef) clearTimer(timer);};
const calls=[], pending=[];
let current='account-a', dispose, change, holdIdentity=false, finishIdentity;
const account={
 getDeviceIdentity:async()=>{const captured=current;if(holdIdentity){holdIdentity=false;return new Promise(resolve=>{finishIdentity=()=>resolve({userId:captured});});}return {userId:captured};},
 getBalance:metadata=>{assert.equal(typeof metadata.version,'string');return new Promise(resolve=>pending.push(resolve));},
 getPlatformSession:()=>{throw new Error('credentials must not be read');},
 watch:()=>({[Symbol.asyncIterator](){return this;},next(){return new Promise(resolve=>{change=state=>resolve({value:state,done:false});});}}),
};
observeAccountUsage({inject:(names,fn)=>{assert.deepEqual(names,['deepseekAccount']);fn({deepseekAccount:account,on:(event,fn)=>{assert.equal(event,'dispose');dispose=fn;}});}},(token,payload)=>{assert.equal(token,'Usage');calls.push(payload);});
const flush=()=>new Promise(resolve=>setImmediate(resolve));
await flush();assert.equal(pending.length,1);
current='account-b';change({status:'credential-stored'});await flush();
pending.shift()({status:'ready',value:[{currency:'CNY',balance:'99'}],bonusWallets:[]});await flush();
assert.ok(!calls.some(c=>c.status==='ready'),'old account response leaked');
assert.equal(pending.length,1);
pending.shift()({status:'ready',value:[{currency:'CNY',balance:'12.3400'}],bonusWallets:[{currency:'USD',balance:'0'}]});await flush();
const ready=calls.at(-1);assert.equal(ready.status,'ready');assert.equal(ready.account_id.length,64);
assert.equal(ready.wallets[0].amount,'12.3400');assert.ok(!JSON.stringify(calls).includes('account-b'));
tick();await flush();assert.equal(pending.length,1);
holdIdentity=true;
pending.shift()({status:'ready',value:[{currency:'CNY',balance:'88'}],bonusWallets:[]});await flush();
current='account-c';change({status:'credential-stored'});await flush();finishIdentity();await flush();
assert.ok(!calls.some(c=>c.wallets?.some(w=>w.amount==='88')),'identity changed during post-response validation');
assert.equal(pending.length,1);pending.shift()({status:'ready',value:[{currency:'CNY',balance:'7'}],bonusWallets:[]});await flush();
assert.equal(calls.at(-1).wallets[0].amount,'7');assert.notEqual(calls.at(-1).account_id,ready.account_id);
tick();await flush();assert.equal(pending.length,1);expire();tick();tick();await flush();
assert.equal(pending.length,1,'timed-out native request started parallel retries');
current=undefined;change({status:'signed-out'});await flush();
pending.shift()({status:'ready',value:[{currency:'CNY',balance:'100'}],bonusWallets:[]});await flush();
assert.equal(calls.at(-1).status,'signed-out');
const count=calls.length;dispose();tick();await flush();assert.equal(calls.length,count);
