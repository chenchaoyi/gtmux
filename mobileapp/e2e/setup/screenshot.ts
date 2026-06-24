import {writeFileSync} from 'fs';
import {join} from 'path';
import {getArtifactsDir, getDriver} from './driver';

/** Save a screenshot of the current sim state; returns the absolute path. */
export async function screenshot(name: string): Promise<string> {
  const driver = getDriver();
  const safe = name.replace(/[^A-Za-z0-9._-]/g, '_');
  const path = join(getArtifactsDir(), safe.endsWith('.png') ? safe : `${safe}.png`);
  await driver.saveScreenshot(path);
  return path;
}

/** On failure: dump a screenshot + the XCUITest page-source XML, then rethrow. */
export async function captureOnFailure(label: string, err: unknown): Promise<never> {
  try {
    const png = await screenshot(`fail-${label}`);
    const xml = await getDriver().getPageSource();
    const xmlPath = join(getArtifactsDir(), `fail-${label}.xml`);
    writeFileSync(xmlPath, xml);
    // eslint-disable-next-line no-console
    console.error(`[e2e] FAIL @ ${label}\n  screenshot: ${png}\n  page source: ${xmlPath}`);
  } catch (capErr) {
    // eslint-disable-next-line no-console
    console.error(`[e2e] capture failed: ${capErr}`);
  }
  throw err;
}
