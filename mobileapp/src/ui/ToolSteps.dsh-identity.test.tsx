import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {TouchableOpacity, View} from 'react-native';
import {ToolSteps} from './ToolSteps';
import {callTitle} from './toolPresentation';
import {build} from '../trace/model';
import {traceCounts, traceEventLabel} from '../trace/presentation';
import type {TranscriptStep} from '../api/client';

test('ASA010 DSH foreign id remains neutral in actual rows and Chat/Trace', () => {
  // The bounded cross-language runner supplies actual Go adapter output. The
  // default keeps this regression independently runnable in the normal RN suite.
  const steps: TranscriptStep[] = JSON.parse(process.env.GTMUX_ASA_NATIVE_STEPS || JSON.stringify([
    {kind:'tool', title:'skill', operation:'tool'},
    {kind:'tool', title:'skill', operation:'tool'},
    {kind:'tool', title:'skill', operation:'tool', action:'skill', detail:'design-kpis', subject_count:1},
    {kind:'tool', title:'skill', operation:'tool', action:'skill', detail:'design-kpis', subject_count:1},
  ]));
  expect(steps).toHaveLength(4);
  const before = JSON.stringify(steps);
  for (const lang of ['en', 'zh'] as const) {
    let tree!: renderer.ReactTestRenderer;
    act(() => {tree = renderer.create(<ToolSteps steps={steps} open onToggle={() => {}} lang={lang} fontSize={18} />);});
    try {
      const rows = [
        ...tree.root.findAllByType(TouchableOpacity).filter(x => String(x.props.testID).startsWith('tool-step-toggle-')),
        ...tree.root.findAllByType(View).filter(x => String(x.props.testID).startsWith('tool-step-static-')),
      ].sort((a, b) => Number(String(a.props.testID).split('-').pop()) - Number(String(b.props.testID).split('-').pop()));
      expect(rows).toHaveLength(4);
      steps.forEach((step, i) => {
        const title = callTitle(step, lang);
        const knownTitle = lang === 'en' ? 'Read skill' : '读取技能';
        if (i < 2) {
          expect(rows[i].props.accessibilityLabel).not.toContain(knownTitle);
          expect(step.action || '').toBe('');
          expect(title).not.toBe(knownTitle);
        } else {
          expect(title).toBe(knownTitle);
          expect(rows[i].props.accessibilityLabel).toContain(knownTitle);
        }
        const trace = build([{agent:i === 3 ? 'opencode' : 'dsh', prompt:'fixture', segments:[{steps:[step]}]}]);
        const event = trace.events.find(e => e.kind === 'tool')!;
        expect(traceEventLabel(event, lang)).toBe(title);
        expect(traceCounts(trace.events).calls).toBe(1);
      });
      expect(JSON.stringify(steps)).toBe(before);
    } finally {act(() => tree.unmount());}
  }
});
