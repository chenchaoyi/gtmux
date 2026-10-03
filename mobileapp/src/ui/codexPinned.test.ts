import {splitCodexPinned} from './codexPinned';

const prompt = '你是独立只读诊断 worker，不是 HQ，不接管监管，不派发其他 agent。 任务：核实 HQ 自轮换会话归属，读 hook 和 resume 记录。';
// The shape reported on 2026-10-03: the pinned row cut with "…", tool output, the composer.
const screen = [
  '\x1b[1m› \x1b[0m你是独立只读诊断 worker，不是 HQ，不接管监管，不派发其他 agent。 任务：核实…',
  '        1    {"agent":"codex","sessionId":"01a0ffc1-2208"}',
  '',
  '• Explored',
  '  └ Search resume.Save in internal',
  '',
  '› Ask Codex to do anything',
  '',
  '  GPT-6.1-Sol high · ~/gtmux',
].join('\n');

test('a truncated pinned prompt is lifted out with its full text', () => {
  const r = splitCodexPinned(screen, 'Codex', ['older prompt', prompt]);
  expect(r?.prompt).toBe(prompt);
  expect(r?.text.split('\n')[0]).toContain('{"agent":"codex"');
  expect(r?.text.split('\n')).toHaveLength(screen.split('\n').length - 1);
});

test('the rest of the capture is kept byte for byte, colours included', () => {
  const r = splitCodexPinned(screen, 'Codex', [prompt]);
  expect(r?.text).toBe(screen.split('\n').slice(1).join('\n'));
});

test('a prompt wrapped over two pinned rows still matches', () => {
  const two = ['› 你是独立只读诊断 worker，不是 HQ，不接管监管，', '  不派发其他 agent。 任务：核实…', 'out', '› Ask Codex'].join('\n');
  expect(splitCodexPinned(two, 'Codex', [prompt])?.text).toBe('out\n› Ask Codex');
});

test('the newest matching prompt wins, and a just-sent prompt counts', () => {
  const again = prompt + ' 第二次';
  expect(splitCodexPinned(screen, 'codex', [prompt, again])?.prompt).toBe(again);
});

describe('anything else leaves the capture untouched', () => {
  test.each([
    ['not Codex', screen, 'Claude Code', [prompt]],
    ['no prompts known', screen, 'Codex', []],
    ['the row is not cut', screen.replace('核实…', '核实'), 'Codex', [prompt]],
    ['the row is not a prompt', screen.replace('› ', '• '), 'Codex', [prompt]],
    ['no recent prompt matches', screen, 'Codex', ['something else entirely']],
    ['too little text to identify', ['› 你是…', 'x', '› Ask'].join('\n'), 'Codex', [prompt]],
    ['no composer below (row 0 could be the composer)', screen.replace('› Ask Codex to do anything', 'Ask Codex'), 'Codex', [prompt]],
    ['the cut comes after a non-continuation row', ['› 你是独立只读诊断 worker', 'not indented…', '› Ask'].join('\n'), 'Codex', [prompt]],
    ['the pinned block is longer than three rows', ['› 你是独立只读', '  诊断 worker，', '  不是 HQ，', '  不接管…', '› Ask'].join('\n'), 'Codex', [prompt]],
    ['an empty screen', '', 'Codex', [prompt]],
  ])('%s', (_name, text, agent, prompts) => {
    expect(splitCodexPinned(text as string, agent as string, prompts as string[])).toBeNull();
  });
});

test('a history prompt at the top that is not cut is not mistaken for the pinned one', () => {
  const history = ['› 你是独立只读诊断 worker，不是 HQ，不接管监管，', '  不派发其他 agent。', '• Ran ls', '› Ask Codex'].join('\n');
  expect(splitCodexPinned(history, 'Codex', [prompt])).toBeNull();
});

test('a symbol paneLines gives a variation selector still matches', () => {
  const warn = '⚠ 先别合并：核实 HQ 自轮换会话归属，再决定是否回滚上一次发布。';
  const s = ['› ⚠ 先别合并：核实 HQ 自轮换会话归属…', 'out', '› Ask Codex'].join('\n');
  expect(splitCodexPinned(s, 'Codex', [warn])?.prompt).toBe(warn);
});
