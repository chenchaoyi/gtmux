import {getDriver} from './driver';

export type TextRect = {x: number; y: number; width: number; height: number};
export type NativeTextLayout = {
  identifier: string; revision: string; scale: number; selectionActive: boolean; queued: boolean;
  fontPoints: number[]; frame: TextRect; bounds: TextRect; measuredHeight: number;
  selectionRectCount: number; selectionBounds: TextRect; lastCharacterRect: TextRect;
};

// A snapshot's handles can expire during live reflow. Discard that entire sample;
// never pair a fresh native record with dimensions from an earlier view instance.
export async function waitForNativeTextLayout(panel: string, read: () => NativeTextLayout[]) {
  const driver = getDriver();
  const deadline = Date.now() + 20000;
  let last = '', stable = 0;
  let observedTextFrames: Array<TextRect & {identifier: string}> = [];
  let nativeTextLayouts: NativeTextLayout[] = [];
  const sameSize = (a: TextRect, b: TextRect) => Math.abs(a.width - b.width) <= 2 && Math.abs(a.height - b.height) <= 2;
  await driver.waitUntil(async () => {
    if (Date.now() > deadline) return false;
    const frames: typeof observedTextFrames = [];
    try {
      const views = await driver.$$('-ios predicate string:type == "XCUIElementTypeTextView" AND value != nil AND value != ""');
      for (const view of views) {
        if (!await view.isDisplayed()) continue;
        const identifier = await view.getAttribute('name');
        if (!identifier) throw new Error('visible native text has no diagnostic identifier');
        frames.push({...await driver.getElementRect(await view.elementId), identifier});
      }
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
