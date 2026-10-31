import {
  codeLanguage,
  codeTokens,
  darkCodeBackground,
  syntaxColors,
  toolCodeHint,
} from './codeReading';

test.each([
  [
    'const rs = await Promise.allSettled([tools.exec_command({cmd: "pwd"})]);',
    '',
    'javascript',
  ],
  ['{"duration_ms": 1000}', 'parameters', 'json'],
  ['cd /project && git status', '', 'shell'],
  ['def answer():\n    return 42', 'py', 'python'],
  ['name: example', 'yaml', 'yaml'],
  ['A description with no code', '', 'text'],
])(
  'recognizes the recorded language without parsing or executing %s',
  (source, hint, expected) => {
    expect(codeLanguage(source, hint)).toBe(expected);
  },
);
test.each(['javascript', 'json', 'shell', 'python', 'yaml', 'text'] as const)(
  'preserves every source character for %s',
  language => {
    const source =
      '#* comment\nconst x = 1 + 2e-3; // note\n{"hello": "<script>\\n文字🙂", "done": true}\n"unfinished';
    const tokens = codeTokens(source, language);
    expect(tokens.map(t => t.text).join('')).toBe(source);
    expect(tokens.every(t => t.text.length > 0)).toBe(true);
  },
);
test('distinguishes keys, strings, keywords and arithmetic numbers', () => {
  expect(codeTokens('{"name":"value", "size":12}', 'json')).toEqual(
    expect.arrayContaining([
      { text: '"name"', kind: 'property' },
      { text: '"value"', kind: 'string' },
      { text: '12', kind: 'number' },
    ]),
  );
  expect(
    codeTokens('const x=1+2;', 'javascript')
      .filter(t => t.kind === 'number')
      .map(t => t.text),
  ).toEqual(['1', '2']);
  expect(codeTokens('#* note\nnext', 'shell')[0]).toEqual({
    text: '#* note',
    kind: 'comment',
  });
});
test('budgets token work while retaining the complete recorded payload and Unicode boundary', () => {
  const source = 'const x=1; '.repeat(10000);
  const tokens = codeTokens(source, 'javascript');
  expect(tokens.length).toBeLessThan(1604);
  expect(tokens.map(t => t.text).join('')).toBe(source);
  const emoji = ' '.repeat(31999) + '🙂' + ' const a=2;';
  expect(
    codeTokens(emoji, 'javascript')
      .map(t => t.text)
      .join(''),
  ).toBe(emoji);
});
const luminance = (hex: string) => {
  const rgb = hex
    .slice(1)
    .match(/../g)!
    .map(h => parseInt(h, 16) / 255)
    .map(n => (n <= 0.04045 ? n / 12.92 : ((n + 0.055) / 1.055) ** 2.4));
  return rgb[0] * 0.2126 + rgb[1] * 0.7152 + rgb[2] * 0.0722;
};
test.each([
  ['light', '#F2F2F7'],
  ['dark', '#2A2A2E'],
] as const)(
  '%s syntax tokens meet normal-text contrast on the raised surface',
  (mode, bg) => {
    for (const color of Object.values(syntaxColors[mode])) {
      const a = luminance(color),
        b = luminance(bg);
      expect(
        (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05),
      ).toBeGreaterThanOrEqual(4.5);
    }
    expect(darkCodeBackground(bg)).toBe(mode === 'dark');
  },
);

test('translucent code surfaces use their surrounding text palette', () => {
  expect(darkCodeBackground('rgba(255,255,255,0.08)', '#E5E5EA')).toBe(true);
  expect(darkCodeBackground('rgba(0,0,0,0.04)', '#1D1D1F')).toBe(false);
  expect(darkCodeBackground('#fff')).toBe(false);
});

test('tool metadata disambiguates command scripts and code with long leading comments', () => {
  expect(toolCodeHint('exec_command', 'command', 'command')).toBe('shell');
  expect(
    codeLanguage(
      '// comment'.repeat(100) + '\nconst value = 1;',
      toolCodeHint('exec', 'code', 'code'),
    ),
  ).toBe('javascript');
  expect(toolCodeHint('run_python', 'code', 'code')).toBe('python');
  expect(toolCodeHint('wait', 'parameters')).toBe('json');
  expect(toolCodeHint('future_tool')).toBe('future_tool');
});

test('Go and other source languages keep recognizable labels and syntax', () => {
  expect(codeLanguage('package server\nfunc test() {}')).toBe('go');
  expect(codeTokens('package server\nfunc test() {}', 'go')).toEqual(
    expect.arrayContaining([
      { text: 'package', kind: 'keyword' },
      { text: 'func', kind: 'keyword' },
      { text: 'test', kind: 'property' },
    ]),
  );
});
