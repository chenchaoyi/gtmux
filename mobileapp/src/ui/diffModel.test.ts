import { diffFiles, diffPresentation } from './diffModel';
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
