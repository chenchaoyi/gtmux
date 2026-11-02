const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const {scenario} = require('./harness.cjs');

async function fitted(page, selector) {
  await page.locator(selector + ' .xterm-screen').waitFor();
  await page.waitForFunction(selector => document.querySelector(selector + ' .xterm-rows')?.innerText.includes('边界'), selector);
  await page.waitForTimeout(250);
  const geometry = await page.locator(selector).evaluate(host => {
    const term = host.querySelector('.xterm'), screen = host.querySelector('.xterm-screen');
    const h = host.getBoundingClientRect(), s = screen.getBoundingClientRect(), t = term.getBoundingClientRect();
    const p = getComputedStyle(term);
    return {hostLeft:h.left, hostRight:h.right, hostBottom:h.bottom, left:s.left, right:s.right, bottom:s.bottom,
      termRight:t.right, inset:parseFloat(p.paddingRight), rowText:host.querySelector('.xterm-rows')?.innerText};
  });
  assert.ok(geometry.right <= geometry.hostRight - geometry.inset + 1, JSON.stringify(geometry));
  assert.ok(geometry.left >= geometry.hostLeft, JSON.stringify(geometry));
  assert.ok(geometry.bottom <= geometry.hostBottom + 1, JSON.stringify(geometry));
  await page.waitForFunction(selector => document.querySelector(selector + ' .xterm-rows')?.innerText.includes('边界'), selector);
}
for (const deviceScaleFactor of [1,2]) {
  scenario(`Terminal grid fits desktop tile and focus at DPR ${deviceScaleFactor}`, {viewport:{width:1440,height:900},deviceScaleFactor}, async(page,fixture) => {
    fixture.state.screens['%901'] = ('中文边界'.repeat(45)+'END\n').repeat(12);
    await page.goto(fixture.url+'/#code=TEST-1234');
    await page.locator('#tree').getByText('Investigate retries',{exact:true}).dblclick();
    await page.locator('.tile-modes [data-m="term"]').click();
    await fitted(page,'.tile-body .xt');
    await page.locator('.tile-name').click();
    await fitted(page,'.tile-body .xt');
    await page.setViewportSize({width:1000,height:760});
    await fitted(page,'.tile-body .xt');
    await page.locator('.tile-btn[data-action="focus"]').click();
    await fitted(page,'#term');
    if(process.env.GTMUX_WEB_EVIDENCE) {
      await fs.mkdir(process.env.GTMUX_WEB_EVIDENCE,{recursive:true});
      await page.screenshot({path:path.join(process.env.GTMUX_WEB_EVIDENCE,`terminal-fit-${deviceScaleFactor}.png`)});
    }
  });
}
scenario('Terminal grid fits narrow browser after rotation', {viewport:{width:390,height:844},locale:'zh-CN',hasTouch:true}, async(page,fixture) => {
  fixture.state.screens['%901'] = ('中文边界'.repeat(40)+'END\n').repeat(8);
  await page.goto(fixture.url+'/#code=TEST-1234');
  await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();
  await page.locator('#mode [data-mode="term"]').click();
  await fitted(page,'#term');
  await page.setViewportSize({width:844,height:390});
  await fitted(page,'#term');
});
