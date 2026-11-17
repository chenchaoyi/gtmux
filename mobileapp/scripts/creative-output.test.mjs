import test from 'node:test';
import assert from 'node:assert/strict';
import {deflateSync} from 'node:zlib';
import {CREATIVE_LAYOUTS, assertSafeGeometry} from './creative-layout.mjs';
import {phoneStoryRegion, verifyCreativeOutput} from './creative-output.mjs';

const canvas = {width: 64, height: 192};
const devices = {phone: {x: 4, y: 100, width: 12, height: 60},
  tablet: {x: 28, y: 110, width: 20, height: 60}};

function chunk(type, bytes) {
  const content = Buffer.concat([Buffer.from(type), bytes]);
  let crc = 0xffffffff;
  for (const byte of content) {
    crc ^= byte;
    for (let bit = 0; bit < 8; bit++) crc = (crc >>> 1) ^ ((crc & 1) ? 0xedb88320 : 0);
  }
  const length = Buffer.alloc(4), sum = Buffer.alloc(4);
  length.writeUInt32BE(bytes.length); sum.writeUInt32BE((crc ^ 0xffffffff) >>> 0);
  return Buffer.concat([length, content, sum]);
}

function png(offset = 0, painted = devices) {
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(canvas.width); ihdr.writeUInt32BE(canvas.height, 4);
  ihdr[8] = 8; ihdr[9] = 2;
  const stride = canvas.width * 3 + 1, rows = Buffer.alloc(stride * canvas.height, 255);
  for (let y = 0; y < canvas.height; y++) rows[y * stride] = 0;
  for (const r of Object.values(painted)) {
    const x = Math.floor(r.x + r.width / 2), y = r.y + offset, at = y * stride + 1 + x * 3;
    rows.set([16, 17, 20], at);
  }
  return Buffer.concat([Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    chunk('IHDR', ihdr), chunk('IDAT', deflateSync(rows)), chunk('IEND', Buffer.alloc(0))]);
}

test('final RGB PNG device anchors match recorded canvas coordinates', () => {
  const result = verifyCreativeOutput(png(), canvas, devices);
  assert.deepEqual(result.anchors, {phone: {x: 10, expectedY: 100, actualY: 100},
    tablet: {x: 38, expectedY: 110, actualY: 110}});
});

test('rejects malformed, non-RGB, wrong-size and incomplete final output', () => {
  assert.throws(() => verifyCreativeOutput(Buffer.from('not a PNG'), canvas, devices), /Invalid final PNG/);
  assert.throws(() => verifyCreativeOutput(png().subarray(0, 40), canvas, devices), /Truncated|Incomplete/);
  const alpha = png(); alpha[25] = 6;
  assert.throws(() => verifyCreativeOutput(alpha, canvas, devices), /RGB PNG/);
  assert.throws(() => verifyCreativeOutput(png(), {...canvas, height: 191}, devices), /dimensions mismatch/);
  assert.throws(() => verifyCreativeOutput(png(0, {phone: devices.phone}), canvas, devices), /anchor mismatch: tablet/);
  assert.throws(() => verifyCreativeOutput(png(), canvas, {phone: devices.phone}), /device geometry: tablet/);
});

test('protects the full HQ header and complete decision card in both layouts', () => {
  for (const layout of Object.values(CREATIVE_LAYOUTS)) {
    const story = phoneStoryRegion(layout);
    assert.doesNotThrow(() => assertSafeGeometry(layout, {story}));
    const screenTop = layout.stage.y + layout.phoneTop + 10;
    const storyBottom = screenTop + 1920 * layout.phone / 1320;
    assert.ok(Math.abs(story.y + story.height - storyBottom) < 0.001,
      'protect the full source card through y1920, not just the top58%');
    assert.throws(() => assertSafeGeometry(layout, {story: {...story, y: story.y + 64}}), /outside Apple's/);
    assert.throws(() => phoneStoryRegion(layout, {x: 1200, y: 190, width: 500, height: 1730}), /outside the verified/);
  }
});

test('detects the original final-image shift of minus64 pixels', () => {
  assert.doesNotThrow(() => verifyCreativeOutput(png(), canvas, devices));
  assert.throws(() => verifyCreativeOutput(png(-64), canvas, devices), /expected y=100, actual y=36/);
});
