// A phone the Mac has revoked. The Mac still answers, and answers 401 — so the app has to
// say "access rejected" (pair again), never "Can't reach" (go find a network), and say it
// the same way on every screen the reader passes through.
//
// Measured on the simulator, 2026-10-05, before this was fixed: a revoke that happened
// while the app was closed opened on "Can't reach debug", because the live stream's own
// refusal was read as a dropped connection and it landed after the HTTP read's verdict;
// an open pane said "reconnecting"; the server list, where the banner sends the reader,
// said "Offline".
import {getDriver} from '../setup/driver';
import {screenshot} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

let fake: Fake;
beforeAll(async () => {
  fake = await startFake();
});
afterAll(async () => {
  await fake?.close();
});

const BANNER = '~Access rejected, re-pair';
const flags = () => ({GTMUX_DEBUG_PAIR_URL: fake.url, GTMUX_DEBUG_PAIR_TOKEN: fake.token, GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LANG: 'en'});

it('a phone revoked while the app was closed opens on "access rejected", and the server list agrees', async () => {
  const driver = getDriver();
  fake.world.reset();
  fake.world.revoked = true;
  await launchWithFlags(flags());
  await driver.$(BANNER).waitForExist({timeout: 20_000});
  // Both the HTTP read and the stream are retried on a backoff and both are refused each
  // time; whichever lands last, the verdict must stay put across a few attempts.
  for (let i = 0; i < 4; i++) {
    await settle(3000);
    expect(await driver.$(BANNER).isExisting()).toBe(true);
    expect(await driver.$("-ios predicate string:label BEGINSWITH \"Can't reach\"").isExisting()).toBe(false);
  }
  await screenshot('revoked-cold-radar');

  // The banner sends the reader to the server list to pair again: the Mac must read the
  // same there.
  await driver.$(BANNER).click();
  await driver.$(`~${TestIds.servers.screen}`).waitForExist({timeout: 10_000});
  await settle(1500);
  await screenshot('revoked-servers');
  expect(await driver.$('-ios predicate string:label == "debug, Access rejected"').isExisting()).toBe(true);
  expect(await driver.$('-ios predicate string:label == "debug, Offline"').isExisting()).toBe(false);
});

it('a pane open when the Mac revokes the phone says so in its header', async () => {
  const driver = getDriver();
  fake.world.reset();
  await launchWithFlags(flags());
  await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
  await settle(2000);
  await driver.$('~agent-row-%12').click();
  await driver.$(`~${TestIds.detail.screen}`).waitForExist({timeout: 15_000});
  await settle(1500);
  fake.world.revoked = true;
  fake.bumpAgents();
  const rejected = driver.$('-ios predicate string:label CONTAINS "debug access rejected ·"');
  await rejected.waitForExist({timeout: 15_000});
  await screenshot('revoked-detail');
  expect(await driver.$('-ios predicate string:label CONTAINS "reconnecting ·"').isExisting()).toBe(false);
});
