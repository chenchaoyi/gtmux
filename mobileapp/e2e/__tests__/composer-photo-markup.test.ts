import {getDriver} from '../setup/driver';
import {screenshot} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

// A photo from the library, marked up, uploaded and referenced in the message that goes to
// the Mac: attach → Photos → pick → the markup editor (redact a region, Undo comes alive)
// → Done → upload → send carries the uploaded path.
//
// The system photo picker runs out of process: the app's accessibility tree cannot see
// it, so a photo is picked by position (the grid starts below the picker's bar and its
// privacy note). Needs a library with photos in it; a new simulator ships with samples.
let fake: Fake;
beforeAll(async () => {
  fake = await startFake();
});
afterAll(async () => {
  await fake?.close();
});

const label = (l: string) => getDriver().$(`-ios predicate string:label == "${l}" AND visible == 1`);

it('a library photo goes through markup and reaches the Mac with the message', async () => {
  const driver = getDriver();
  await launchWithFlags({GTMUX_DEBUG_PAIR_URL: fake.url, GTMUX_DEBUG_PAIR_TOKEN: fake.token, GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LANG: 'en'});
  await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
  await settle(2000);
  await driver.$('~agent-row-%12').click();
  await settle(2500);
  await driver.$(`~${TestIds.composer.keyboard}`).click();
  await driver.$(`~${TestIds.composer.input}`).waitForDisplayed({timeout: 8000});
  await driver.$(`~${TestIds.composer.attach}`).click();
  await settle(1500);
  await driver.$('~attach-0').click();
  await settle(15_000); // the picker loads the library out of process
  const win = await driver.getWindowSize();
  const at = (x: number, y: number) => ({x: Math.round(win.width * x), y: Math.round(win.height * y)});
  const tap = async (x: number, y: number) =>
    driver.action('pointer', {parameters: {pointerType: 'touch'}}).move(at(x, y)).down().pause(80).up().perform();
  await tap(0.17, 0.435);

  // The markup editor: nothing to undo until something is drawn.
  await label('Done').waitForExist({timeout: 15_000});
  expect(await label('Undo').getAttribute('enabled')).toBe('false');
  await driver.$('-ios predicate string:label CONTAINS "Redact" AND visible == 1').click();
  await settle(500);
  await driver
    .action('pointer', {parameters: {pointerType: 'touch'}})
    .move(at(0.25, 0.42))
    .down()
    .move({duration: 300, ...at(0.55, 0.5)})
    .up()
    .perform();
  await settle(800);
  expect(await label('Undo').getAttribute('enabled')).toBe('true');
  await screenshot('photo-markup-drawn');
  await label('Done').click();
  await settle(4000);

  const input = driver.$(`~${TestIds.composer.input}`);
  await input.setValue('look at this');
  await driver.$(`~${TestIds.composer.send}`).click();
  await settle(4000);
  await screenshot('photo-markup-sent');

  const uploads = fake.world.writesTo('/api/upload') as Array<{bytes: number; type: string; path: string; body: Buffer}>;
  expect(uploads).toHaveLength(1);
  expect(uploads[0].type).toBe('multipart/form-data');
  // What went up is the marked-up image as a JPEG: since #1333 the markup is exported at the
  // photo's own size as markup.jpg (ui/markupGeometry MARKUP_FILE), not as a PNG.
  expect(uploads[0].body.includes(Buffer.from('Content-Type: image/jpeg', 'latin1')) || uploads[0].body.includes(Buffer.from('content-type: image/jpeg', 'latin1'))).toBe(true);
  expect(uploads[0].body.includes(Buffer.from([0xff, 0xd8, 0xff]))).toBe(true);
  const sends = fake.world.writesTo('/api/send') as Array<{id: string; text: string; enter: boolean}>;
  expect(sends[sends.length - 1]).toMatchObject({id: '%12', text: `look at this\n${uploads[0].path}`, enter: true});
});
