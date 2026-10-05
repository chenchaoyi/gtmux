import {agentCount, summaryText} from './RadarSummary';

// The tally counts in each language. The word used to be 'agents' in both halves, so
// English read "1 agents" and Chinese "7 agents · 1 等输入" (%6, 2026-10-06).
describe('the radar tally', () => {
  it('counts agents in English, singular and plural', () => {
    expect(agentCount(1, 'en')).toBe('1 agent');
    expect(agentCount(0, 'en')).toBe('0 agents');
    expect(agentCount(7, 'en')).toBe('7 agents');
  });
  it('counts them in Chinese with 个', () => {
    expect(agentCount(7, 'zh')).toBe('7 个 agent');
  });
  it('leads the summary with that count', () => {
    const c = {total: 1, waiting: 1, errored: 0, working: 0, idle: 0} as unknown as Parameters<typeof summaryText>[0];
    expect(summaryText(c, 'en').startsWith('1 agent · ')).toBe(true);
    expect(summaryText({...c, total: 7} as typeof c, 'zh').startsWith('7 个 agent · ')).toBe(true);
  });
});
