import {execFileSync} from 'child_process';
import {getDriver} from '../setup/driver';
import {BUNDLE, launchWithFlags, settle, writeDebugFlags} from '../setup/app';
import {screenshot} from '../setup/screenshot';
import {startFake, Fake} from '../fake-serve/server';

// Picking another Mac in the server list must stay on it. 2026-10-05: the app had been
// launched by tapping a notification from one Mac, and every later switch bounced back to
// that Mac and opened the notified pane, because the launch notification was acted on
// again each time the push bridge remounted with the new Mac.
//
// Needs the push permission (GTMUX_E2E_ACCEPT_ALERTS=1) and GTMUX_E2E_UDID for
// `simctl push`; skipped otherwise.
const udid = process.env.GTMUX_E2E_UDID;
const run = udid && process.env.GTMUX_E2E_ACCEPT_ALERTS === '1' ? it : it.skip;

let alpha: Fake;
let beta: Fake;
beforeEach(async () => {
  alpha = await startFake({token: 'alpha-token'});
  beta = await startFake({token: 'beta-token'});
  // Each Mac's fleet says whose it is, so the radar on screen names the Mac it came from.
  alpha.world.agents.forEach(a => { a.session = `alpha ${a.session}`; });
  beta.world.agents.forEach(a => { a.session = `beta ${a.session}`; });
});
afterEach(async () => { await alpha?.close(); await beta?.close(); });

function macs() {
  return JSON.stringify([
    {url: alpha.url, token: alpha.token, name: 'Mac Alpha'},
    {url: beta.url, token: beta.token, name: 'Mac Beta'},
  ]);
}

// A notification from Mac Alpha about one of its panes, as the relay sends it.
function pushFromAlpha(pane: string): void {
  const payload = JSON.stringify({aps: {alert: {title: 'Mac Alpha', body: 'Claude Code is waiting'}}, pane, server: 'Mac Alpha'});
  execFileSync('xcrun', ['simctl', 'push', udid!, BUNDLE, '-'], {input: payload});
}

async function openServers(): Promise<void> {
  const driver = getDriver();
  // From a pane, back to the radar first: the server chip lives in the radar's header.
  const back = driver.$('~detail-back');
  if (await back.isExisting()) {
    await back.click();
    await settle(800);
  }
  const chip = driver.$('~radar-server-chip');
  await chip.waitForDisplayed({timeout: 20000});
  await chip.click();
  await settle(800);
}

async function source(): Promise<string> { return getDriver().getPageSource(); }

run('a launch from a notification does not pull a later server switch back', async () => {
  const driver = getDriver();
  const pane = alpha.world.agents.find(a => a.role !== 'supervisor')!.pane_id;
  // First launch: grant notifications (the alert is accepted) and land on the server list.
  await launchWithFlags({GTMUX_DEBUG_SERVERS: macs(), GTMUX_DEBUG_LANG: 'en'});
  await settle(2500);
  // Cold start from the notification: the app is not running when it is tapped.
  await driver.terminateApp(BUNDLE);
  writeDebugFlags({GTMUX_DEBUG_SERVERS: macs(), GTMUX_DEBUG_LANG: 'en'});
  pushFromAlpha(pane);
  await settle(1500);
  await driver.execute('mobile: activateApp', {bundleId: 'com.apple.springboard'});
  const banner = driver.$('-ios predicate string:label CONTAINS "Claude Code is waiting"');
  await banner.waitForDisplayed({timeout: 15000});
  await banner.click();
  await settle(4000);
  expect(await source()).toContain('alpha'); // the notified Mac's pane opened
  await screenshot('server-switch-launched-from-notification');

  // Switch to Mac Beta: the app must stay there.
  await openServers();
  await driver.$('-ios predicate string:label BEGINSWITH "Mac Beta"').click();
  await settle(6000);
  let now = await source();
  expect(now).toContain('beta');
  expect(now).not.toContain('alpha gtmux dev');
  await screenshot('server-switch-stays-on-beta');

  // Background and back: still Mac Beta, and nothing re-opened Alpha's pane.
  await driver.execute('mobile: backgroundApp', {seconds: 3});
  await settle(3000);
  now = await source();
  expect(now).toContain('beta');
  expect(now).not.toContain('alpha gtmux dev');
  expect(alpha.world.recorded.filter(r => r.path === '/api/agents').length).toBeGreaterThan(0);
});

run('without a launch notification, a switch goes where it is pointed', async () => {
  const driver = getDriver();
  await launchWithFlags({GTMUX_DEBUG_SERVERS: macs(), GTMUX_DEBUG_LANG: 'en'});
  await driver.$('-ios predicate string:label BEGINSWITH "Mac Alpha"').click();
  await settle(4000);
  expect(await source()).toContain('alpha');
  await openServers();
  await driver.$('-ios predicate string:label BEGINSWITH "Mac Beta"').click();
  await settle(6000);
  expect(await source()).toContain('beta');
});
