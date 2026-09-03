import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, readDebugLog, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

/**
 * Reported 2026-09-03: "都折叠起来后，没法下拉刷新了" — collapse every section on the
 * radar and the pull-to-refresh gesture stops working.
 *
 * The check is the HANDLER, not the spinner. A screenshot of a refresh spinner is a
 * timing lottery (it shows for ~600ms), while `onRefresh` records a `radar-refresh`
 * line in the debug log the moment the list accepts the pull. So: collapse the
 * sections, drag down, and ask whether the pull ever reached the list.
 *
 * Gated the same way as radar.test.ts — needs a live serve with at least one section.
 */
const url = process.env.GTMUX_E2E_URL;
const token = process.env.GTMUX_E2E_TOKEN;
const gated = url && token ? describe : describe.skip;

async function pullDown(fromFrac: number): Promise<void> {
  const driver = getDriver();
  const {width, height} = await driver.getWindowSize();
  // Slowly: a fast flick is read as a fling, not a pull.
  await driver.execute('mobile: dragFromToForDuration', {
    fromX: width / 2,
    fromY: height * fromFrac,
    toX: width / 2,
    toY: Math.min(height * (fromFrac + 0.45), height * 0.95),
    duration: 1.2,
  });
}

/** Where the list's content ends, as a fraction of the screen. */
async function contentBottomFrac(): Promise<number> {
  const driver = getDriver();
  const {height} = await driver.getWindowSize();
  let bottom = 0;
  for (const status of SECTIONS) {
    const bar = driver.$(`~${TestIds.radar.section}-${status}`);
    if (!(await bar.isExisting())) continue;
    const loc = await bar.getLocation();
    const size = await bar.getSize();
    bottom = Math.max(bottom, loc.y + size.height);
  }
  return bottom / height;
}

const SECTIONS = ['waiting', 'working', 'idle', 'running', 'errored'];

gated('radar pull-to-refresh with every section collapsed', () => {
  it('still refreshes when the list is shorter than the screen', async () => {
    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: url!,
      GTMUX_DEBUG_PAIR_TOKEN: token!,
      GTMUX_DEBUG_NO_PUSH: '1',
      GTMUX_DEBUG_LOG_NET: '1',
    });

    const radar = driver.$(`~${TestIds.radar.screen}`);
    try {
      await radar.waitForDisplayed({timeout: 25_000});
    } catch (err) {
      return captureOnFailure('refresh-no-radar', err);
    }
    await settle(2500); // let the first poll land so the sections exist

    // 1. A pull on the FULL list is the control: it must refresh, or the gesture
    //    itself is wrong and the collapsed case proves nothing.
    const before = readDebugLog().filter(e => e.event === 'radar-refresh').length;
    await pullDown(0.3);
    await settle(1500);
    const expanded = readDebugLog().filter(e => e.event === 'radar-refresh').length - before;
    await screenshot('refresh-1-expanded');

    // 2. Collapse every section that is on screen.
    let collapsedAny = 0;
    for (const status of SECTIONS) {
      const bar = driver.$(`~${TestIds.radar.section}-${status}`);
      if (await bar.isExisting()) {
        await bar.click();
        collapsedAny++;
        await settle(400);
      }
    }
    await screenshot('refresh-2-collapsed');

    // 3. Pull again — but from BELOW where the content now ends, which is where a
    //    thumb lands once the sections are collapsed and the lower half is blank.
    //    That area only scrolls if the list itself fills the screen.
    const bottom = await contentBottomFrac();
    const mid = readDebugLog().filter(e => e.event === 'radar-refresh').length;
    await pullDown(Math.min(bottom + 0.08, 0.72));
    await settle(1500);
    const collapsed = readDebugLog().filter(e => e.event === 'radar-refresh').length - mid;

    // eslint-disable-next-line no-console
    console.log(
      `[refresh] collapsed=${collapsedAny} content-bottom=${bottom.toFixed(2)} ` +
        `expanded-pull=${expanded} collapsed-pull=${collapsed}`,
    );
    expect(collapsedAny).toBeGreaterThan(0);
    expect(expanded).toBeGreaterThan(0); // control
    expect(collapsed).toBeGreaterThan(0); // the reported bug
  });
});
