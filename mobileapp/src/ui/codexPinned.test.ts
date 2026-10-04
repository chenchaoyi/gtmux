import {hasCodexCutRow, splitCodexPinned} from './codexPinned';

// Captured from a real Codex (v0.160.0) in a throwaway 60x14 pane on 2026-10-04, colours
// and all; only the working directory in the status line is shortened.
const COLS = 60;
const tail = [
  '',
  '\x1b[0;1m›\x1b[0m \x1b[2mAsk Codex to do anything\x1b[0m',
  '',
  '  \x1b[38;2;246;226;183mGPT-6.1-Sol high\x1b[39m · \x1b[38;2;171;223;167m~/probe',
  '\x1b[39m  \x1b[1m←\x1b[0m for agents · \x1b[1m?\x1b[0m for shortcuts   ⚠ \x1b[38;2;196;167;103m1 warning\x1b[39m · \x1b[1mf2\x1b[0m to view',
];
const probe =
  'This is a throwaway rendering probe for gtmux tests; please do not edit any files. First run the shell command: ' +
  'for i in $(seq 1 30); do echo probe line $i; done — then reply with a numbered list of twelve fruit names, one per line, and nothing else.';
// Idle after the turn: the cut row stays (59 cells: the width minus one).
const idle = [
  '\x1b[1;2m› \x1b[0mThis is a throwaway rendering probe for gtmux tests; ple…',
  '  \x1b[38;5;12m8. \x1b[39mRaspberry',
  '  \x1b[38;5;12m9. \x1b[39mPeach',
  '  \x1b[38;5;12m10. \x1b[39mPear',
  '  \x1b[38;5;12m11. \x1b[39mWatermelon',
  '  \x1b[38;5;12m12. \x1b[39mKiwi',
  '',
  '\x1b[2m  Worked for 18s • 01:28',
  '',
  ...tail,
].join('\n');
// A two-line prompt: Codex joins the lines with a space before cutting.
const twoLine = 'Probe two, first line is short.\nThen list fifteen vegetable names, one per line, nothing else.';
const joined = [
  '\x1b[1;2m› \x1b[0mProbe two, first line is short. Then list fifteen vegeta…',
  '',
  '  \x1b[38;5;12m1. \x1b[39mCarrot',
  ...tail,
].join('\n');
// Wide characters: 28 of them, the mark and the "…" make 59 cells.
const zhPrompt = '第三个探针：这是一条很长的中文提示，用来观察窄窗口里宽字符怎样截断。请列出十五种常见的蔬菜名称，每行一个，不要写别的内容。';
const zh = ['\x1b[1;2m› \x1b[0m第三个探针：这是一条很长的中文提示，用来观察窄窗口里宽字…', '  番茄', '  黄瓜', ...tail].join('\n');

test('the cut row of a real capture is lifted out with the full prompt', () => {
  const r = splitCodexPinned(idle, 'Codex', ['an older prompt', probe], COLS);
  expect(r?.prompt).toBe(probe);
  expect(r?.text.split('\n')[0]).toContain('Raspberry');
});

test('the rest of the capture is kept byte for byte, colours included', () => {
  expect(splitCodexPinned(idle, 'Codex', [probe], COLS)?.text).toBe(idle.split('\n').slice(1).join('\n'));
});

test('a prompt over several lines matches the row Codex joined them into', () => {
  expect(splitCodexPinned(joined, 'Codex', [twoLine], COLS)?.prompt).toBe(twoLine);
});

test('wide characters are measured in cells, as Codex cut them', () => {
  expect(splitCodexPinned(zh, 'Codex', [zhPrompt], COLS)?.prompt).toBe(zhPrompt);
  // A wide character that did not fit leaves one more cell empty: still the edge.
  expect(splitCodexPinned(zh, 'Codex', [zhPrompt], COLS + 1)?.prompt).toBe(zhPrompt);
});

test('the same prompt sent twice is one prompt, not an ambiguity', () => {
  expect(splitCodexPinned(idle, 'Codex', [probe, 'something else', probe], COLS)?.prompt).toBe(probe);
});

test('hasCodexCutRow sees the shape whether or not a prompt explains it yet', () => {
  expect(hasCodexCutRow(idle, 'Codex', COLS)).toBe(true);
  expect(splitCodexPinned(idle, 'Codex', ['not this one'], COLS)).toBeNull();
  expect(hasCodexCutRow(idle, 'Claude Code', COLS)).toBe(false);
  expect(hasCodexCutRow(idle.replace('ple…', 'ple'), 'Codex', COLS)).toBe(false);
  expect(hasCodexCutRow(idle, 'Codex', undefined)).toBe(false);
});

describe('several prompts open the same way (review L1)', () => {
  // A templated dispatch: what the row shows is the same in both.
  const a = probe + ' First task: count the fruit.';
  const b = probe + ' Second task: sort the fruit.';

  test('two different prompts that both explain the row: the screen stays as it is', () => {
    expect(splitCodexPinned(idle, 'Codex', [a, b], COLS)).toBeNull();
    expect(hasCodexCutRow(idle, 'Codex', COLS)).toBe(true);
  });

  test('a prompt the row does not fit is no rival', () => {
    expect(splitCodexPinned(idle, 'Codex', [a, 'This is a throwaway note.'], COLS)?.prompt).toBe(a);
  });
});

