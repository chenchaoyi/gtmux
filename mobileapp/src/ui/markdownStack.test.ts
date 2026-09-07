import {stackRows} from './MarkdownView';
import {Inline, parseBlocks, stripComments} from './markdown';

const t = (s: string): Inline[] => [{t: 'text', s} as Inline];

describe('stackRows', () => {
  // A phone is ~390pt wide. The board's table has seven columns, so it rendered three and
  // a half of them and cut the fourth mid-word — which reads as broken, not as scrollable.
  it('pairs every cell after the first with its column heading', () => {
    const header = [t('pane'), t('在做什么'), t('谁派的')];
    const rows = [[t('%23'), t('tmux-id-surface'), t('user-direct')]];
    expect(stackRows(header, rows)).toEqual([
      {head: t('%23'), fields: [{label: '在做什么', value: t('tmux-id-surface')}, {label: '谁派的', value: t('user-direct')}]},
    ]);
  });

  // Half the board's cells are empty or a dash. A label with nothing after it is noise.
  it('drops empty and placeholder cells', () => {
    const header = [t('pane'), t('等你定'), t('教训')];
    const rows = [[t('%7'), t('  '), t('—')]];
    expect(stackRows(header, rows)[0].fields).toEqual([]);
  });

  it('keeps the first cell as the row head — that is what the row IS', () => {
    expect(stackRows([t('pane')], [[t('%12')]])[0].head).toEqual(t('%12'));
  });

  // A ragged row (fewer cells than headings) is normal in hand-written markdown.
  it('survives a row shorter than the header', () => {
    const header = [t('pane'), t('在做什么'), t('状态')];
    expect(stackRows(header, [[t('%1')]])).toEqual([{head: t('%1'), fields: []}]);
  });

  it('survives a row with no cells at all', () => {
    expect(stackRows([t('pane')], [[]])[0].head).toEqual([]);
  });
});

// The situation board opens with `<!-- 写法规则:一格一句话… -->`, a note HQ leaves for
// whoever edits the board next. Rendered, it was the first thing a reader saw under the
// section heading: an instruction addressed to someone else, in the most prominent place
// on the page. (Both surfaces showed it; both strip it now.)
describe('stripComments', () => {
  it('drops an HTML comment, which is the author writing to themselves', () => {
    const md = '<!-- 写法规则:一格一句话 -->\n\nreal content';
    expect(parseBlocks(md).map(b => (b.t === 'p' ? b.spans.map(s => s.s).join('') : ''))).toContain('real content');
    expect(JSON.stringify(parseBlocks(md))).not.toContain('写法规则');
  });

  it('handles a comment that opens and closes mid-line', () => {
    expect(stripComments('before <!-- x --> after')).toBe('before  after');
  });

  it('handles several, and a multi-line one', () => {
    expect(stripComments('a<!--1-->b<!--\n2\n-->c')).toBe('abc');
  });

  it('KEEPS the text after an unterminated comment', () => {
    // HTML would call the rest a comment. The board is written by hand, and a typo'd
    //  would then blank it from that point with nothing on screen to say why. A
    // stray marker in the prose is the smaller failure. The Swift twin agrees.
    expect(stripComments('keep <!-- this too')).toBe('keep <!-- this too');
  });

  it('leaves ordinary text with angle brackets alone', () => {
    expect(stripComments('a < b and c > d')).toBe('a < b and c > d');
  });
});
