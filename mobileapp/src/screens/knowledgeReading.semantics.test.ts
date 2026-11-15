import {knowledgeReading} from './knowledgeReading';
import {parseBlocks} from '../ui/markdown';

function sections(source: string) {
  const parts = knowledgeReading(source);
  expect(parts.lead + parts.body + parts.history).toBe(source);
  return parts;
}

test('History API is current guidance rather than an explicit historical title', () => {
  const source = 'Use browser navigation.\n\n## History API\nCall pushState for current navigation.';
  const parts = sections(source);
  expect(parts.history).toBe('');
  expect(parts.lead + parts.body).toContain('Call pushState for current navigation.');
});

test('a current peer section after History remains visible by default', () => {
  const source = 'Advice.\n\n## History\nOld record.\n\n## Current rules\nUse the current limit.';
  expect(sections(source)).toEqual({lead: '', body: source, history: ''});
});

test('a multiline comment across the first blank stays hidden through actual Markdown parsing', () => {
  // The three primary cases reproduce the independent fad5 semantic findings.
  const source = 'Current advice. <!-- internal annotation\n\n## History\nINTERNAL_ONLY\n-->\nContinue advice.';
  const parts = sections(source);
  const visible = [parts.lead, parts.body, parts.history].filter(Boolean).flatMap(parseBlocks);
  expect(JSON.stringify(parseBlocks(source))).not.toContain('INTERNAL_ONLY');
  expect(JSON.stringify(visible)).not.toContain('INTERNAL_ONLY');
  expect(visible).toEqual(parseBlocks(source));
});

test.each(['History API usage', 'History notes for navigation', '历史记录查询'])('an ordinary title with a historical prefix stays current: %s', title => {
  const source = `Advice.\n\n## ${title}\nCurrent instructions.`;
  const parts = sections(source);
  expect(parts.history).toBe('');
  expect(parts.lead + parts.body).toContain('Current instructions.');
});

test.each(['History', 'History records', 'History note', 'History: earlier context', 'History (earlier context)', '历史', '历史记录（旧判断仅供出处）', 'History ###'])('a complete supported historical title can fold: %s', title => {
  const source = `Advice.\n\n## ${title}\nOld record.`;
  const parts = sections(source);
  expect(parts.history).toBe(`## ${title}\nOld record.`);
  expect(parts.lead).toBe('Advice.\n\n');
});

test('a higher current heading after historical subheadings keeps the whole body open', () => {
  const source = 'Advice.\n\n## History\n### Earlier context\nOld.\n\n# Current rules\nKeep these visible.';
  expect(sections(source)).toEqual({lead: '', body: source, history: ''});
});

test('nested history and code or quote heading lookalikes stay within history', () => {
  const history = '## History\n### Earlier context\nOld.\n\n```md\n# Current rules\nExample.\n```\n> # Quoted heading';
  expect(sections(`Advice.\n\n${history}`).history).toBe(history);
});

test.each([
  'Advice. <!-- INTERNAL_ONLY -->\n\nMore advice.',
  'Advice.\n\n<!-- INTERNAL_ONLY\n\ncontinued annotation -->\n\n## History\nOld record.',
])('comment-containing source retains one Markdown scope: %s', source => {
  const parts = sections(source);
  expect(parts).toEqual({lead: '', body: source, history: ''});
  const visible = [parts.lead, parts.body, parts.history].filter(Boolean).flatMap(parseBlocks);
  expect(visible).toEqual(parseBlocks(source));
  expect(JSON.stringify(visible)).not.toContain('INTERNAL_ONLY');
});
