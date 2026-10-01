import {getDriver} from '../setup/driver';
import {launchWithFlags, settle, typeInto} from '../setup/app';
import {screenshot} from '../setup/screenshot';
import {startFake, Fake} from '../fake-serve/server';

let fake: Fake;
beforeEach(async () => { fake = await startFake(); });
afterEach(async () => { await fake?.close(); });

it('one keyboard-up tap reports a duplicate; another name creates and opens the exact terminal', async () => {
  const driver = getDriver();
  await launchWithFlags({GTMUX_DEBUG_PAIR_URL: fake.url, GTMUX_DEBUG_PAIR_TOKEN: fake.token,
    GTMUX_DEBUG_PAIR_NAME: 'Studio Mac', GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LANG: 'en'});
  const open = driver.$('~new-session-open');
  await open.waitForDisplayed({timeout: 25000});
  await open.click();
  await typeInto('new-session-name', 'gtmux dev');
  // Do not hide the keyboard or tap twice: the radar's responder ancestry used to
  // consume the first tap to dismiss it instead of delivering the create action.
  await driver.$('~new-session-create').click();
  await settle(1500);
  expect(fake.world.recorded.filter(r => r.path === '/api/sessions')).toHaveLength(1);
  expect(await driver.getPageSource()).toContain('That name is already in use');
  await screenshot('new-session-duplicate');
  await typeInto('new-session-name', 'Phone review');
  await driver.$('~new-session-create').click();
  await settle(2000);
  expect(fake.world.createdSessions.size).toBe(1);
  const receipt = [...fake.world.createdSessions.values()][0];
  const source = await driver.getPageSource();
  expect(source).toContain(receipt.pane_id);
  expect(source).toContain('New session ready');
  expect(source).not.toContain('new-session-name');
  await screenshot('new-session-created');
});
