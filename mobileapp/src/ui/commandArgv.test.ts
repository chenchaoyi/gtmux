import { codeParts, formatCode } from './codeFormatting';

const py = 'import json\nx={"count":1};print(x)\n';
const shell = "python3 - <<'PY'\n" + py + 'PY\nrg --files src';
test('ARGV-001 shell argv decodes once and keeps complete call before command and typed script', () => {
  const source = JSON.stringify(['/bin/bash', '-c', shell]).replace(
    /</g,
    '\\u003c',
  );
  for (const hint of ['json', 'shell', 'command']) {
    const parts = codeParts(source, hint);
    expect(parts.map(p => p.hint)).toEqual([hint, 'shell', 'python']);
    expect(parts.map(p => p.source)).toEqual([source, shell, py]);
    expect(new Set(parts.map(p => p.id)).size).toBe(3);
    expect(formatCode(parts[2].source, parts[2].hint)).toContain(
      'x = {"count": 1}\nprint(x)',
    );
  }
});
test('ARGV-001 cmd fields and JS literal arrays retain command expansion order', () => {
  for (const source of [
    JSON.stringify({ command: ['/bin/zsh', '-lc', shell] }),
    `await tools.exec_command({command:['/bin/sh','-c',${JSON.stringify(
      shell,
    )}]});`,
  ]) {
    const parts = codeParts(source, 'javascript');
    expect(parts.map(p => p.source)).toEqual([source, shell, py]);
  }
  const source = JSON.stringify([
    { cmd: ['python3', '-u', '-c', py] },
    { command: ['node', '--eval', 'const x={n:1};console.log(x);'] },
  ]);
  expect(codeParts(source, 'json').map(p => p.hint)).toEqual([
    'json',
    'python',
    'javascript',
  ]);
});
test('ARGV-001 ordinary argv and dynamic or incomplete wrappers stay original without execution', () => {
  for (const source of [
    JSON.stringify(['echo', 'import json; x=1']),
    JSON.stringify(['python3', 'script.py']),
    JSON.stringify({ args: ['/bin/sh', '-c', shell] }),
    `tools.exec_command({cmd:['bash','-c',danger()]})`,
    `tools.exec_command({cmd:['bash','-c',...args]})`,
    '["/bin/bash","-c",',
    JSON.stringify(['python3', '-c']),
    JSON.stringify(['node', 'file.js']),
    JSON.stringify(['/bin/bash', '-x', shell]),
  ]) {
    expect(codeParts(source, 'json')).toEqual([
      { id: 'original', source, hint: 'json', path: '' },
    ]);
  }
  const source = JSON.stringify([
    'bash',
    '-c',
    "node - <<'JS'\nconst v={n:1};\nJS\nbash <<'SH'\necho python\nSH",
  ]);
  expect(codeParts(source, 'json').map(p => p.hint)).toEqual([
    'json',
    'shell',
    'javascript',
    'shell',
  ]);
  const broken = JSON.stringify([
    'bash',
    '-c',
    "python3 - <<'PY'\nmissing end",
  ]);
  expect(codeParts(broken, 'json').map(p => p.hint)).toEqual(['json', 'shell']);
  const large = JSON.stringify(['python3', '-c', 'x=1\n'.repeat(70000)]);
  expect(codeParts(large, 'json')).toHaveLength(1);
});
