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
    expect(b[0]).toEqual({t: 'ol', items: [[{t: 'text', s: 'first'}], [{t: 'text', s: 'second'}]]});
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
