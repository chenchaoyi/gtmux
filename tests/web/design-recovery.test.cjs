const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const {scenario} = require('./harness.cjs');

for (const [width, failure] of [[1440, 'message'], [390, 'later attachment']]) {
  scenario('DFR-003 edited upload recovery after ' + failure,
    {viewport: {width, height: 1000}, locale: 'en-US'}, async (page, fixture) => {
      let uploadCount = 0, sendCount = 0, retry = false, release;
      const held = new Promise(resolve => {release = resolve;});
      const uploaded = [], sent = [];
      fixture.state.expectedHttpErrors.push(500);
      await page.route('**/api/upload?*', async route => {
        const number = ++uploadCount;
        uploaded.push(route.request().postDataBuffer());
        if (!retry && failure === 'later attachment' && number === 2) {
          await route.fulfill({status: 500, contentType: 'application/json', body: '{"error":"synthetic upload refusal"}'}); return;
        }
        if (retry && number === 3) await held;
        await route.fulfill({status: 200, contentType: 'application/json', body: JSON.stringify({path: '/saved/receipt-' + number})});
      });
      await page.route('**/api/send', async route => {
        sendCount++; sent.push(route.request().postDataJSON());
        await route.fulfill({status: !retry && failure === 'message' ? 500 : 200,
          contentType: 'application/json', body: '{"status":"ok"}'});
      });
      try {
        await page.goto(fixture.url + '/#code=TEST-1234');
        if (width < 800) {
          await page.locator('#radar').getByText('Investigate retries', {exact: true}).click();
          await page.locator('#mode [data-mode="chat"]').click();
        } else {
          await page.locator('#tree').getByText('Investigate retries', {exact: true}).dblclick();
          await page.locator('.tile-modes [data-m="chat"]').click();
        }
        const composer = page.locator(width < 800 ? '#chat-input .composer' : '.tile .composer');
        const input = composer.locator('.cx-input');
        await input.fill('Retained synthetic caption');
        await composer.getByRole('button', {name: 'Add attachment', exact: true}).click();
        const chooser = page.waitForEvent('filechooser');
        await page.getByRole('dialog', {name: 'Add attachment', exact: true}).getByRole('button', {name: 'Photos', exact: true}).click();
        const png = await fs.readFile(path.resolve(__dirname, '../../assets/agent-icons/codex.png'));
        await (await chooser).setFiles(['first.png', 'second.png'].map(name => ({name, mimeType: 'image/png', buffer: png})));
        assert.equal(await page.locator('.image-editor').count(), 0, 'multi-pick stages without serial editors');
        await composer.locator('.cx-send').click();
        await page.waitForFunction(() => [...document.querySelectorAll('.cx-note')].some(n => !n.hidden && /failed/i.test(n.textContent)));
        assert.equal(uploadCount, 2); assert.equal(sendCount, failure === 'message' ? 1 : 0);
        assert.equal(await input.inputValue(), 'Retained synthetic caption');
        const first = composer.locator('.cx-attachment').filter({has: page.locator('img[alt="first.png"]')});
        assert.equal(await first.locator('.cx-upload-status').innerText(), 'Uploaded');
        const otherUrl = await composer.locator('img[alt="second.png"]').getAttribute('src');
        // Cancel an edit first: the previously valid upload remains usable.
        await first.locator('.cx-preview').click();
        await page.getByRole('dialog', {name: 'Annotate image'}).getByRole('button', {name: 'Use original', exact: true}).click();
        assert.equal(await first.locator('.cx-upload-status').innerText(), 'Uploaded');
        await first.locator('.cx-preview').click();
        const dialog = page.getByRole('dialog', {name: 'Annotate image'});
        await page.waitForFunction(() => !document.querySelector('.image-editor .dialog-primary').disabled);
        const box = await dialog.locator('canvas').boundingBox();
        await page.mouse.move(box.x + box.width * .2, box.y + box.height * .2); await page.mouse.down();
        await page.mouse.move(box.x + box.width * .7, box.y + box.height * .7); await page.mouse.up();
        await dialog.getByRole('button', {name: 'Done', exact: true}).click();
        await dialog.waitFor({state: 'detached'});
        const edited = composer.locator('.cx-attachment').filter({has: page.locator('img[alt="first-marked.jpg"]')});
        await edited.waitFor();
        assert.equal(await edited.locator('.cx-upload-status').count(), 0, 'edited bytes have no old upload receipt');
        assert.equal(await composer.locator('img[alt="second.png"]').getAttribute('src'), otherUrl);
        assert.equal(await input.inputValue(), 'Retained synthetic caption'); assert.equal(uploadCount, 2);
        retry = true; await composer.locator('.cx-send').click();
        await page.waitForFunction(() => !!document.querySelector('.cx-upload-status[data-phase="uploading"], .cx-upload-status[data-phase="saving"]'));
        assert.equal(await edited.locator('.cx-upload-status[data-phase="complete"]').count(), 0);
        assert.equal(sendCount, failure === 'message' ? 1 : 0, 'no send before replacement acknowledgment');
        if (process.env.GTMUX_WEB_RESULTS) {
          await fs.mkdir(process.env.GTMUX_WEB_RESULTS, {recursive: true});
          await page.screenshot({path: path.join(process.env.GTMUX_WEB_RESULTS, 'edited-retry-' + width + '.png')});
        }
        release();
        await page.waitForFunction(() => document.querySelectorAll('.cx-attachment').length === 0);
        assert.equal(uploadCount, failure === 'message' ? 3 : 4, 'only invalidated or previously failed files are reuploaded');
        assert.equal(await composer.locator('.cx-attachment').count(), 0);
        assert.match(uploaded[2].toString('latin1'), /filename="first-marked.jpg"/);
        assert.match(sent.at(-1).text, /Retained synthetic caption\n\/saved\/receipt-3\n\/saved\/receipt-(2|4)$/);
        assert.doesNotMatch(sent.at(-1).text, /receipt-1\b/);
      } finally {release();}
    });
}
