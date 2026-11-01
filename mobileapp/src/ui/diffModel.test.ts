import { diffDisplayRows, diffFiles, diffPresentation } from './diffModel';
const source =
  'diff --git a/a.go b/a.go\nindex x..y\n--- a/a.go\n+++ b/a.go\n@@ -2,2 +2,3 @@\n old\n-gone\n+new\n+extra\n@@ -8 +9 @@\n-before\n+after\ndiff --git a/b.txt b/b.txt\n--- a/b.txt\n+++ b/b.txt\n@@ -1 +1 @@\n-x\n+y';
test('files and hunks count only real additions and keep line numbers', () => {
  const files = diffFiles(source);
  expect(files.map(f => [f.path, f.added, f.removed])).toEqual([
    ['a.go', 3, 2],
    ['b.txt', 1, 1],
  ]);
  const code = files[0].rows.filter(r => r.meta.kind === 'code');
  expect(code[0].meta).toMatchObject({ old: 2, next: 2 });
  expect(code[2].meta).toMatchObject({ next: 3 });
  expect(code[2].meta.old).toBeUndefined();
  expect(files[0].raw).toContain('+after');
});
test('first hunk opens initially, each file and hunk folds independently without dropping source', () => {
  const first = diffPresentation(source);
  expect(first.lines).toContain('+extra');
  expect(first.lines).not.toContain('+after');
  expect(first.lines).not.toContain('+y');
  const hunk = first.meta.filter(m => m.kind === 'hunk')[1];
  const open = diffPresentation(source, { [hunk.key]: false });
  expect(open.lines).toContain('+after');
  expect(open.lines).not.toContain('+y');
  expect(
    diffPresentation(source, { [first.files[0].key]: true }).lines,
  ).not.toContain('+extra');
  expect(first.added).toBe(4);
  expect(first.removed).toBe(3);
});
test('plain and binary diffs remain readable', () => {
  expect(diffPresentation('+plain').lines).toEqual(['+plain']);
  const binary = diffFiles(
    'diff --git a/a.png b/a.png\nBinary files a/a.png and b/a.png differ',
  );
  expect(binary[0].rows[0].meta.kind).toBe('meta');
  expect(binary[0].added).toBe(0);
});

test('Git octal-quoted UTF-8 and literal percent filenames remain recognizable', () => {
  const text = String.raw`diff --git "a/\346\226\207\344\273\266.go" "b/\346\226\207\344\273\266.go"
--- "a/\346\226\207\344\273\266.go"
+++ "b/\346\226\207\344\273\266.go"
@@ -1 +1 @@
-old
+new`;
  expect(diffFiles(text)[0].path).toBe('文件.go');
  expect(
    diffFiles('diff --git "a/100% ready.go" "b/100% ready.go"')[0].path,
  ).toBe('100% ready.go');
});


test('display rows preserve recorded whitespace and original patch numbers', () => {
  const raw = 'diff --git a/a.js b/a.js\n--- a/a.js\n+++ b/a.js\n@@ -7 +7 @@\n-const old=1;\n+const current={value:1};';
  const presentation = diffPresentation(raw);
  const shown = diffDisplayRows(presentation);
  const start = shown.lines.indexOf('+const current={value:1};');
  expect(start).toBeGreaterThan(-1);
  expect(shown.meta[start].next).toBe(7);
  expect(shown.lines.filter((_, i) => shown.meta[i].kind === 'code')).toEqual([
    '-const old=1;', '+const current={value:1};',
  ]);
  expect(presentation.files[0].raw).toBe(raw);
});
test('complete long lines are split without losing Unicode or the final character', () => {
  const long = 'x'.repeat(1023) + '🙂鲸'.repeat(1500) + 'END';
  const raw = 'diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n+' + long;
  const shown = diffDisplayRows(diffPresentation(raw));
  const code = shown.lines.filter((_, i) => shown.meta[i].kind === 'code');
  expect(code.map(line => line.slice(1)).join('')).toBe(long);
  expect(code.every(line => line.length <= 1025)).toBe(true);
  expect(code.at(-1)).toContain('END');
  expect(shown.shortened).toBe(false);
});
