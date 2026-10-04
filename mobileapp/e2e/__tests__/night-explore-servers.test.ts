// Night exploration (not for merge as-is): two fake Macs with different names and fleets,
// to look at the server list, switching, a dropped Mac and its return. Dumps a screenshot
// and the accessibility tree at every step, so the first run is a map, not a verdict.
import {writeFileSync} from 'fs';
import {join} from 'path';
import {getDriver, getArtifactsDir} from '../setup/driver';
import {screenshot} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

const HOME_PORT = Number(process.env.NIGHT_HOME_PORT) || 47811;
const OFFICE_PORT = Number(process.env.NIGHT_OFFICE_PORT) || 47812;

let home: Fake;
let office: Fake;

function label(f: Fake, prefix: string) {
  for (const a of f.world.agents) a.session = `${prefix}-${a.session}`;
}

beforeAll(async () => {
  home = await startFake({port: HOME_PORT});
  office = await startFake({port: OFFICE_PORT});
  label(home, 'home');
  label(office, 'office');
});
afterAll(async () => {
  await home?.close();
  await office?.close();
});

async function dump(name: string) {
  const driver = getDriver();
  await screenshot(name);
  writeFileSync(join(getArtifactsDir(), `${name}.xml`), await driver.getPageSource(), 'utf8');
}

const connState = async (): Promise<string> => {
  const src = await getDriver().getPageSource();
  const m = /name="(Connection:[^"]*)"/.exec(src) ?? /name="(连接：[^"]*)"/.exec(src);
  return m ? m[1] : '(none)';
};

// Which Mac's fleet is on screen: each fake's sessions carry its own prefix.
const fleet = async (): Promise<string> => {
  const src = await getDriver().getPageSource();
  const h = (src.match(/home-/g) ?? []).length;
  const o = (src.match(/office2?-/g) ?? []).length;
  return `home=${h} office=${o}`;
};

describe('night: two Macs', () => {
  it('maps the server list, a switch, a drop and a return', async () => {
    const driver = getDriver();
    const servers = JSON.stringify([
      {url: home.url, token: home.token, name: 'Home'},
      {url: office.url, token: office.token, name: 'Office'},
    ]);
    await launchWithFlags({
      GTMUX_DEBUG_NO_PUSH: '1',
      GTMUX_DEBUG_LOG_NET: '1',
      GTMUX_DEBUG_RESET_SERVERS: '1',
      GTMUX_DEBUG_SERVERS: servers,
    });
    await settle(4000);
    await dump('n1-launch');
    // eslint-disable-next-line no-console
    console.log('[night] conn after launch:', await connState());

    // Whatever the launch landed on, get to the servers page.
    const chip = driver.$(`~${TestIds.radar.serverChip}`);
    if (await chip.isDisplayed().catch(() => false)) {
      await chip.click();
      await settle(1500);
    }
    await dump('n2-servers');

    const office1 = driver.$('-ios predicate string:label CONTAINS "Office" AND type != "XCUIElementTypeStaticText"');
    const officeAny = (await office1.isExisting()) ? office1 : driver.$('-ios predicate string:label CONTAINS "Office"');
    await officeAny.click();
    await settle(4000);
    await dump('n3-office');
    // eslint-disable-next-line no-console
    console.log('[night] conn on office:', await connState());
    // eslint-disable-next-line no-console
    console.log('[night] fleet on screen:', await fleet());

    // Drop the Office Mac entirely, then bring it back on the same address.
    await office.close();
    await settle(6000);
    await dump('n4-office-gone');
    // eslint-disable-next-line no-console
    console.log('[night] conn with office gone:', await connState());
    office = await startFake({port: OFFICE_PORT});
    label(office, 'office2');
    let back = '';
    for (let i = 0; i < 30; i++) {
      back = await connState();
      if (/connected|已连接/.test(back) && !/not|未/.test(back)) break;
      await settle(1000);
    }
    await dump('n5-office-back');
    // eslint-disable-next-line no-console
    console.log('[night] conn after office returned:', back);
  });
});
