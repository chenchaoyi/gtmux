const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const docs = require('./documents.js');
test('browser document references and validation are generated from the App contract', () => {
  const src = fs.readFileSync(path.resolve(__dirname,'../../../mobileapp/src/documents/references.ts'),'utf8');
  assert.ok(fs.readFileSync(path.join(__dirname,'documents.js'),'utf8').includes('SHA256 '+crypto.createHash('sha256').update(src).digest('hex')));
  assert.equal(docs.documentReference('/tmp/test.png').path,'/tmp/test.png');
  assert.equal(docs.documentReference('file://other-host/tmp/test.png'),null);
  assert.equal(docs.documentReference('javascript:alert(1)'),null);
  assert.equal(docs.parseDocument({id:'a'.repeat(32),sha256:'b'.repeat(64),name:'a',kind:'image',media_type:'image/png',bytes:1,created_at:1,source:'/tmp/a',session_id:'other'},'one'),null);
});
