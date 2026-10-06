import {execFileSync} from 'child_process';
import {mkdirSync} from 'fs';
import {join, resolve} from 'path';
import {getDriver} from '../setup/driver';
import {captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import type {DigestRow} from '../../src/api/client';
import {TestIds} from '../../src/constants/testIds';
import {decisions} from '../../src/screens/hqZones';

/**
 * The regular shell on an iPad (change ipad-universal-app, phase 1): the radar is a
 * sidebar, and a row, the HQ card and All panes each fill the MAIN PANE instead of
 * pushing a screen — no back button anywhere. The sidebar hides and comes back.
 *
 * Needs a live serve (the demo runs outside the navigator, so it never shows this shell)
 * and a BOOTED iPad simulator:
 *
 *   GTMUX_E2E_UDID="${AUDIT_IPAD_UDID:?the owned iPad simulator UDID}" GTMUX_E2E_DEVICE='iPad Pro 13-inch (M5)' \
 *   GTMUX_E2E_URL="${AUDIT_SERVE_URL:?the isolated fixture URL}" GTMUX_E2E_TOKEN="${AUDIT_SERVE_TOKEN:?its synthetic token}" \
 *   npm run test:e2e -- split-shell
 */
const url = process.env.GTMUX_E2E_URL;
const token = process.env.GTMUX_E2E_TOKEN;
const gated = url && token && /ipad/i.test(process.env.GTMUX_E2E_DEVICE || '') ? describe : describe.skip;
const UDID = process.env.GTMUX_E2E_UDID || 'booted';
const OUT = resolve(__dirname, '../../.e2e-artifacts/ipad');

function shot(name: string): void {
  execFileSync('xcrun', ['simctl', 'io', UDID, 'screenshot', join(OUT, `${name}.png`)], {stdio: 'ignore'});
}

type Rect = {x: number; y: number; width: number; height: number};
async function rectOf(id: string): Promise<Rect> {
  return getDriver().getElementRect(await getDriver().$(`~${id}`).elementId);
}
const inside = (r: Rect, box: Rect): boolean =>
  r.width > 0 && r.x >= box.x && r.x + r.width <= box.x + box.width && r.y >= box.y && r.y + r.height <= box.y + box.height;

gated('the regular shell on an iPad', () => {
  it('shows the sidebar beside the main pane, and opens everything in place', async () => {
    mkdirSync(OUT, {recursive: true});
    const driver = getDriver();
    await driver.setOrientation('LANDSCAPE').catch(() => {});
    await launchWithFlags({GTMUX_DEBUG_PAIR_URL: url!, GTMUX_DEBUG_PAIR_TOKEN: token!, GTMUX_DEBUG_NO_PUSH: '1'});
    try {
      await driver.$(`~${TestIds.radar.split}`).waitForDisplayed({timeout: 25_000});
    } catch (err) {
      return captureOnFailure('ipad-no-split', err);
    }
    await settle(2500);
    shot('01-landscape-first-agent');
    // The first agent is open in the main pane: a detail with NO back button.
    expect(await driver.$(`~${TestIds.detail.modeChat}`).isExisting()).toBe(true);
    expect(await driver.$(`~${TestIds.detail.back}`).isExisting()).toBe(false);

    // A sidebar row switches the main pane in place.
    const rows = await driver.$$(`-ios predicate string:name BEGINSWITH '${TestIds.agent.row}-'`).getElements();
    expect(rows.length).toBeGreaterThan(0);
    if (rows.length > 1) {
      await rows[1].click();
      await settle(1500);
      expect(await driver.$(`~${TestIds.detail.back}`).isExisting()).toBe(false);
      expect(await driver.$(`~${TestIds.radar.split}`).isDisplayed()).toBe(true);
    }
    shot('02-landscape-second-agent');

    // The HQ card opens the HQ page in the main pane (its doors are on screen), no back.
    // On the regular shell the page is report header + console + an INSPECTOR carrying
    // the two zones (phase 2, D5): no console tab, the calls beside the conversation.
    const hqCard = driver.$('~radar-hq-card');
    if (await hqCard.isExisting()) {
      await hqCard.click();
      await driver.$('~hq-knowledge-open').waitForDisplayed({timeout: 15_000});
      await settle(1500);
      shot('03-landscape-hq');
      expect(await driver.$(`~${TestIds.radar.split}`).isDisplayed()).toBe(true);
      expect(await driver.$('~hq-inspector').isExisting()).toBe(true);
      expect(await driver.$('~hq-tab-console').isExisting()).toBe(false);
      // The calls are not behind a tab here (there is no tab bar on this shell): the
      // inspector carries them. The expectation is the fixture's own waiting worker
      // (fake-serve world: pane %11, session "MP analysis"), found in the digest directly,
      // so a fixture or a decisions() that loses it fails here instead of passing on an
      // empty list. Positions, not isDisplayed: XCUITest reports some touchable views as
      // not visible. The digest is fetched directly: the API client pulls in react-native.
      const digest = (await (await fetch(`${url}/api/digest`, {headers: {Authorization: `Bearer ${token}`}})).json()) as DigestRow[];
      const waiting = digest.find(r => r.pane_id === '%11');
      expect(waiting?.status).toBe('waiting');
      const loc = waiting!.loc!;
      const box = await rectOf('hq-inspector');
      expect(inside(await rectOf(`hq-call-${loc}`), box)).toBe(true);
      expect(await driver.$(`~hq-call-${loc}`).getAttribute('label')).toContain('MP analysis');
      expect(inside(await rectOf(`hq-call-open-${loc}`), box)).toBe(true);
      expect(inside(await rectOf(`hq-call-ask-${loc}`), box)).toBe(true);
      // Second layer: the page's own rule counts that worker among its decisions, and
      // every decision it counts has a card.
      const calls = decisions(digest);
      expect(calls.map(c => c.loc)).toContain(loc);
      for (const c of calls) expect(await driver.$(`~hq-call-${c.loc}`).isExisting()).toBe(true);
      expect(await driver.$(`~${TestIds.composer.keyboard}`).isDisplayed()).toBe(true);

      // The knowledge sheet: list on the left, the entry on the right, no back button.
      await driver.$('~hq-knowledge-open').click();
      await driver.$('~knowledge-find').waitForDisplayed({timeout: 10_000});
      await settle(1200);
      const entry = driver.$("-ios predicate string:identifier BEGINSWITH 'knowledge-entry-'");
      if (await entry.isExisting()) {
        await entry.click();
        await settle(1200);
        shot('03b-landscape-knowledge');
        expect(await driver.$('~knowledge-back').isExisting()).toBe(false);
        expect(await driver.$('~knowledge-find').isDisplayed()).toBe(true); // the list is still there
      }
      await driver.$('~knowledge-close').click();
      await settle(800);
    }

    // All panes, likewise: the browser without its back button.
    await driver.$(`~${TestIds.radar.panes}`).click();
    await driver.$(`~${TestIds.panes.search}`).waitForDisplayed({timeout: 10_000});
    await settle(1200);
    expect(await driver.$(`~${TestIds.panes.back}`).isExisting()).toBe(false);
    shot('04-landscape-panes');

    // Hide the sidebar: the floating button appears; show it again.
    await driver.$(`~${TestIds.radar.hideSidebar}`).click();
    await settle(600);
    expect(await driver.$(`~${TestIds.radar.showSidebar}`).isDisplayed()).toBe(true);
    shot('05-landscape-sidebar-hidden');
    await driver.$(`~${TestIds.radar.showSidebar}`).click();
    await settle(600);
    expect(await driver.$(`~${TestIds.radar.hideSidebar}`).isDisplayed()).toBe(true);

    // Portrait keeps the two columns (D2: no drawer), narrower sidebar.
    await driver.setOrientation('PORTRAIT').catch(() => {});
    await settle(1500);
    expect(await driver.$(`~${TestIds.radar.split}`).isDisplayed()).toBe(true);
    shot('06-portrait');
    await driver.setOrientation('LANDSCAPE').catch(() => {});
  });
});
