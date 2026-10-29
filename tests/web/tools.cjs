// Exercise the real tools entry; modes remain Chat and Terminal.
async function openTools(page, kind = 'trace', tile = false) {
  await page.locator((tile ? '.tile ' : '#session-actions ') + '[data-tool="' + kind + '"]').click();
  await page.locator('.session-tool').waitFor();
  return page.locator('.session-tool');
}
async function closeTools(page) { await page.locator('.session-tool > .dialog-head .dialog-close').click(); await page.locator('.session-tool').waitFor({state:'detached'}); }
module.exports = {openTools, closeTools};
