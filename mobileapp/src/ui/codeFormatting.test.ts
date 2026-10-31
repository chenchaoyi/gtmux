import { formatCode } from './codeFormatting';

test.each([
  [
    'javascript',
    'const a={name:"hello",size:2};for(let i=0;i<2;i++){text(a);}',
  ],
  ['typescript', 'const value: {name: string}={name:"hello"};'],
  ['json', '{"name":"hello","options":{"size":2,"on":true}}'],
  ['html', '<div><h1>Hello</h1><p>World</p></div>'],
  ['css', '.card{color:red;padding:2px;}'],
  ['yaml', 'items: [first,second,third]'],
])(
  'compact formatting for %s is stable and retains literal values',
  (hint, source) => {
    const formatted = formatCode(source, hint);
    expect(formatted).not.toBe(source);
    expect(formatted).toMatch(/hello|Hello|red|first/);
    expect(formatCode(formatted, hint)).toBe(formatted);
    expect(formatted).not.toMatch(/\n[ \t]*\n[ \t]*\n/);
  },
);
test.each([
  ['shell', "cat <<'EOF'\n  exact indentation\nEOF\n"],
  ['python', 'def fn():\n    return "unchanged"\n'],
  ['javascript', 'const broken = {'],
  ['text', '  preserve this\n'],
])('unsupported or incomplete %s stays verbatim', (hint, source) =>
  expect(formatCode(source, hint)).toBe(source),
);
test('a formatter never evaluates input or removes long recorded values', () => {
  const text = '鲸🙂'.repeat(10000);
  const source = `globalThis.__mustNotRun = true; text(${JSON.stringify(
    text,
  )});`;
  expect(formatCode(source, 'javascript')).toContain(text);
  expect((globalThis as Record<string, unknown>).__mustNotRun).toBeUndefined();
});

test('very large input bypasses parser work while retaining every character', () => {
  const source = 'const value={name:"鲸🙂"};\n'.repeat(20000);
  expect(formatCode(source, 'javascript')).toBe(source);
});
