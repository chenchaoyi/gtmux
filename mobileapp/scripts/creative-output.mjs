import {inflateSync} from 'node:zlib';

// Verified in the existing EN and ZH 03-hq.png captures (1320 x 2868):
// the HQ header through the whole needs-input card, including its time label.
export const PHONE_STORY_BOUNDS = {x: 40, y: 190, width: 1240, height: 1730};

export function phoneStoryRegion(layout, bounds = PHONE_STORY_BOUNDS) {
  if (![bounds.x, bounds.y, bounds.width, bounds.height].every(Number.isFinite) ||
      bounds.x < 0 || bounds.y < 0 || bounds.width <= 0 || bounds.height <= 0 ||
      bounds.x + bounds.width > 1320 || bounds.y + bounds.height > 2868) {
    throw new Error('Critical story bounds fall outside the verified phone capture');
  }
  const scale = layout.phone / 1320;
  return {x: layout.stage.x + 10 + bounds.x * scale,
    y: layout.stage.y + layout.phoneTop + 10 + bounds.y * scale,
    width: bounds.width * scale, height: bounds.height * scale};
}

// Decode only the bounded, non-interlaced RGB format emitted by our renderer.
// Pixel anchors verify the final file, independently of the earlier DOM dump.
function rgbPixels(png) {
  const signature = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);
  if (png.length < 33 || !png.subarray(0, 8).equals(signature) ||
      png.toString('ascii', 12, 16) !== 'IHDR') throw new Error('Invalid final PNG');
  const width = png.readUInt32BE(16), height = png.readUInt32BE(20);
  if (!width || !height || width > 4096 || height > 4096 ||
      png[24] !== 8 || png[25] !== 2 || png[26] || png[27] || png[28]) {
    throw new Error('Final artwork must be a bounded 8-bit non-interlaced RGB PNG');
  }
  const chunks = [];
  let ended = false;
  for (let at = 8; at + 12 <= png.length;) {
    const length = png.readUInt32BE(at), end = at + 12 + length;
    if (end > png.length) throw new Error('Truncated final PNG');
    const type = png.toString('ascii', at + 4, at + 8);
    if (type === 'IDAT') chunks.push(png.subarray(at + 8, at + 8 + length));
    if (type === 'IEND') { ended = true; break; }
    at = end;
  }
  if (!ended || !chunks.length) throw new Error('Incomplete final PNG');
  const stride = width * 3, expected = (stride + 1) * height;
  const bytes = inflateSync(Buffer.concat(chunks), {maxOutputLength: expected});
  if (bytes.length !== expected) throw new Error('Invalid final PNG pixel length');
  const pixels = Buffer.alloc(stride * height);
  for (let y = 0; y < height; y++) {
    const filter = bytes[y * (stride + 1)];
    if (filter > 4) throw new Error('Unsupported final PNG filter');
    for (let x = 0; x < stride; x++) {
      const at = y * stride + x;
      const a = x >= 3 ? pixels[at - 3] : 0;
      const b = y ? pixels[at - stride] : 0;
      const c = y && x >= 3 ? pixels[at - stride - 3] : 0;
      const p = a + b - c;
      const pa = Math.abs(p - a), pb = Math.abs(p - b), pc = Math.abs(p - c);
      const predictor = filter === 0 ? 0 : filter === 1 ? a : filter === 2 ? b :
        filter === 3 ? Math.floor((a + b) / 2) : pa <= pb && pa <= pc ? a : pb <= pc ? b : c;
      pixels[at] = (bytes[y * (stride + 1) + x + 1] + predictor) & 255;
    }
  }
  return {width, height, pixels};
}

export function verifyCreativeOutput(png, layout, auxiliary) {
  const {width, height, pixels} = rgbPixels(png);
  if (width !== layout.width || height !== layout.height) throw new Error('Final PNG canvas dimensions mismatch');
  const anchors = {};
  for (const name of ['phone', 'tablet']) {
    const r = auxiliary[name];
    if (!r || ![r.x, r.y, r.width, r.height].every(Number.isFinite) ||
        r.x < 0 || r.y < 0 || r.width <= 0 || r.height <= 0 ||
        r.x + r.width > width + 1 || r.y + r.height > height + 1) {
      throw new Error(`Invalid final device geometry: ${name}`);
    }
    const x = Math.floor(r.x + r.width / 2), expectedY = Math.round(r.y);
    let actualY = -1;
    for (let y = 0; y < height; y++) {
      const at = (y * width + x) * 3;
      if (pixels[at] === 16 && pixels[at + 1] === 17 && pixels[at + 2] === 20) { actualY = y; break; }
    }
    if (actualY !== expectedY) throw new Error(`Final PNG anchor mismatch: ${name} expected y=${expectedY}, actual y=${actualY}`);
    anchors[name] = {x, expectedY, actualY};
  }
  return {width, height, coordinateOrigin: 'untransformed final PNG', anchors};
}
