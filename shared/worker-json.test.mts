import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readJSONObject, JSONBodyError, MAX_JSON_BODY_BYTES} from './worker-json.mts';
const req = (body: any, headers: Record<string,string>={}) => new Request('https://fixture.invalid', {method:'POST',body,headers,duplex:'half'} as any);
const status = (code: number) => (e: unknown) => e instanceof JSONBodyError && e.status===code;

test('ordinary objects and a full byte-budget body retain their exact data',async()=>{
 const json='{"message":"hello 世界"}';
 assert.deepEqual(await readJSONObject(req(json)),{message:'hello 世界'});
 const bytes=new TextEncoder().encode(json).length;
 assert.deepEqual(await readJSONObject(req(json+' '.repeat(65536-bytes))),{message:'hello 世界'});
});
test('declared overflow is rejected before touching the body',async()=>{
 const request={headers:new Headers({'Content-Length':'65537'}),get body(){throw new Error('body must not be opened');}} as any;
 await assert.rejects(readJSONObject(request),status(413));
});
test('unknown length and a lying small length cannot bypass the stream budget',async()=>{
 for(const headers of [{},{'Content-Length':'1'}]) await assert.rejects(readJSONObject(req('{"message":"'+'a'.repeat(65536)+'"}',headers)),status(413));
 const unicode='{"message":"'+'界'.repeat(24000)+'"}';
 assert.ok(unicode.length<65536);await assert.rejects(readJSONObject(req(unicode)),status(413));
});
test('stream overflow cancels without waiting for a producer cancel promise',async()=>{
 let canceled=false;
 const stream=new ReadableStream({start(c){c.enqueue(new Uint8Array(65537));},cancel(){canceled=true;return new Promise<void>(()=>{});}});
 await assert.rejects(readJSONObject(req(stream)),status(413));assert.equal(canceled,true);
});
test('malformed JSON, invalid UTF8 and non-object roots are rejected',async()=>{
 for(const body of ['null','[]','true','1','"text"','{} {}','{broken',new Uint8Array([0xff,0xfe])]) await assert.rejects(readJSONObject(req(body)),status(400));
});
test('only an explicitly optional empty request becomes an empty object',async()=>{
 await assert.rejects(readJSONObject(req(null)),status(400));
 assert.deepEqual(await readJSONObject(req(null),true),{});
 assert.deepEqual(await readJSONObject(req(' \n '),true),{});
 await assert.rejects(readJSONObject(req('broken'),true),status(400));
});
test('invalid lengths fail without trusting the header as a read budget',async()=>{
 for(const value of ['-1','nope','1e9']) await assert.rejects(readJSONObject(req('{}',{'Content-Length':value})),status(400));
 assert.equal(MAX_JSON_BODY_BYTES,65536);
});

 test('a stricter caller budget keeps its boundary and nonblocking cancellation', async () => {
  assert.deepEqual(await readJSONObject(req('{}' + ' '.repeat(4094)), false, 4096), {});
  await assert.rejects(readJSONObject(req('{}' + ' '.repeat(4095)), false, 4096), status(413));
  let canceled = false;
  const stream = new ReadableStream({start(c) { c.enqueue(new Uint8Array(4097)); }, cancel() { canceled = true; return new Promise<void>(() => {}); }});
  await assert.rejects(readJSONObject(req(stream), false, 4096), status(413));
  assert.equal(canceled, true);
 });
