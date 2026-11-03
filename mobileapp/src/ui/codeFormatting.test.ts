import { codeParts, formatCode, commandOutline } from './codeFormatting';

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

test('Python compact formatting separates statements and preserves suite ownership', () => {
  const source =
    'from pathlib import Path\ns=Path("/example");r={"name":"鲸🙂"};print(s,r)\nif ready: a=1;b=2\n';
  const result = formatCode(source, 'python');
  expect(result).toContain(
    's = Path("/example")\nr = {"name": "鲸🙂"}\nprint(s, r)',
  );
  expect(result).toContain('if ready:\n    a = 1\n    b = 2');
  expect(formatCode(result, 'python')).toBe(result);
});

test('long Python dictionaries and argument lists use compact, syntax-preserving indentation', () => {
  const source =
    'data={"first":"' +
    'a'.repeat(50) +
    '","second":"' +
    'b'.repeat(50) +
    '"};print(data)\n';
  const output = formatCode(source, 'python');
  expect(output).toContain('data = {\n    "first":');
  expect(output).toContain('\n    "second":');
  expect(output).toContain('\n}\nprint(data)');
  expect(formatCode(output, 'python')).toBe(output);
});

test.each([
  'def fn():\n    x=1;y=2\n    return x+y\n',
  'value=f"{some_value=}";raw=r"a;b\\n";text="""a;b\n  exact"""\n',
  'items=[x for x in range(4) if x>0];data={"a:b":items[1:]}\n',
  'if ready:\n\tfirst=1;second=2\n',
])('Python formatting is stable without altering literals: %s', source => {
  const output = formatCode(source, 'python');
  expect(formatCode(output, 'python')).toBe(output);
  if (source.includes('some_value')) {
    expect(output).toContain('f"{some_value=}"');
    expect(output).toContain('r"a;b\\n"');
    expect(output).toContain('"""a;b\n  exact"""');
  }
});

test.each(['if ready:\n', 'print("broken', 'value = f"{broken"'])(
  'incomplete Python remains verbatim',
  source => {
    expect(formatCode(source, 'python')).toBe(source);
  },
);

const python = 'from pathlib import Path\ns=Path("/example");print(s)\n';
const shell = "set -e\npython3 - <<'PY'\n" + python + 'PY\ngit status';
test('static JavaScript command strings decode into Python while preserving the complete wrapper', () => {
  const source =
    'await tools.exec_command({cmd:' + JSON.stringify(shell) + '});';
  const parts = codeParts(source, 'javascript');
  expect(parts.map(part => part.hint)).toEqual([
    'javascript',
    'shell',
    'python',
  ]);
  expect(parts[0].source).toBe(source);
  expect(parts[1].source).toBe(shell);
  expect(parts[2].source).toBe(python);
  expect(parts[1].path).toBe('cmd 1');
  expect(formatCode(parts[2].source, parts[2].hint)).toContain('s = Path');
});

test('JSON arrays and multiple heredocs expose separate, uniquely keyed recorded scripts', () => {
  const source = JSON.stringify([{ cmd: shell }, { command: shell }]);
  const parts = codeParts(source, 'json');
  expect(
    parts.filter(part => part.hint === 'python').map(part => part.source),
  ).toEqual([python, python]);
  expect(new Set(parts.map(part => part.id)).size).toBe(parts.length);
  expect(codeParts(shell, 'shell')[0].source).toBe(shell);
  expect(
    codeParts("python3 - <<'PY'\nmissing terminator", 'shell'),
  ).toHaveLength(1);
});

test('dynamic expressions, interpolated templates, comments and incomplete wrappers are never evaluated', () => {
  for (const source of [
    'tools.exec_command({cmd:globalThis.__neverRun=true})',
    'tools.exec_command({cmd:`python3 ${danger()}`})',
    '// cmd: "python3 -"',
    'const broken={cmd:"python3',
  ]) {
    expect(codeParts(source, 'javascript')).toEqual([
      { id: 'original', source, hint: 'javascript', path: '' },
    ]);
  }
  expect((globalThis as Record<string, unknown>).__neverRun).toBeUndefined();
});

test('command is Shell unless an explicit interpreter identifies an embedded language', () => {
  for (const cmd of [
    'git status\nrg -n "python" src',
    'echo "import json; x=1"',
    'ruby script.rb',
    'node ./index.js',
  ]) {
    const input = JSON.stringify({ cmd });
    expect(codeParts(input, 'json').map(part => part.hint)).toEqual([
      'json',
      'shell',
    ]);
    expect(codeParts(input, 'json')[1].source).toBe(cmd);
  }
  const cmd = "node - <<'JS'\nconst value={count:1};console.log(value);\nJS";
  expect(
    codeParts(JSON.stringify({ cmd }), 'json').map(part => part.hint),
  ).toEqual(['json', 'shell', 'javascript']);
  const bash = "bash <<'SH'\necho 'import json'\nSH";
  expect(
    codeParts(JSON.stringify({ cmd: bash }), 'json').every(
      part => part.hint !== 'python',
    ),
  ).toBe(true);
});

test('full call leads outer commands and their scripts in recorded expansion order', () => {
  const first =
    "python3 - <<'PY'\nprint(1)\nPY\nnode - <<'JS'\nconsole.log(2)\nJS";
  const second = "bash <<'SH'\necho done\nSH";
  const source = JSON.stringify([{ cmd: first }, { command: second }]);
  const parts = codeParts(source, 'json');
  expect(parts.map(part => part.hint)).toEqual([
    'json',
    'shell',
    'python',
    'javascript',
    'shell',
    'shell',
  ]);
  expect(parts.map(part => part.path)).toEqual([
    '',
    'cmd 1',
    'cmd 1',
    'cmd 1',
    'command 2',
    'command 2',
  ]);
  expect(parts[0].source).toBe(source);
  expect(parts[1].source).toBe(first);
  expect(parts[2].source).toBe('print(1)\n');
  expect(parts[3].source).toBe('console.log(2)\n');
  expect(parts[4].source).toBe(second);
  expect(codeParts(first, 'shell').map(part => part.hint)).toEqual([
    'shell',
    'python',
    'javascript',
  ]);
});

test('command outline describes static submitted inputs without inventing dynamic calls or outcomes', () => {
 const source = 'if (ready) await tools.exec_command({cmd:"cat report.md"}); await tools.exec_command({cmd:"rg --files src"}); await tools.exec_command({cmd:"rg pattern src"}); await tools.exec_command({cmd:dynamic});';
 const result=commandOutline(codeParts(source,'javascript'));
 expect(result).toEqual([{partID:'cmd 1',operation:'read',preview:'cat report.md'},{partID:'cmd 2',operation:'list',preview:'rg --files src'},{partID:'cmd 3',operation:'search',preview:'rg pattern src'}]);
 expect(commandOutline(codeParts('await tools.exec_command({cmd:`echo ${dynamic}`})','javascript'))).toEqual([]);
 expect(commandOutline(codeParts('const broken = {','javascript'))).toEqual([]);
});
