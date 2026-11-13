import {activityPreview, activitySummary} from './activity';
import {callTitle} from './toolPresentation';
import {build, sourceInfo} from '../trace/model';
import {recordedStep, traceCounts, traceEventLabel} from '../trace/presentation';
import type {TranscriptStep} from '../api/client';

const cases = [
  [{kind: 'tool', title: 'Read', operation: 'read', action: 'skill', subject_count: 1, detail: 'design-kpis', input: '/skills/design-kpis/SKILL.md', input_kind: 'path'}, 'Read skill', '读取技能', 'design-kpis'],
  [{kind: 'tool', title: 'web.run', operation: 'web', action: 'web_search', detail: 'official metrics'}, 'Search web', '搜索网页', 'official metrics'],
  [{kind: 'tool', title: 'web.run', operation: 'web', action: 'web_open', detail: 'https://example.org/guide'}, 'Open webpage', '打开网页', 'https://example.org/guide'],
  [{kind: 'tool', title: 'web.run', operation: 'web', action: 'web_find', detail: 'outcome'}, 'Find in page', '页内查找', 'outcome'],
  [{kind: 'tool', title: 'exec', operation: 'read', subject_count: 3, detail: 'a.md, b.md, c.md', input: 'cat a.md b.md c.md', input_kind: 'command', input_ref: 'scoped-ref'}, 'Read 3 files', '读取 3 个文件', 'a.md, b.md, c.md'],
] as const;

test('ASA004 recorded actions and targets agree across five agents and Trace without adding calls', () => {
  for (const agent of ['codex', 'claude', 'kimi', 'opencode', 'dsh']) for (const [source, en, zh, preview] of cases) {
    const step: TranscriptStep = JSON.parse(JSON.stringify(source));
    const before = JSON.stringify(step);
    expect([callTitle(step, 'en'), callTitle(step, 'zh'), activityPreview(step)]).toEqual([en, zh, preview]);
    const index = build([{agent, prompt: 'q', segments: [{steps: [step]}]}]);
    const event = index.events.find(e => e.kind === 'tool')!;
    expect([traceEventLabel(event, 'en'), traceEventLabel(event, 'zh')]).toEqual([en, zh]);
    expect(traceCounts(index.events).calls).toBe(1);
    expect(recordedStep(event).input_ref).toBe(step.input_ref || '');
    expect(recordedStep(event).action).toBe(step.action || '');
    expect(recordedStep(event).subject_count).toBe(step.subject_count);
    if (step.action) expect(sourceInfo(event).action).toBe(step.action);
    expect(JSON.stringify(step)).toBe(before);
  }
  expect(activitySummary([JSON.parse(JSON.stringify(cases[1][0]))], 'en')).toBe('Search web');
});

test('ASA005 unknown incompatible and malformed facts use bounded neutral fallback', () => {
  for (const count of [-1, 1.5, 10001, '3', null]) {
    const step: TranscriptStep = JSON.parse(JSON.stringify({kind: 'tool', title: 'Read', operation: 'read', action: 'unknown', subject_count: count, detail: 'file.md'}));
    expect(callTitle(step)).toBe('Read file');
  }
  const step: TranscriptStep = JSON.parse(JSON.stringify({kind: 'tool', title: 'exec', operation: 'command', action: 'skill', subject_count: 3, input: 'cat /skills/design-kpis/SKILL.md'}));
  expect(callTitle(step)).toBe('Run command');
  expect(callTitle({kind: 'tool', title: 'WebSearch'})).toBe('Search web');
  expect(callTitle({kind: 'tool', title: 'WebFetch'})).toBe('Open webpage');
  expect(callTitle({kind: 'tool', title: 'Read', input: 'SKILL.md'})).toBe('Read file');
  expect(activityPreview({...step, detail: 'x'.repeat(20000)}).length).toBeLessThanOrEqual(160);
});
