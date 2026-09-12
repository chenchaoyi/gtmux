import {getDriver} from '../setup/driver';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

// Diagnostic: does XCTest's hardware-key path type into a focused field at all on this
// simulator? Run by hand; not part of any suite's contract.
const url = process.env.GTMUX_E2E_URL;
const token = process.env.GTMUX_E2E_TOKEN;
const gated = url && token && process.env.GTMUX_KEYS_PROBE ? describe : describe.skip;

gated('keys probe', () => {
  it('types into the pane search with mobile: keys', async () => {
    const driver = getDriver();
    await launchWithFlags({GTMUX_DEBUG_PAIR_URL: url!, GTMUX_DEBUG_PAIR_TOKEN: token!, GTMUX_DEBUG_NO_PUSH: '1'});
    await driver.$(`~${TestIds.radar.split}`).waitForDisplayed({timeout: 25_000});
    await settle(2000);
    await driver.$(`~${TestIds.radar.panes}`).click();
    const field = driver.$(`~${TestIds.panes.search}`);
    await field.waitForDisplayed({timeout: 8_000});
    await field.click();
    await settle(600);
    await driver.execute('mobile: keys', {keys: [{key: 'a'}, {key: 'b'}, {key: 'c'}]});
    await settle(800);
    // eslint-disable-next-line no-console
    console.log('[probe] field value after mobile: keys =', JSON.stringify(await field.getText()));
    // With a first responder in place: does a menu command reach the app delegate?
    await driver.execute('mobile: keys', {keys: [{key: 'h', modifierFlags: (1 << 20) | (1 << 17)}]});
    await settle(1500);
    // eslint-disable-next-line no-console
    console.log('[probe] after cmd+shift+h with a focused field, HQ open =', await driver.$('~hq-knowledge-open').isExisting());
  });
});
