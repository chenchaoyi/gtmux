import {parseInline, parseBlocks} from './markdown';

describe('parseInline', () => {
  it('parses bold, italic, code, links and plain text', () => {
    expect(parseInline('plain')).toEqual([{t: 'text', s: 'plain'}]);
    expect(parseInline('a **b** c')).toEqual([
      {t: 'text', s: 'a '},
      {t: 'b', s: 'b'},
      {t: 'text', s: ' c'},
    ]);
    expect(parseInline('use `npm i`')).toEqual([
      {t: 'text', s: 'use '},
      {t: 'code', s: 'npm i'},
    ]);
    expect(parseInline('see [docs](https://x.io)')).toEqual([
      {t: 'text', s: 'see '},
      {t: 'link', s: 'docs', href: 'https://x.io'},
    ]);
    expect(parseInline('*em*')).toEqual([{t: 'i', s: 'em'}]);
  });

  it('bold wins over italic at the same spot', () => {
    expect(parseInline('**x**')).toEqual([{t: 'b', s: 'x'}]);
  });

  it('does NOT italicize snake_case identifiers (no underscore emphasis)', () => {
    expect(parseInline('call my_func_name now')).toEqual([{t: 'text', s: 'call my_func_name now'}]);
  });
});

describe('parseBlocks', () => {
  it('parses headings, paragraphs, lists, code fences and hr', () => {
    const src = '# Title\n\nintro line\n\n- one\n- two\n\n```js\ncode();\n```\n\n---\n> quote';
    const b = parseBlocks(src);
    expect(b[0]).toEqual({t: 'h', level: 1, spans: [{t: 'text', s: 'Title'}]});
    expect(b[1]).toEqual({t: 'p', spans: [{t: 'text', s: 'intro line'}]});
    expect(b[2]).toEqual({
      t: 'ul',
      items: [[{t: 'text', s: 'one'}], [{t: 'text', s: 'two'}]],
    });
    expect(b[3]).toEqual({t: 'code', lang: 'js', text: 'code();'});
    expect(b[4]).toEqual({t: 'hr'});
    expect(b[5]).toEqual({t: 'quote', spans: [{t: 'text', s: 'quote'}]});
  });

  it('keeps fenced code verbatim (no inline parsing inside)', () => {
    const b = parseBlocks('```\na = **not bold**\n```');
    expect(b[0]).toEqual({t: 'code', lang: '', text: 'a = **not bold**'});
  });

  it('parses ordered lists', () => {
    const b = parseBlocks('1. first\n2. second');
    expect(b[0]).toEqual({t: 'ol', start: 1, items: [[{t: 'text', s: 'first'}], [{t: 'text', s: 'second'}]]});
  });

  // The bug behind "every item renders as 1." (reported 2026-08-09 against a chat message
  // whose terminal original read 1, 2, 3, 4, 5): an item's wrapped prose ENDED the list, so
  // each numbered line became its own single-item block and the renderer's `i + 1` restarted
  // at 1 every time. A list item owns its continuation lines.
  it('a list item absorbs its continuation lines instead of ending the list', () => {
    const b = parseBlocks('1. first item\n   wrapped prose under it\n2. second item\n3. third');
    expect(b).toHaveLength(1);
    expect(b[0]).toEqual({
      t: 'ol',
      start: 1,
      items: [
        [{t: 'text', s: 'first item wrapped prose under it'}],
        [{t: 'text', s: 'second item'}],
        [{t: 'text', s: 'third'}],
      ],
    });
  });

  it('bulleted items absorb continuations too', () => {
    const b = parseBlocks('- first\n  more of first\n- second');
    expect(b).toHaveLength(1);
    expect(b[0]).toEqual({
      t: 'ul',
      items: [[{t: 'text', s: 'first more of first'}], [{t: 'text', s: 'second'}]],
    });
  });

  // A list that genuinely starts at 3 renders 3, 4, 5 — the renderer counts from `start`.
  it('keeps the first item\'s number as the start', () => {
    const b = parseBlocks('3. third\n4. fourth');
    expect(b[0]).toEqual({t: 'ol', start: 3, items: [[{t: 'text', s: 'third'}], [{t: 'text', s: 'fourth'}]]});
  });

  // Continuation must not swallow the NEXT block — a blank line, a heading, a fence or a
  // table after an item all still end the list.
  it('a block starter after an item ends the list', () => {
    expect(parseBlocks('1. only\n\na separate paragraph').map(x => x.t)).toEqual(['ol', 'p']);
    expect(parseBlocks('1. only\n## a heading').map(x => x.t)).toEqual(['ol', 'h']);
    expect(parseBlocks('1. only\n```\ncode\n```').map(x => x.t)).toEqual(['ol', 'code']);
    expect(parseBlocks('- a\n1. b').map(x => x.t)).toEqual(['ul', 'ol']);
  });

  it('parses a GitHub pipe table with alignment and inline cells', () => {
    const src = '| Name | Qty |\n|:-----|----:|\n| `a`  | 1   |\n| b    | 22  |';
    const b = parseBlocks(src);
    expect(b[0]).toEqual({
      t: 'table',
      align: ['left', 'right'],
      header: [[{t: 'text', s: 'Name'}], [{t: 'text', s: 'Qty'}]],
      rows: [
        [[{t: 'code', s: 'a'}], [{t: 'text', s: '1'}]],
        [[{t: 'text', s: 'b'}], [{t: 'text', s: '22'}]],
      ],
    });
  });

  it('center alignment via :-:', () => {
    const b = parseBlocks('| a | b |\n| :-: | --- |\n| 1 | 2 |');
    expect((b[0] as any).align).toEqual(['center', 'left']);
  });

  it('does NOT treat a bare --- as a table (stays an hr)', () => {
    const b = parseBlocks('text\n\n---');
    expect(b[1]).toEqual({t: 'hr'});
  });

  it('a paragraph immediately followed by a table does not swallow it', () => {
    const b = parseBlocks('intro\n| a | b |\n|---|---|\n| 1 | 2 |');
    expect(b[0]).toEqual({t: 'p', spans: [{t: 'text', s: 'intro'}]});
    expect((b[1] as any).t).toBe('table');
  });
});
