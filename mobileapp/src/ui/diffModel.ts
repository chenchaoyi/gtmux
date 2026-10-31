// Unified-diff structure shared by native and browser readers. Only actual hunks
// contribute line counts; headers, binary changes and rename metadata remain visible.
export type DiffRowMeta = {
  kind: 'file' | 'hunk' | 'code' | 'meta';
  key: string;
  file?: string;
  hunk?: string;
  old?: number;
  next?: number;
  role?: 'add' | 'del' | 'context';
  path?: string;
  added?: number;
  removed?: number;
  open?: boolean;
};
export type DiffFile = {
  key: string;
  path: string;
  added: number;
  removed: number;
  raw: string;
  rows: Array<{ text: string; meta: DiffRowMeta }>;
};
function gitPath(value: string) {
  if (value.startsWith('"')) {
    try {
      return (JSON.parse(value) as string).replace(/^[ab]\//, '');
    } catch {
      return value;
    }
  }
  return value.replace(/^[ab]\//, '');
}
export function diffFiles(text: string): DiffFile[] {
  const files: DiffFile[] = [];
  let file: DiffFile | undefined;
  let hunk = '';
  let old = 0,
    next = 0,
    inHunk = false;
  for (const [i, line] of text.replace(/\n$/, '').split('\n').entries()) {
    if (line.startsWith('diff --git ')) {
      const match =
        /^diff --git (?:"(?:[^"\\]|\\.)*"|a\/.*?) ("(?:[^"\\]|\\.)*"|b\/.*)$/.exec(
          line,
        );
      file = {
        key: 'file:' + i,
        path: match ? gitPath(match[1]) : line.slice(11),
        added: 0,
        removed: 0,
        raw: line,
        rows: [],
      };
      files.push(file);
      hunk = '';
      inHunk = false;
      continue;
    }
    if (!file) continue;
    file.raw += '\n' + line;
    if (!inHunk && line.startsWith('+++ ') && line !== '+++ /dev/null')
      file.path = gitPath(line.slice(4));
    if (!inHunk && /^(?:index |--- |\+\+\+ )/.test(line)) continue;
    const header = /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@/.exec(line);
    if (header) {
      hunk = file.key + ':hunk:' + i;
      old = Number(header[1]);
      next = Number(header[3]);
      inHunk = true;
      file.rows.push({
        text: line,
        meta: { kind: 'hunk', key: hunk, file: file.key, hunk },
      });
      continue;
    }
    const role =
      inHunk && line[0] === '+'
        ? 'add'
        : inHunk && line[0] === '-'
        ? 'del'
        : inHunk && line[0] === ' '
        ? 'context'
        : undefined;
    const meta: DiffRowMeta = {
      kind: role ? 'code' : 'meta',
      key: 'line:' + i,
      file: file.key,
      hunk: hunk || undefined,
      role,
    };
    if (role !== 'add' && role) {
      meta.old = old++;
    }
    if (role !== 'del' && role) {
      meta.next = next++;
    }
    if (role === 'add') file.added++;
    if (role === 'del') file.removed++;
    file.rows.push({ text: line, meta });
  }
  for (const file of files) {
    let current: DiffRowMeta | undefined;
    for (const row of file.rows) {
      row.meta.path = file.path;
      if (row.meta.kind === 'hunk') {
        current = row.meta;
        current.added = 0;
        current.removed = 0;
      }
      if (current && row.meta.role === 'add') current.added!++;
      if (current && row.meta.role === 'del') current.removed!++;
    }
  }
  return files;
}
export function diffPresentation(
  text: string,
  folded: Record<string, boolean> = {},
) {
  const files = diffFiles(text),
    lines: string[] = [],
    meta: DiffRowMeta[] = [];
  if (!files.length) {
    return {
      files,
      lines: text.replace(/\n$/, '').split('\n'),
      meta: [] as DiffRowMeta[],
      added: 0,
      removed: 0,
    };
  }
  for (const [index, file] of files.entries()) {
    const open = !folded[file.key];
    lines.push(file.path);
    meta.push({
      kind: 'file',
      key: file.key,
      file: file.key,
      path: file.path,
      added: file.added,
      removed: file.removed,
      open,
    });
    if (!open) continue;
    let show = true,
      first = true;
    for (const row of file.rows) {
      if (row.meta.kind === 'hunk') {
        show =
          folded[row.meta.key] === undefined
            ? index === 0 && first
            : !folded[row.meta.key];
        first = false;
        lines.push(row.text);
        meta.push({ ...row.meta, open: show });
      } else if (!row.meta.hunk || show) {
        lines.push(row.text);
        meta.push(row.meta);
      }
    }
  }
  return {
    files,
    lines,
    meta,
    added: files.reduce((n, f) => n + f.added, 0),
    removed: files.reduce((n, f) => n + f.removed, 0),
  };
}

export function hunkRange(header: string): string {
  const match = /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@/.exec(header);
  if (!match) return header;
  const count = Number(match[4] ?? 1);
  const start = Number(count > 0 ? match[3] : match[1]);
  const span = count > 0 ? count : Number(match[2] ?? 1);
  return span > 1 ? `${start}–${start + span - 1}` : String(start);
}
