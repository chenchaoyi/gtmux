import {getDriver} from './driver';
import {screenshot} from './screenshot';

type Display = {id: number; metrics: {widthPixels: number; heightPixels: number}};

export async function posture(angle: number, panel: 'inner' | 'outer', evidence: object[], modalTarget?: string) {
    const driver = getDriver();
    const observation: {requestedAngle: number; panel: string; measured?: number; lastFrame?: object; selected?: number} = {requestedAngle: angle, panel};
    evidence.push(observation);
    await driver.execute('mobile: setSimulatedHingeAngle', {angle});
    let measured = NaN;
    // The documented command requests a transition; it does not guarantee
    // that a reading made immediately afterward has already settled.
    await driver.waitUntil(async () => {
      measured = await driver.execute('mobile: getSimulatedHingeAngle') as number;
      observation.measured = measured;
      return Math.abs(measured - angle) <= 5;
    }, {timeout: 20000, interval: 500, timeoutMsg: `hinge did not settle near ${angle}°`});
    const displays = await driver.execute('mobile: listDisplays') as Display[];
    expect(displays.length).toBeGreaterThanOrEqual(2);
    const sorted = [...displays].sort((a, b) =>
      a.metrics.widthPixels * a.metrics.heightPixels - b.metrics.widthPixels * b.metrics.heightPixels);
    const selected = panel === 'inner' ? sorted[sorted.length - 1] : sorted[0];
    observation.selected = selected.id;
    await driver.updateSettings({currentDisplayId: selected.id});
    const target = driver.$(modalTarget ?? '~window-metrics-root');
    await target.waitForExist({timeout: 15000});
    let previous = '', stable = 0;
    let size = {width: 0, height: 0};
    await driver.waitUntil(async () => {
      if (modalTarget && !await target.isDisplayed()) return false;
      const frame = await driver.getElementRect(await target.elementId);
      const bounds = modalTarget ? await driver.getWindowSize() : frame;
      observation.lastFrame = {frame, bounds};
      size = {width: bounds.width, height: bounds.height};
      const inside = frame.width > 0 && frame.height > 0 && (!modalTarget ||
        (frame.x >= 0 && frame.y >= 0 && frame.x + frame.width <= bounds.width && frame.y + frame.height <= bounds.height));
      const current = JSON.stringify({frame, bounds});
      stable = inside && current === previous ? stable + 1 : 0;
      previous = current;
      return stable >= 2;
    }, {timeout: 20000, interval: 500, timeoutMsg: 'scene layout did not settle on the selected display'});
    evidence.push({angle, measured, displays, selected: selected.id, size});
    await screenshot(`duo-fold-${panel}`);
    return size;
}

