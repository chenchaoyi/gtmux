import {parseBoardSections} from './boardSections';

describe('parseBoardSections', () => {
  it('splits at ## and keeps the heading text', () => {
    const s = parseBoardSections('## One\nalpha\n\n## Two\nbeta\n');
    expect(s.map(x => x.title)).toEqual(['One', 'Two']);
    expect(s[0].body).toBe('alpha');
    expect(s[1].body).toBe('beta');
  });

  it('keeps content that comes before the first heading', () => {
    const s = parseBoardSections('intro line\n\n## One\nalpha\n');
    expect(s[0].title).toBe('');
    expect(s[0].body).toBe('intro line');
    expect(s[1].title).toBe('One');
  });

  it('does not split on a ## inside a fenced code block', () => {
    // The board quotes shell and JSON constantly. Splitting on a comment would cut a
    // section in half at a line that is not a heading at all.
    const md = '## One\n```sh\n## not a heading\necho hi\n```\nstill section one\n\n## Two\nb';
    const s = parseBoardSections(md);
    expect(s.map(x => x.title)).toEqual(['One', 'Two']);
    expect(s[0].body).toContain('## not a heading');
    expect(s[0].body).toContain('still section one');
  });

  it('keeps deeper headings inside the body for the renderer', () => {
    const s = parseBoardSections('## One\n### Detail\ntext\n');
    expect(s).toHaveLength(1);
    expect(s[0].body).toBe('### Detail\ntext');
  });

  it('preserves the AUTHOR order and never sorts', () => {
    // The board is not chronological: a pinned handoff sits at the top and the newest
    // progress is appended at the bottom. Re-ordering would misrepresent both.
    const s = parseBoardSections('## 2026-08-13 handoff\na\n\n## 2026-08-12\nb\n\n## 2026-08-13 later\nc');
    expect(s.map(x => x.title)).toEqual(['2026-08-13 handoff', '2026-08-12', '2026-08-13 later']);
  });

  it('gives every section a distinct key even when titles repeat', () => {
    const s = parseBoardSections('## Same\na\n\n## Same\nb');
    expect(s[0].key).not.toBe(s[1].key);
  });

  it('handles a board with no headings at all', () => {
    const s = parseBoardSections('just prose\nmore prose');
    expect(s).toHaveLength(1);
    expect(s[0].title).toBe('');
    expect(s[0].body).toBe('just prose\nmore prose');
  });

  it('is empty for an empty board, rather than one blank section', () => {
    expect(parseBoardSections('')).toEqual([]);
    expect(parseBoardSections('\n\n  \n')).toEqual([]);
  });
});

describe('the document title', () => {
  // The sheet has its own header saying "Situation board / 态势板". The file's `# ` title
  // rendered directly under it as a second, larger one.
  it('is dropped from the preamble', () => {
    const secs = parseBoardSections('# gtmux HQ — 态势板\n\nYour durable posture.\n\n## ① 现状\n\nrow');
    expect(secs[0].body).toBe('Your durable posture.');
    expect(secs[0].body).not.toContain('态势板');
  });

  it('leaves a section heading alone — only the preamble can hold a document title', () => {
    const secs = parseBoardSections('## ① 现状\n\n# not a document title\n\nrow');
    expect(secs[0].body).toContain('# not a document title');
  });

  it('drops a preamble that was nothing but the title', () => {
    const secs = parseBoardSections('# 态势板\n\n## ① 现状\n\nrow');
    expect(secs.map(s => s.title)).toEqual(['① 现状']);
  });
});
