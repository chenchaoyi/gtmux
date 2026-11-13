import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {TouchableOpacity} from 'react-native';
import {ToolSteps} from './ToolSteps';
import type {TranscriptStep} from '../api/client';

test('ASA004 actual App rows identify skill web intent and file targets with unchanged inspector input', () => {
  const steps: TranscriptStep[] = JSON.parse(JSON.stringify([
    {kind: 'tool', title: 'Read', operation: 'read', action: 'skill', detail: 'design-kpis', input: '/skills/design-kpis/SKILL.md', input_kind: 'path'},
    {kind: 'tool', title: 'web.run', operation: 'web', action: 'web_search', detail: 'official metrics'},
    {kind: 'tool', title: 'web.run', operation: 'web', action: 'web_find', detail: 'outcome'},
    {kind: 'tool', title: 'exec', operation: 'read', subject_count: 3, detail: 'a.md, b.md, c.md', input: 'cat a.md b.md c.md', input_ref: 'same-ref'},
  ]));
  const original = JSON.stringify(steps);
  for (const [lang, titles] of [['en', ['Read skill', 'Search web', 'Find in page', 'Read 3 files']], ['zh', ['读取技能', '搜索网页', '页内查找', '读取 3 个文件']]] as const) {
    let tree!: renderer.ReactTestRenderer;
    act(() => {tree = renderer.create(<ToolSteps steps={steps} open onToggle={() => {}} lang={lang} fontSize={18} />);});
    try {
      const rows = tree.root.findAllByType(TouchableOpacity).filter(x => String(x.props.testID).startsWith('tool-step-toggle-'));
      expect(rows).toHaveLength(4);
      titles.forEach((title, i) => expect(rows[i].props.accessibilityLabel).toContain(title));
      expect(rows[0].props.accessibilityLabel).toContain('design-kpis');
      expect(rows[2].props.accessibilityLabel).toContain('outcome');
      expect(JSON.stringify(steps)).toBe(original);
    } finally {act(() => tree.unmount());}
  }
});
