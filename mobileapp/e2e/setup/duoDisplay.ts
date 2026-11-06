import {getDriver} from './driver';

/** Select the unfolded inner panel before screenshots or pointer actions. */
export async function selectUnfoldedInnerDisplay(): Promise<void> {
  const driver = getDriver();
  await driver.execute('mobile: setSimulatedHingeAngle', {angle: 180});
  await driver.waitUntil(async () => {
    const angle = await driver.execute('mobile: getSimulatedHingeAngle') as number;
    return Math.abs(angle - 180) <= 5;
  }, {timeout: 20000, interval: 500, timeoutMsg: 'Duo did not unfold before selecting the inner display'});
  const displays = await driver.execute('mobile: listDisplays') as Array<{id: number; metrics: {widthPixels: number; heightPixels: number}}>;
  if (displays.length < 2) throw new Error('Duo did not expose both displays');
  const inner = displays.reduce((a, b) => a.metrics.widthPixels * a.metrics.heightPixels > b.metrics.widthPixels * b.metrics.heightPixels ? a : b);
  await driver.updateSettings({currentDisplayId: inner.id});
}
