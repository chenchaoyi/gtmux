import {knowledgeReading} from './knowledgeReading';
import {resolveEntry} from './knowledgeModel';
import type {KnowledgeEntry} from '../api/client';

test('keeps every character while separating explicit history from literal guidance', () => {
  const source = 'Memory and disk are different.\r\n\r\nMeasure before changing resources.\r\n\r\n## 历史记录（旧判断仅供出处）\r\nOriginal historical paragraph.\r\n';
  const parts = knowledgeReading(source);
  expect(parts.lead).toBe('Memory and disk are different.\r\n\r\n');
  expect(parts.body).toBe('Measure before changing resources.\r\n\r\n');
  expect(parts.history).toBe('## 历史记录（旧判断仅供出处）\r\nOriginal historical paragraph.\r\n');
  expect(parts.lead + parts.body + parts.history).toBe(source);
});

test.each(['```', '~~~~'])('history-like headings inside %s code stay with the current body', fence => {
  const source = `Guidance.\n\n${fence}text\n## History\nExample only.\n${fence}\n\n## History notes\nActual history.`;
  const parts = knowledgeReading(source);
  expect(parts.body).toContain('## History\nExample only.');
  expect(parts.history).toBe('## History notes\nActual history.');
  expect(parts.lead + parts.body + parts.history).toBe(source);
});

test.each(['', '\n\n', '# Current guidance\nKeep this whole.', '- First item\n- Second item', '    code example', '> Quoted passage', '| Column |\n| --- |\n| Value |', '```text\n## History\nunclosed'])('non-prose openings stay whole rather than inventing a summary: %s', source => {
  const parts = knowledgeReading(source);
  expect(parts.lead).toBe(''); expect(parts.body).toBe(source); expect(parts.history).toBe('');
});

test('reference-style links retain their single Markdown scope', () => {
  const source = 'Read [the source][reference].\n\n## History\nOlder note.\n\n[reference]: https://example.invalid/source';
  expect(knowledgeReading(source)).toEqual({lead: '', body: source, history: ''});
});

test('comments and arbitrary headings do not hide prose as history', () => {
  const source = 'Guidance.\n\n<!--\n## History\n-->\n\n## Other information\nStill current.';
  const parts = knowledgeReading(source);
  expect(parts.history).toBe(''); expect(parts.lead + parts.body).toBe(source);
});

test('long guidance is preserved without generated or truncated advice', () => {
  const lead = 'Evidence matters. '.repeat(120);
  const source = lead + '\n\n## History\nEarlier observation.';
  const parts = knowledgeReading(source);
  expect(parts.lead).toBe(lead + '\n\n');
  expect(parts.lead + parts.body + parts.history).toBe(source);
});

test('language resolution happens before display-only sectioning', () => {
  const entry: KnowledgeEntry = {id: 'pitfalls/synthetic', topic: 'pitfalls', title: '原文', at: 1, lang: 'zh', body: '原文建议。\n\n## 历史记录\n原文历史。', alt_lang: 'en', alt_title: 'Translation', alt_body: 'Translated guidance.\n\n## History\nTranslated history.'};
  const english = knowledgeReading(resolveEntry(entry, false).body);
  expect(english.lead).toBe('Translated guidance.\n\n');
  const fallback = knowledgeReading(resolveEntry({...entry, alt_body: undefined, alt_title: undefined}, false).body);
  expect(fallback.lead).toBe('原文建议。\n\n');
});
