import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {fcmMessage, sendFCM} from './fcm.ts';
import relay from './index.ts';

const fixtures = JSON.parse(readFileSync(new URL('../../api/testdata/fcm-contract.json', import.meta.url), 'utf8'));
for (const row of fixtures) test(`FCM contract: ${row.name}`, () => assert.deepEqual(fcmMessage(row.intent), row.message));
test('Android never falls through to APNs, including missing FCM config', async () => {
  const original = globalThis.fetch;
  let calls = 0;
  globalThis.fetch = async () => { calls++; throw new Error('network must not run'); };
  try {
    const response = await relay.fetch(new Request('https://relay.test/push', {method:'POST',headers:{authorization:'Bearer synthetic'},body:JSON.stringify({platform:'android',token:'synthetic-token'})}), {RELAY_TOKEN:'synthetic'} as any);
    assert.equal(response.status,503); assert.equal(calls,0);
    const unsupported = await relay.fetch(new Request('https://relay.test/push', {method:'POST',headers:{authorization:'Bearer synthetic'},body:JSON.stringify({platform:'harmony',token:'synthetic-token'})}), {RELAY_TOKEN:'synthetic'} as any);
    assert.equal(unsupported.status,400); assert.equal(calls,0);
  } finally {globalThis.fetch = original;}
});
test('unsupported activities, oversized bodies and malformed credentials fail closed', async () => {
  assert.throws(() => fcmMessage({token:'t',liveActivity:true}));
  assert.throws(() => fcmMessage({token:'t',body:'x'.repeat(5000)}));
  assert.equal((await sendFCM({token:'t'}, JSON.stringify({type:'service_account',project_id:'../../other',client_email:'fake@example.test',private_key:'x'}))).status,502);
});

test('FCM signs scoped RS256 assertions, caches OAuth and normalizes revoked tokens', async () => {
  const keys = await crypto.subtle.generateKey({name:'RSASSA-PKCS1-v1_5',modulusLength:2048,publicExponent:new Uint8Array([1,0,1]),hash:'SHA-256'},true,['sign','verify']);
  const der = Buffer.from(await crypto.subtle.exportKey('pkcs8',keys.privateKey)).toString('base64');
  const credentials = JSON.stringify({type:'service_account',project_id:'synthetic-worker',client_email:'fixture@synthetic-worker.iam.gserviceaccount.com',private_key:`-----BEGIN PRIVATE KEY-----\n${der}\n-----END PRIVATE KEY-----`});
  const original = globalThis.fetch; let oauth=0, deliveries=0;
  globalThis.fetch = async (input, init) => {
    const url = String(input);
    assert.equal(init?.redirect,'error');
    if(url === 'https://oauth2.googleapis.com/token') {
      oauth++; const form = new URLSearchParams(String(init?.body));
      assert.equal(form.get('grant_type'),'urn:ietf:params:oauth:grant-type:jwt-bearer');
      const parts=form.get('assertion')!.split('.');assert.equal(parts.length,3);
      assert.equal(await crypto.subtle.verify('RSASSA-PKCS1-v1_5',keys.publicKey,Buffer.from(parts[2],'base64url'),Buffer.from(parts[0]+'.'+parts[1])),true);
      const claims=JSON.parse(Buffer.from(parts[1],'base64url').toString());
      assert.equal(claims.iss,'fixture@synthetic-worker.iam.gserviceaccount.com');assert.equal(claims.aud,url);
      assert.equal(claims.scope,'https://www.googleapis.com/auth/firebase.messaging');assert.equal(claims.exp-claims.iat,3600);
      assert.ok(Math.abs(claims.iat-Date.now()/1000)<60);
      return Response.json({access_token:'synthetic-access',token_type:'Bearer',expires_in:3600});
    }
    assert.equal(url,'https://fcm.googleapis.com/v1/projects/synthetic-worker/messages:send');
    assert.equal(new Headers(init?.headers).get('authorization'),'Bearer synthetic-access');
    assert.deepEqual(JSON.parse(String(init?.body)),fcmMessage({token:'synthetic',pane:'%12'})); deliveries++;
    return deliveries === 1 ? Response.json({name:'synthetic-message'}) : Response.json({error:{details:[{errorCode:'UNREGISTERED'}],message:'private-provider-detail'}},{status:404});
  };
  try {
    assert.equal((await sendFCM({token:'synthetic',pane:'%12'},credentials)).status,200);
    const revoked=await sendFCM({token:'synthetic',pane:'%12'},credentials);assert.equal(revoked.status,502);
    assert.deepEqual(await revoked.json(),{error:'fcm',status:404,code:'UNREGISTERED'});
    assert.equal(oauth,1);assert.equal(deliveries,2);
  } finally {globalThis.fetch=original;}
});