describe("a row that ends in the user's own \"…\" is not a cut (review L2)", () => {
  // Composer below, as on a real screen, so only the row itself can tell.
  const own = '先看一下 deploy 日志里那段报错…';
  const at0 = (row: string, next = '• Ran ls') => [row, next, ...tail].join('\n');

  test('short of the right edge: it ended there, nothing was cut', () => {
    expect(splitCodexPinned(at0('\x1b[1;2m› \x1b[0m' + own), 'Codex', [own], COLS)).toBeNull();
    expect(hasCodexCutRow(at0('› ' + own), 'Codex', COLS)).toBe(false);
  });

  test('at the edge, but the prompt is no longer than the row: nothing was cut', () => {
    // A pane exactly as wide as the row: "› " 2, eleven wide characters 22, " deploy " 8, "…" 1.
    const w = 33;
    expect(splitCodexPinned(at0('› ' + own), 'Codex', [own], w)).toBeNull();
  });

  test('at the edge, and the next row carries on with the same prompt: a history message wrapped', () => {
    const p = '先看一下 deploy 日志里那段报错…然后告诉我是哪一次提交引入的，再列出要回滚的文件。';
    const w = 33;
    const wrapped = at0('› 先看一下 deploy 日志里那段报错…', '  然后告诉我是哪一次提交引入的，');
    expect(splitCodexPinned(wrapped, 'Codex', [p], w)).toBeNull();
    // The same row over unrelated output is the cut row.
    expect(splitCodexPinned(at0('› 先看一下 deploy 日志里那段报错…'), 'Codex', [p], w)?.prompt).toBe(p);
  });
});

describe('anything else leaves the capture untouched', () => {
  test.each([
    ['not Codex', idle, 'Claude Code', [probe], COLS],
    ['the pane width is unknown (a Mac server before v1.0.53)', idle, 'Codex', [probe], undefined],
    ['the row is wider than the pane', idle, 'Codex', [probe], COLS - 2],
    ['the row ends well short of the edge', idle, 'Codex', [probe], COLS + 3],
    ['no prompts known', idle, 'Codex', [], COLS],
    ['the row is not cut', idle.replace('ple…', 'ple'), 'Codex', [probe], COLS],
    ['the row is not a prompt', idle.replace('› ', '• '), 'Codex', [probe], COLS],
    ['no recent prompt matches', idle, 'Codex', ['something else entirely'], COLS],
    ['too little text to identify', ['› 你是…', 'x', '› Ask'].join('\n'), 'Codex', ['你是独立只读诊断 worker'], 7],
    ['no composer below (row 0 could be the composer)', idle.replace('\x1b[0;1m›\x1b[0m ', ''), 'Codex', [probe], COLS],
    ['a prompt wrapped over two rows, cut on the second', ['› This is a throwaway rendering probe for gtmux tests;', '  please do not edit any files. First run the shell co…', '• out', '› Ask'].join('\n'), 'Codex', [probe], COLS],
    ['an empty screen', '', 'Codex', [probe], COLS],
  ])('%s', (_name, text, agent, prompts, cols) => {
    expect(splitCodexPinned(text as string, agent as string, prompts as string[], cols as number | undefined)).toBeNull();
  });
});

test('a history prompt at the top that is not cut is not mistaken for the pinned one', () => {
  const history = ['› This is a throwaway rendering probe for gtmux tests;', '  please do not edit any files.', '• Ran ls', '› Ask Codex'].join('\n');
  expect(splitCodexPinned(history, 'Codex', [probe], COLS)).toBeNull();
});

test('a symbol paneLines gives a variation selector still matches', () => {
  const warn = '⚠ 先别合并：核实 HQ 自轮换会话归属，再决定是否回滚上一次发布。';
  const row = '› ⚠ 先别合并：核实 HQ 自轮换会话归属…';
  // 37 cells: "› " 2, "⚠ " 2, "先别合并：核实" 14, " HQ " 4, "自轮换会话归属" 14, "…" 1. The
  // selector paneLines adds after ⚠ takes no cell.
  const s = [row, 'out', '› Ask Codex'].join('\n');
  expect(splitCodexPinned(s, 'Codex', [warn], 38)?.prompt).toBe(warn);
});

// Emoji tmux draws two cells wide count as two, so a prompt opening with them still reaches
// the right edge by the matcher's arithmetic (review L5 of #1285: five ✅ read as five cells
// short, and the bar did not appear).
test('a prompt opening with wide emoji is still recognised', () => {
  const p = '✅✅✅✅✅ 全部检查已通过，下一步把发布说明写进看板，再通知 HQ 复核这一版的改动范围。';
  // "› " 2 + five ✅ 10 + " " 1 + 23 wide characters 46 + "…" 1 = 60 cells: a 61-column pane.
  const row = '› ✅✅✅✅✅ 全部检查已通过，下一步把发布说明写进看板，再通…';
  expect(splitCodexPinned([row, '• Ran ls', ...tail].join('\n'), 'Codex', [p], 61)?.prompt).toBe(p);
});

// The browser mirror carries a JavaScript copy of this matcher (internal/server/web/app.js).
// Both run these cases; a change to one that the other does not make fails one of them.
describe('the cases shared with the browser mirror', () => {
  const {cases} = require('./codexPinnedCases.json') as {
    cases: {name: string; text: string; agent: string; prompts: string[]; cols: number | null; want: string | null; rest: string | null}[];
  };
  test.each(cases.map(c => [c.name, c]))('%s', (_name, c) => {
    const r = splitCodexPinned(c.text, c.agent, c.prompts, c.cols ?? undefined);
    expect(r?.prompt ?? null).toBe(c.want);
    if (c.want !== null) expect(r?.text).toBe(c.rest);
  });
});
