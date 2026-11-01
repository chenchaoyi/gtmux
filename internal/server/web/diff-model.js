// Generated from mobileapp/src/ui/diffModel.ts; SHA256 20401f8c7eabf2a1bfaed4f9b09fea265ff712845d7f51f006dad3d73fe788a8
(function(root){var exports={};
"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.diffFiles = diffFiles;
exports.diffPresentation = diffPresentation;
exports.hunkRange = hunkRange;
exports.diffDisplayRows = diffDisplayRows;
function gitPath(value) {
    if (value.startsWith('"') && value.endsWith('"')) {
        try {
            value = JSON.parse(value);
        }
        catch {
            // Git quotes non-ASCII filenames as UTF-8 octal bytes, not JSON escapes.
            const quoted = value.slice(1, -1);
            const escapes = {
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
            let encoded = '', cursor = 0;
            for (const match of quoted.matchAll(/\\([0-7]{1,3}|[abfnrtv"\\])/g)) {
                encoded += encodeURIComponent(quoted.slice(cursor, match.index));
                encoded += /^[0-7]/.test(match[1])
                    ? '%' + parseInt(match[1], 8).toString(16).padStart(2, '0')
                    : encodeURIComponent(escapes[match[1]]);
                cursor = match.index + match[0].length;
            }
            encoded += encodeURIComponent(quoted.slice(cursor));
            try {
                value = decodeURIComponent(encoded);
            }
            catch {
                /* Preserve unfamiliar filename bytes verbatim. */
            }
        }
    }
    return value.replace(/^[ab]\//, '');
}
function diffFiles(text) {
    const files = [];
    let file;
    let hunk = '';
    let old = 0, next = 0, inHunk = false;
    for (const [i, line] of text.replace(/\n$/, '').split('\n').entries()) {
        if (line.startsWith('diff --git ')) {
            const match = /^diff --git (?:"(?:[^"\\]|\\.)*"|a\/.*?) ("(?:[^"\\]|\\.)*"|b\/.*)$/.exec(line);
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
        if (!file)
            continue;
        file.raw += '\n' + line;
        if (!inHunk && line.startsWith('+++ ') && line !== '+++ /dev/null')
            file.path = gitPath(line.slice(4));
        if (!inHunk && /^(?:index |--- |\+\+\+ )/.test(line))
            continue;
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
        const role = inHunk && line[0] === '+'
            ? 'add'
            : inHunk && line[0] === '-'
                ? 'del'
                : inHunk && line[0] === ' '
                    ? 'context'
                    : undefined;
        const meta = {
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
        if (role === 'add')
            file.added++;
        if (role === 'del')
            file.removed++;
        file.rows.push({ text: line, meta });
    }
    for (const file of files) {
        let current;
        for (const row of file.rows) {
            row.meta.path = file.path;
            if (row.meta.kind === 'hunk') {
                current = row.meta;
                current.added = 0;
                current.removed = 0;
            }
            if (current && row.meta.role === 'add')
                current.added++;
            if (current && row.meta.role === 'del')
                current.removed++;
        }
    }
    return files;
}
function diffPresentation(text, folded = {}) {
    const files = diffFiles(text), lines = [], meta = [];
    if (!files.length) {
        return {
            files,
            lines: text.replace(/\n$/, '').split('\n'),
            meta: [],
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
        if (!open)
            continue;
        let show = true, first = true;
        for (const row of file.rows) {
            if (row.meta.kind === 'hunk') {
                show =
                    folded[row.meta.key] === undefined
                        ? index === 0 && first
                        : !folded[row.meta.key];
                first = false;
                lines.push(row.text);
                meta.push({ ...row.meta, open: show });
            }
            else if (!row.meta.hunk || show) {
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
function hunkRange(header) {
    const match = /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@/.exec(header);
    if (!match)
        return header;
    const count = Number(match[4] ?? 1);
    const start = Number(count > 0 ? match[3] : match[1]);
    const span = count > 0 ? count : Number(match[2] ?? 1);
    return span > 1 ? `${start}–${start + span - 1}` : String(start);
}
/** Split recorded patch lines into bounded native rows without reformatting source. */
function diffDisplayRows(presentation) {
    const lines = [], meta = [];
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
            if (end < physical.length && /[\uD800-\uDBFF]/.test(physical[end - 1]))
                end--;
            const source = physical.slice(offset, end);
            lines.push((part ? ' ' : line[0]) + source);
            meta.push(part ? { ...original, key: original.key + ':part:' + part,
                old: undefined, next: undefined, continuation: true } : original);
            columns = Math.max(columns, Array.from(source).reduce((n, c) => n + (c === '\t' ? 4 : c.codePointAt(0) >= 0x1100 ? 2 : 1), 0));
            part++;
            offset = end;
        } while (offset < physical.length);
    });
    return { lines, meta, columns, shortened: false };
}

if(typeof module==="object"&&module.exports)module.exports=exports;else root.GtmuxDiffModel=exports;})(typeof window!=="undefined"?window:globalThis);
