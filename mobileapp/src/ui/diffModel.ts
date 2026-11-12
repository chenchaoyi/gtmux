// Unified-diff structure shared by native and browser readers. Only actual hunks
// contribute line counts; headers, binary changes and rename metadata remain visible.
export type DiffRowMeta = {
  kind: 'directory' | 'file' | 'hunk' | 'code' | 'meta';
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
  continuation?: boolean;
  depth?: number;
  fileCount?: number;
  untracked?: boolean;
};
export type DiffFile = {
  key: string;
  path: string;
  added: number;
  removed: number;
  untracked?: boolean;
  raw: string;
  rows: Array<{ text: string; meta: DiffRowMeta }>;
};
function gitPath(value: string, stripPrefix = true) {
  if (value.startsWith('"') && value.endsWith('"')) {
    try {
      value = JSON.parse(value) as string;
    } catch {
      // Git quotes non-ASCII filenames as UTF-8 octal bytes, not JSON escapes.
      const quoted = value.slice(1, -1);
      const escapes: Record<string, string> = {
        a: '\x07',
        b: '\b',
        f: '\f',
        n: '\n',
        r: '\r',
        t: '\t',
        v: '\v',
        '"': '"',
        '\\': '\\',
      };
      let encoded = '',
        cursor = 0;
      for (const match of quoted.matchAll(/\\([0-7]{1,3}|[abfnrtv"\\])/g)) {
        encoded += encodeURIComponent(quoted.slice(cursor, match.index));
        encoded += /^[0-7]/.test(match[1])
          ? '%' + parseInt(match[1], 8).toString(16).padStart(2, '0')
          : encodeURIComponent(escapes[match[1]]);
        cursor = match.index! + match[0].length;
      }
      encoded += encodeURIComponent(quoted.slice(cursor));
      try {
        value = decodeURIComponent(encoded);
      } catch {
        /* Preserve unfamiliar filename bytes verbatim. */
      }
    }
  }
  return stripPrefix ? value.replace(/^[ab]\//, '') : value;
}

export function diffFiles(text: string): DiffFile[] {
  const files: DiffFile[] = [];
  let file: DiffFile | undefined;
  let hunk = '';
  let old = 0,
    next = 0,
    inHunk = false;
  let untracked = false;
  for (const [i, line] of text.replace(/\n$/, '').split('\n').entries()) {
    if (line === '# untracked:') {untracked = true; file = undefined; continue;}
    if (line === '… (diff truncated)') {untracked = false; file = undefined; continue;}
    if (untracked) {
      if (line.startsWith('+ ')) files.push({key: 'untracked:' + i, path: gitPath(line.slice(2), false), added: 0, removed: 0, untracked: true, raw: line, rows: []});
      continue;
    }
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
  initiallyFolded = false,
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
    const open = folded[file.key] === undefined ? !initiallyFolded : !folded[file.key];
    lines.push(file.path);
    meta.push({
      kind: 'file',
      key: file.key,
      file: file.key,
      path: file.path,
      added: file.added,
      removed: file.removed,
      open,
      untracked: file.untracked,
    });
    if (!open) continue;
    let show = true,
      first = true;
    for (const row of file.rows) {
      if (row.meta.kind === 'hunk') {
        show =
          folded[row.meta.key] === undefined
            ? !initiallyFolded && index === 0 && first
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

/** Metadata emitted by the bounded Git capture, never inferred from a session title. */
export function diffContext(text: string) {
  let worktree = '', branch = '';
  for (const line of text.split('\n')) {
    if (line.startsWith('diff --git ') || line === '# untracked:') break;
    if (line.startsWith('# branch ')) branch = line.slice(9);
    if (line.startsWith('# worktree ')) {
      try {const value: unknown = JSON.parse(line.slice(11)); if (typeof value === 'string' && value.startsWith('/')) worktree = value;} catch { /* Older servers may not provide context. */ }
    }
  }
  return {worktree, branch, truncated: text.trimEnd().endsWith('… (diff truncated)')};
}

export type DiffTreeNode = {
  key: string; name: string; path: string; depth: number;
  added: number; removed: number; fileCount: number;
  file?: DiffFile; children: DiffTreeNode[];
};

/** Shared directory aggregation; Maps keep filenames such as __proto__ inert. */
export function diffTree(files: DiffFile[]): DiffTreeNode[] {
  const root: DiffTreeNode = {key: '', name: '', path: '', depth: -1, added: 0, removed: 0, fileCount: 0, children: []};
  const directories = new Map<string, DiffTreeNode>([['', root]]);
  for (const file of files) {
    const parts = file.path.split('/');
    const name = parts.pop() || file.path;
    // Preserve very deep paths as a compact final directory, with bounded stack depth.
    if (parts.length > 24) parts.splice(23, parts.length - 23, parts.slice(23).join('/'));
    let parent = root, path = '';
    for (const [depth, part] of parts.entries()) {
      path += (path ? '/' : '') + part;
      let node = directories.get(path);
      if (!node) {
        node = {key: 'directory:' + path, name: part, path, depth, added: 0, removed: 0, fileCount: 0, children: []};
        directories.set(path, node); parent.children.push(node);
      }
      node.added += file.added; node.removed += file.removed; node.fileCount++;
      parent = node;
    }
    parent.children.push({key: file.key, name, path: file.path, depth: parts.length, fileCount: 1, added: file.added, removed: file.removed, file, children: []});
  }
  const sort = (nodes: DiffTreeNode[]) => {
    nodes.sort((a, b) => Number(!!a.file) - Number(!!b.file) || (a.name < b.name ? -1 : a.name > b.name ? 1 : 0));
    nodes.forEach(node => sort(node.children));
  };
  sort(root.children);
  return root.children;
}

/** Native virtualized rows; source parsing and counts are identical to the browser tree. */
export function diffTreePresentation(text: string, folded: Record<string, boolean> = {}) {
  const parsed = diffPresentation(text, folded, true);
  const tree = diffTree(parsed.files);
  if (!parsed.files.length) {
    const context = diffContext(text);
    const lines = context.worktree || context.branch
      ? parsed.lines.filter(line => !line.startsWith('# worktree ') && !line.startsWith('# branch '))
      : parsed.lines;
    return {...parsed, tree, lines};
  }
  const byFile = new Map<string, number[]>();
  parsed.meta.forEach((meta, index) => {
    if (!meta.file) return;
    const indices = byFile.get(meta.file) || []; indices.push(index); byFile.set(meta.file, indices);
  });
  const lines: string[] = [], meta: DiffRowMeta[] = [];
  const visit = (nodes: DiffTreeNode[]) => {
    for (const node of nodes) {
      if (node.file) {
        for (const index of byFile.get(node.key) || []) {
          lines.push(parsed.lines[index]); meta.push({...parsed.meta[index], depth: node.depth});
        }
      } else {
        const open = folded[node.key] === false;
        lines.push(node.name); meta.push({kind: 'directory', key: node.key, path: node.path, depth: node.depth, fileCount: node.fileCount, added: node.added, removed: node.removed, open});
        if (open) visit(node.children);
      }
    }
  };
  visit(tree);
  return {...parsed, tree, lines, meta};
}

/** Split recorded patch lines into bounded native rows without reformatting source. */
export function diffDisplayRows(
  presentation: ReturnType<typeof diffPresentation>,
) {
  const lines: string[] = [], meta: DiffRowMeta[] = [];
  let columns = 0;
  presentation.lines.forEach((line, index) => {
    const original = presentation.meta[index];
    if (original?.kind !== 'code') {
      lines.push(line);
      meta.push(original);
      return;
    }
    const physical = line.slice(1);
    let part = 0;
    let offset = 0;
    do {
      let end = Math.min(offset + 1024, physical.length);
      if (end < physical.length && /[\uD800-\uDBFF]/.test(physical[end - 1])) end--;
      const source = physical.slice(offset, end);
      lines.push((part ? ' ' : line[0]) + source);
      meta.push(part ? {...original, key: original.key + ':part:' + part,
        old: undefined, next: undefined, continuation: true} : original);
      columns = Math.max(columns, Array.from(source).reduce((n, c) =>
        n + (c === '\t' ? 4 : c.codePointAt(0)! >= 0x1100 ? 2 : 1), 0));
      part++;
      offset = end;
    } while (offset < physical.length);
  });
  return {lines, meta, columns, shortened: false};
}
