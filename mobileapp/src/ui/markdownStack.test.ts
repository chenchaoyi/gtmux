import {stackRows} from './MarkdownView';
import {Inline} from './markdown';

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
