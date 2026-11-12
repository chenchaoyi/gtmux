import {diffContext, diffFiles, diffTree, diffTreePresentation} from './diffModel';

const patch = '# worktree "/tmp/worktree with spaces"\n# branch feature/review\n' +
  'diff --git a/docs/a.md b/docs/a.md\n--- a/docs/a.md\n+++ b/docs/a.md\n@@ -1 +1 @@\n-old\n+new\n' +
  'diff --git a/src/a.ts b/src/a.ts\n--- a/src/a.ts\n+++ b/src/a.ts\n@@ -7 +7 @@\n-before\n+after';

test('DTV001 directories start closed and preserve exact code and totals on expansion', () => {
  const closed = diffTreePresentation(patch);
  expect(closed.lines).toEqual(['docs', 'src']);
  expect(closed.meta.every(m => m.kind === 'directory' && m.open === false)).toBe(true);
  expect(closed.added).toBe(2); expect(closed.removed).toBe(2);
  expect(closed.meta.map(m => [m.path, m.fileCount, m.added, m.removed])).toEqual([['docs',1,1,1],['src',1,1,1]]);
  const file = closed.files[0], hunk = file.rows.find(r => r.meta.kind === 'hunk')!;
  const expanded = diffTreePresentation(patch, {'directory:docs':false,[file.key]:false,[hunk.meta.key]:false});
  expect(expanded.lines).toEqual(['docs','docs/a.md','@@ -1 +1 @@','-old','+new','src']);
  expect(expanded.meta[4].next).toBe(1);
  expect(expanded.files[0].raw).toContain('+new');
  expect(expanded.files[0].raw).not.toContain('# worktree');
});

test('DTV001 context stays outside hunks and untracked names never count as code additions', () => {
  const text = patch + '\n# untracked:\n+ a/new.md\n+ "\\346\\226\\207\\344\\273\\266.txt"\n\n… (diff truncated)\n';
  expect(diffContext(text)).toEqual({worktree:'/tmp/worktree with spaces',branch:'feature/review',truncated:true});
  const result = diffTreePresentation(text);
  expect(result.files).toHaveLength(4);
  expect(result.files.slice(2).map(f => [f.path,f.untracked,f.added])).toEqual([['a/new.md',true,0],['文件.txt',true,0]]);
  expect(result.added).toBe(2); expect(result.removed).toBe(2);
  expect(result.files[1].raw).not.toContain('untracked');
  expect(diffContext(patch + '\n+# branch fake')).toMatchObject({branch:'feature/review'});
  expect(diffContext('+legacy')).toEqual({worktree:'',branch:'',truncated:false});
  expect(diffTreePresentation('# worktree "/tmp/clean"\n# branch main\n').lines.every(line => !line.trim())).toBe(true);
  expect(diffTreePresentation('+legacy').lines).toEqual(['+legacy']);
});

test('DTV001 prototype-like and deep folder names stay inert with bounded hierarchy', () => {
  const paths = ['__proto__/constructor/a.txt', Array(200).fill('nested').join('/')+'/a.txt'];
  const files = diffFiles(paths.map(path => `diff --git a/${path} b/${path}\nBinary files differ`).join('\n'));
  const tree = diffTree(files);
  const depth = (nodes: ReturnType<typeof diffTree>): number => nodes.length ? 1+Math.max(...nodes.map(n => depth(n.children))) : 0;
  expect(depth(tree)).toBeLessThanOrEqual(25);
  expect(tree[0].name).toBe('__proto__');
  expect(({} as Record<string, unknown>).fileCount).toBeUndefined();
  expect(files.map(f => f.path)).toEqual(paths);
});
