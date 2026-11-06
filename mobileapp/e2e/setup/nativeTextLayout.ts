import {getDriver} from './driver';

export type TextRect = {x: number; y: number; width: number; height: number};
export type NativeTextLayout = {
  identifier: string; revision: string; scale: number; selectionActive: boolean; queued: boolean;
  fontPoints: number[]; frame: TextRect; bounds: TextRect; measuredHeight: number;
  selectionRectCount: number; selectionBounds: TextRect; lastCharacterRect: TextRect;
};

// One WDA snapshot supplies names, visibility and frames together. Per-element
// queries can consume the whole budget before three complete samples are read.
function snapshotTextFrames(source: unknown): Array<TextRect & {identifier: string}> {
  const frames: Array<TextRect & {identifier: string}> = [];
  const identifiers = new Set<string>();
  const visit = (value: unknown) => {
    if (!value || typeof value !== 'object') throw new Error('invalid native source node');
    const node = value as Record<string, unknown>;
    if (node.type === 'TextView' && typeof node.value === 'string' && node.value !== '') {
      if (![true, false, 1, 0, '1', '0'].includes(node.isVisible as string)) throw new Error('native text visibility is missing');
      if ([true, 1, '1'].includes(node.isVisible as string)) {
        const identifier = node.name;
        if (typeof identifier !== 'string' || !identifier) throw new Error('visible native text has no diagnostic identifier');
        if (identifiers.has(identifier)) throw new Error('duplicate native text diagnostic identifier');
        const rect = node.rect as TextRect | undefined;
        if (!rect || ![rect.x, rect.y, rect.width, rect.height].every(Number.isFinite) || rect.width <= 0 || rect.height <= 0) {
          throw new Error('visible native text has invalid geometry');
        }
        identifiers.add(identifier);
        frames.push({identifier, x: rect.x, y: rect.y, width: rect.width, height: rect.height});
      }
    }
    if (node.children !== undefined) {
      if (!Array.isArray(node.children)) throw new Error('invalid native source children');
      node.children.forEach(visit);
    }
  };
  visit(typeof source === 'string' ? JSON.parse(source) : source);
  return frames;
}

// Discard a transiently stale whole observation; never combine old and new sets.
export async function waitForNativeTextLayout(panel: string, read: () => NativeTextLayout[]) {
  const driver = getDriver();
  const deadline = Date.now() + 20000;
  let last = '', stable = 0;
  let observedTextFrames: Array<TextRect & {identifier: string}> = [];
  let nativeTextLayouts: NativeTextLayout[] = [];
  const sameSize = (a: TextRect, b: TextRect) => Math.abs(a.width - b.width) <= 2 && Math.abs(a.height - b.height) <= 2;
  await driver.waitUntil(async () => {
    if (Date.now() > deadline) return false;
    let frames: typeof observedTextFrames;
    try {
      frames = snapshotTextFrames(await driver.execute('mobile: source', {format: 'json'}));
    } catch (error) {
      if (!/stale element reference/i.test(String(error))) throw error;
      last = ''; stable = 0;
      return false;
    }
    const records = read();
    const matched = frames.map(frame => records.find(record => record.identifier === frame.identifier && sameSize(record.bounds, frame) && record.scale > 1))
      .filter((record): record is NativeTextLayout => !!record);
    if (frames.length < 3 || matched.length !== frames.length) { last = ''; stable = 0; return false; }
    const current = JSON.stringify({frames, matched});
    stable = current === last ? stable + 1 : 0;
    last = current;
    const settled = stable >= 2 && matched.every(record => !record.selectionActive && !record.queued &&
      !!record.revision && record.fontPoints.length > 0 && record.fontPoints.every(point => point > 0) &&
      record.selectionRectCount > 0 && record.measuredHeight <= record.bounds.height + 1 &&
      record.lastCharacterRect.height > 0 && record.lastCharacterRect.y >= record.bounds.y - 1 &&
      record.lastCharacterRect.y + record.lastCharacterRect.height <= record.bounds.y + record.bounds.height + 1 &&
      record.selectionBounds.y >= record.bounds.y - 1 &&
      record.selectionBounds.y + record.selectionBounds.height <= record.bounds.y + record.bounds.height + 1);
    if (!settled || Date.now() > deadline) return false;
    observedTextFrames = frames; nativeTextLayouts = matched;
    return true;
  }, {timeout: 20000, interval: 500, timeoutMsg: `${panel} native text did not settle into an unclipped current-scale layout`});
  return {panel, observedTextFrames, nativeTextLayouts};
}
