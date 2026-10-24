import {knowledgeUsage, knowledgeKind, knowledgeProvenance, knowledgeTopic} from './knowledgePresentation';
import {axesLine, matchEntries} from './knowledgeModel';
import {KnowledgeEntry} from '../api/client';
const e: KnowledgeEntry = {id: 'corrections/example', topic: 'corrections', title: 'Avoid duplicate controls', at: 1, kind: 'judgment', provenance: 'correction'};

test('default HQ lookup is visible and is different from a standing HQ instruction', () => {
  const base = knowledgeUsage(e, true);
  expect(base.title).toBe('HQ 按需查阅');
  expect(base.state).toBe('');
  const plan = knowledgeUsage({...e, promoted_at: 2, audience: 'hq'}, true);
  expect(plan.title).toBe('HQ 固定规则');
  expect(plan.state).toBe('待完成写入');
  expect(plan.line).not.toBe(base.line);
});
test('planning, recorded application and product feedback do not promise agent loading or a product release', () => {
  for (const a of ['hq', 'machine', 'repo']) {
    const pending = knowledgeUsage({...e, audience: a, promoted_at: 2}, false);
    const recorded = knowledgeUsage({...e, audience: a, promoted_at: 2, landed_at: 3}, false);
    expect(pending.state).toBe('Write pending');
    expect(recorded.state).toBe('Application recorded');
    expect(recorded.explanation).toContain('does not confirm');
  }
  expect(knowledgeUsage({...e, audience: 'everyone', promoted_at: 2}, true).state).toBe('待提交反馈');
});
test('legacy and future scopes remain unknown rather than being guessed as HQ-only or already applied', () => {
  for (const old of [{promoted_at: 2}, {landed_at: 3}, {audience: 'future-scope'}]) {
    expect(knowledgeUsage({...e, ...old}, true).title).toBe('用途待确认');
  }
});
test('Chinese metadata and search use visible vocabulary while preserving custom topic values', () => {
  expect(knowledgeKind('judgment', true)).toBe('判断依据');
  expect(knowledgeProvenance('correction', true)).toBe('用户纠正');
  expect(axesLine(e, true)).toBe('判断依据 · 来源：用户纠正');
  expect(knowledgeTopic('my-project', true)).toBe('my-project');
  expect(matchEntries([e], '判断依据 用户纠正')).toEqual([e]);
});
