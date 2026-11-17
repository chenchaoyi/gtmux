const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

// Execute production annotation handlers. Only image/canvas/DOM boundaries are fake.
const source = fs.readFileSync(path.join(__dirname, 'app.js'), 'utf8');
function extract(start, end) {
  const begin = source.indexOf(start), finish = source.indexOf(end, begin);
  assert.ok(begin >= 0 && finish > begin);
  return source.slice(begin, finish);
}
function editor(mode = 'success') {
  let image, pending, changes = 0, closed = false;
  const revoked = [], owned = new Set(['blob:original']);
  const node = tag => ({tag, children: [], classList: {add() {}}, attrs: {},
    appendChild(child) {this.children.push(child);}, setAttribute(k, v) {this.attrs[k] = v;}});
  const sheet = {el: node('dialog'), body: node('body'), foot: node('foot'), close() {closed = true;}};
  const canvas = Object.assign(node('canvas'), {getBoundingClientRect: () => ({left: 0, top: 0, width: 64, height: 48}),
    setPointerCapture() {}, getContext: () => ({fillRect() {}, drawImage() {}, beginPath() {}, moveTo() {}, lineTo() {}, stroke() {}}),
    toBlob(callback) {
      if (mode === 'throw') throw new Error('synthetic export refusal');
      if (mode === 'deferred') {pending = callback; return;}
      callback(mode === 'null' ? null : {size: mode === 'oversize' ? 31 * 1024 * 1024 : 123});
    }});
  const context = {revoked: false, urls: owned};
  const original = {size: 80}, other = {name: 'other.png', phase: 'queued', file: {size: 90}};
  const attachment = {file: original, name: 'first.png', url: 'blob:original', path: '/saved/first.png', phase: 'complete', progress: 1};
  const scope = {document: {createElement: tag => tag === 'canvas' ? canvas : node(tag)}, T: en => en,
    uiIcon: () => '', browserDialog: () => sheet, Image: class {constructor() {image = this; this.naturalWidth = 64; this.naturalHeight = 48;}},
    URL: {revokeObjectURL: url => revoked.push(url), createObjectURL: () => 'blob:replacement'},
    File: class {constructor(parts, name, options) {Object.assign(this, {parts, name, ...options});}}};
  vm.runInNewContext(extract('  function annotateImage(', '\n  var composeCache') +
    extract('  function attachmentFeedback(', '\n  function attachmentStatusEl(') + '\nglobalThis.start = annotateImage;globalThis.feedback = attachmentFeedback;', scope);
  scope.start(attachment, context, () => {changes++;}); image.onload();
  const done = sheet.foot.children.find(n => n.textContent === 'Done');
  function draw() {
    const event = (x, y) => ({clientX: x, clientY: y, pointerId: 1, preventDefault() {}});
    canvas.onpointerdown(event(4, 5)); canvas.onpointermove(event(30, 20)); canvas.onpointerup();
  }
  return {attachment, original, other, context, sheet, done, draw, revoked, owned, scope,
    changed: () => changes, closed: () => closed, finish: () => pending({size: 123})};
}

test('DFR-001 saving marks retires the old uploaded receipt before rendering new bytes', () => {
  const e = editor(), otherBefore = JSON.stringify(e.other);
  assert.equal(e.scope.feedback(e.attachment).text, 'Uploaded');
  e.draw(); e.done.onclick();
  assert.notEqual(e.attachment.file, e.original);
  assert.equal(e.attachment.file.name, 'first-marked.jpg');
  assert.equal(e.attachment.path, undefined); assert.equal(e.attachment.progress, null);
  assert.equal(e.scope.feedback(e.attachment), null);
  assert.deepEqual(e.revoked, ['blob:original']); assert.deepEqual([...e.owned], ['blob:replacement']);
  assert.equal(JSON.stringify(e.other), otherBefore); assert.equal(e.changed(), 1); assert.equal(e.closed(), true);
});
for (const action of ['no marks', 'use original']) test('DFR-001 ' + action + ' preserves the uploaded original', () => {
  const e = editor();
  if (action === 'no marks') e.done.onclick();
  else {e.draw(); e.sheet.foot.children.find(n => n.textContent === 'Use original').onclick();}
  assert.equal(e.attachment.file, e.original); assert.equal(e.attachment.path, '/saved/first.png');
  assert.equal(e.scope.feedback(e.attachment).text, 'Uploaded'); assert.equal(e.changed(), 0);
  assert.equal(e.closed(), true); assert.deepEqual(e.revoked, []);
});
for (const mode of ['null', 'oversize', 'throw']) test('DFR-001 rejected ' + mode + ' export retains original and allows recovery', () => {
  const e = editor(mode); e.draw(); e.done.onclick();
  assert.equal(e.attachment.file, e.original); assert.equal(e.attachment.path, '/saved/first.png');
  assert.equal(e.scope.feedback(e.attachment).text, 'Uploaded'); assert.equal(e.changed(), 0);
  assert.equal(e.closed(), false); assert.equal(e.done.disabled, false); assert.deepEqual(e.revoked, []);
});
test('DFR-001 revoked annotation export cannot mutate a retired draft', () => {
  const e = editor('deferred'); e.draw(); e.done.onclick(); e.context.revoked = true; e.finish();
  assert.equal(e.attachment.file, e.original); assert.equal(e.attachment.path, '/saved/first.png');
  assert.equal(e.changed(), 0); assert.deepEqual(e.revoked, []);
});
