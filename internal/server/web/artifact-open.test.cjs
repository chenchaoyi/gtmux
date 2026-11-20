const {test} = require('node:test');
const assert = require('node:assert/strict');
const {startupDocument} = require('./artifact-open.js');
const id = 'a'.repeat(32);
test('LAP-003 canonical startup keeps the original scope and pairing stays separate', () => {
 assert.deepEqual(startupDocument('#home=home&document='+id+'&session=original-session'), {reference:'gtmux-file:'+id,session_id:'original-session',home_id:'home'});
 assert.deepEqual(startupDocument('#document='+id+'&session=original-session'), {reference:'gtmux-file:'+id,session_id:'original-session',home_id:''});
});
for (const hash of ['#c=abcdef012345&document='+id+'&session=s', '', '#c=abcdef012345', '#document='+id, '#document='+id.toUpperCase()+'&session=s', '#document='+id+'&session=s&session=other', '#document='+id+'&session=s&home=h&home=x', '#document='+id+'&session=../s', '#document='+id+'&session=s%0A', '#document='+id+'&session='+('s'.repeat(257)), '#document='+id+'&session=s&home='+('h'.repeat(129)), '#document='+id+'&document='+id+'&session=s']) {
 test('LAP-003 rejects ambiguous or malformed '+hash.slice(0,75), () => assert.equal(startupDocument(hash), null));
}
