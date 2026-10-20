import {activityPreview} from './activity';
import type {TranscriptStep} from '../api/client';

const parameters = (title: string, input: unknown): TranscriptStep => ({kind: 'tool', title, input: JSON.stringify(input), input_kind: 'parameters'});

test('delegation previews the purpose instead of JSON or an internal task key', () => {
  const step = parameters('spawn_agent', {task_name: 'review_settings', message: 'Review the settings screen', fork_turns: 'none'});
  expect(activityPreview(step, 'en')).toBe('Review the settings screen');
  expect(activityPreview(step, 'zh')).toBe('Review the settings screen');
  expect(activityPreview({...step, detail: 'Check layout'})).toBe('Check layout');
});

test.each([
  ['sleep', {duration_ms: 1500}, 'Requested 1.5s', '设定 1.5 秒'],
  ['wait', {cell_id: 'fixture-cell', yield_time_ms: 1000, max_tokens: 2000}, 'Up to 1s', '最多 1 秒'],
  ['wait_agent', {ids: ['one', 'two'], timeout_ms: 60000}, 'Timeout setting 60s', '超时设置 60 秒'],
])('%s previews recorded timing without claiming elapsed time or a result', (title, input, en, zh) => {
  expect(activityPreview(parameters(title, input), 'en')).toBe(en);
  expect(activityPreview(parameters(title, input), 'zh')).toBe(zh);
});

test('old object arguments work and unknown units are never inferred', () => {
  expect(activityPreview({...parameters('wait', {duration_ms: 0}), input_kind: 'arguments'}, 'en')).toBe('Requested 0s');
  expect(activityPreview(parameters('wait', {timeout: 10}), 'en')).toBe('Recorded parameters');
  expect(activityPreview(parameters('wait', {duration_ms: -1}), 'zh')).toBe('已记录参数');
  expect(activityPreview(parameters('wait', {duration_ms: '1000'}), 'en')).toBe('Recorded parameters');
});

test('unfamiliar, malformed or truncated parameters use a quiet fallback', () => {
  expect(activityPreview(parameters('future_tool', {extra: {value: 'keep in inspector'}}), 'zh')).toBe('已记录参数');
  expect(activityPreview({...parameters('wait', {}), input: '{"yield_time_ms":', details_truncated: true})).toBe('Recorded parameters');
  expect(activityPreview(parameters('spawn_agent', {message: ['not a string']}))).toBe('Recorded parameters');
});

test('prose and command previews are bounded without changing inspector input', () => {
  const step = {...parameters('spawn_agent', {}), input: JSON.stringify({message: '😀'.repeat(200)})};
  const preview = activityPreview(step);
  expect(preview.length).toBeLessThanOrEqual(160);
  expect(preview).toMatch(/…$/);
  expect(preview).not.toMatch(/[\uD800-\uDBFF]…$/);
  expect(step.input).toBe(JSON.stringify({message: '😀'.repeat(200)}));
  expect(activityPreview({kind: 'tool', title: 'exec', input: 'printf one\nprintf two', input_kind: 'command'})).toBe('printf one printf two');
});
