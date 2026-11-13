import {activityPreview} from './activity';
import {callTitle} from './toolPresentation';
import {build} from '../trace/model';
import {traceCounts, traceEventLabel} from '../trace/presentation';
import type {TranscriptStep} from '../api/client';

test('ASA008 native provider skill and queries facts agree in Chat and Trace', () => {
  for (const agent of ['opencode', 'dsh']) for (const [step, en, zh, preview] of [
    [{kind:'tool', title:'skill', operation:'tool', action:'skill', detail:'design-kpis', subject_count:1}, 'Read skill', '读取技能', 'design-kpis'],
    [{kind:'tool', title:'web_search', operation:'web', action:'web_search', detail:'official metrics, delivery metrics'}, 'Search web', '搜索网页', 'official metrics, delivery metrics'],
  ] as const) {
    const recorded: TranscriptStep = {...step};
    const before = JSON.stringify(recorded);
    expect([callTitle(recorded, 'en'), callTitle(recorded, 'zh'), activityPreview(recorded)]).toEqual([en, zh, preview]);
    const trace = build([{agent, prompt:'fixture', segments:[{steps:[recorded]}]}]);
    const event = trace.events.find(e => e.kind === 'tool')!;
    expect([traceEventLabel(event, 'en'), traceEventLabel(event, 'zh')]).toEqual([en, zh]);
    expect(traceCounts(trace.events).calls).toBe(1);
    expect(JSON.stringify(recorded)).toBe(before);
  }
});
