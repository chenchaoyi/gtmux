import {readFileSync} from 'fs';
import {resolve} from 'path';
const {manualMode, sendTarget, verifySendTarget} = require('./manual-tools.cjs');
const env = {GTMUX_E2E_MANUAL:'1', GTMUX_E2E_TOOL:'diagnostics', GTMUX_E2E_OWNED_FIXTURE:'1', GTMUX_E2E_PANE:'%901', GTMUX_E2E_URL:'http://127.0.0.1:19234', GTMUX_E2E_TOKEN:'synthetic'};

test('DG-001 ordinary discovery excludes all four manual tools without discarding them', () => {
  const auto = require('../jest.config.e2e.js');
  expect(auto.testMatch).toEqual(['<rootDir>/e2e/__tests__/**/*.test.ts']);
  expect(require('../../jest.config.js').testPathIgnorePatterns).toEqual(expect.arrayContaining(['/e2e/diagnostics/', '/e2e/capture/']));
  expect(() => manualMode({})).toThrow('Manual tools require');
  expect(manualMode(env)).toBe('diagnostics');
  for (const [folder,name] of [['diagnostics','ipad-keys-probe'],['diagnostics','send-repro'],['capture','hq-header-shot'],['capture','knowledge-shot']]) {
    const source = readFileSync(resolve(__dirname, '..', folder, name+'.test.ts'),'utf8');
    expect(source.length).toBeGreaterThan(100);
    if (folder==='capture') expect(source).not.toMatch(/knowledge-act-land.*\.click/);
  }
});

test('DG-002 rejects missing target, opt-in, foreign origin and real nonfixture endpoints before driver access', async () => {
  for (const overrides of [{GTMUX_E2E_PANE:''}, {GTMUX_E2E_PANE:'%9;bad'}, {GTMUX_E2E_OWNED_FIXTURE:''}, {GTMUX_E2E_TOKEN:''}, {GTMUX_E2E_URL:'https://example.org'}, {GTMUX_E2E_URL:'http://127.0.0.1@elsewhere/'}, {GTMUX_E2E_URL:'http://localhost/api'}]) {
    const request = jest.fn();
    await expect(verifySendTarget({...env,...overrides},request)).rejects.toThrow();
    expect(request).not.toHaveBeenCalled();
  }
  await expect(verifySendTarget(env,jest.fn().mockResolvedValue({ok:true,json:async()=>({ok:true})}))).rejects.toThrow('not the synthetic');
});

test('DG-003 confirms one exact fixture pane, rejects absence/ambiguity, and send repro selects that pane', async () => {
  for (const rows of [[], [{pane_id:'%902'}], [{pane_id:'%901'},{pane_id:'%901'}]]) {
    const request=jest.fn().mockResolvedValueOnce({ok:true,json:async()=>({fixture:'gtmux-e2e-fake-serve'})}).mockResolvedValueOnce({ok:true,json:async()=>rows});
    await expect(verifySendTarget(env,request)).rejects.toThrow('absent or ambiguous');
  }
  const request=jest.fn().mockResolvedValueOnce({ok:true,json:async()=>({fixture:'gtmux-e2e-fake-serve'})}).mockResolvedValueOnce({ok:true,json:async()=>[{pane_id:'%901'}]});
  expect((await verifySendTarget(env,request)).pane).toBe('%901');
  expect(sendTarget(env).pane).toBe('%901');
  const source=readFileSync(resolve(__dirname,'../diagnostics/send-repro.test.ts'),'utf8');
  expect(source).toContain('openAgentDetail(target.pane)');
  expect(source).not.toContain('openFirstAgentDetail');
  expect(source.indexOf('await verifySendTarget')).toBeLessThan(source.indexOf('const driver = getDriver()'));
});
