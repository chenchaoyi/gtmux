#!/usr/bin/env python3
"""Code comments are English (CLAUDE.md, "CODE IS ENGLISH", 2026-09-13).

A comment may QUOTE Chinese — a user report, a product string, a pane name — because the
evidence is worth keeping verbatim. What it may not be is written in Chinese. The line
between the two is mechanical enough to check: a comment line whose Chinese outweighs its
Latin letters, and that carries no quotation marks (「」“”"『』 or backticks) and is not a
Go doc code block (a `//` followed by a tab), is Chinese prose and fails the build.

Scans every tracked Go / Swift / TS / JS / Ruby / shell file. Docs, specs, the generated
release notes and the Chinese playbook (a string, not a comment) are out of scope.
"""
import os, re, subprocess, sys

CJK = re.compile(r'[一-鿿]')
QUOTED = re.compile(r'[「」“”"『』`]')
EXTS = {'.go': 'c', '.swift': 'c', '.ts': 'c', '.tsx': 'c', '.js': 'c', '.mjs': 'c', '.rb': 'h', '.sh': 'h', '.py': 'h'}
SKIP = ('docs/', 'openspec/', 'README', 'mobileapp/src/releaseNotes.ts', 'internal/hq/playbook_zh.go',
        'mobileapp/node_modules', 'mobileapp/ios/Pods', 'scripts/check-comment-language.py')


def comments(text, kind):
    out = []
    lines = text.split('\n')
    if kind == 'h':
        for n, line in enumerate(lines, 1):
            s = line.lstrip()
            if s.startswith('#'):
                out.append((n, s))
        return out
    for m in re.finditer(r'/\*.*?\*/', text, re.S):
        n = text.count('\n', 0, m.start()) + 1
        for i, l in enumerate(m.group(0).split('\n')):
            out.append((n + i, l.strip()))
    for n, line in enumerate(lines, 1):
        i = 0
        while True:
            j = line.find('//', i)
            if j < 0:
                break
            before = line[:j]
            if before.endswith(':') or before.count('"') % 2 or before.count('`') % 2 or before.count("'") % 2:
                i = j + 2
                continue
            out.append((n, line[j:].strip()))
            break
    return out


def prose_in_chinese(line):
    if line.startswith('//\t'):  # a Go doc code block: example content, not prose
        return False
    if QUOTED.search(line):
        return False
    cjk = len(CJK.findall(line))
    return cjk > 0 and cjk * 2 > len(re.findall(r'[A-Za-z]', line))


def main():
    files = subprocess.check_output(['git', 'ls-files'], text=True).split('\n')
    bad = []
    for f in files:
        if not f or f.startswith(SKIP):
            continue
        kind = EXTS.get(os.path.splitext(f)[1])
        if not kind:
            continue
        try:
            text = open(f, encoding='utf-8').read()
        except (OSError, UnicodeDecodeError):
            continue
        for n, l in comments(text, kind):
            if prose_in_chinese(l):
                bad.append(f'{f}:{n}: {l[:100]}')
    if bad:
        print('comment-language: comments are English; a quoted Chinese report is fine, Chinese prose is not (CLAUDE.md "CODE IS ENGLISH"):')
        for b in bad:
            print('  ' + b)
        return 1
    print('comment-language: OK — no Chinese prose in code comments')
    return 0


if __name__ == '__main__':
    sys.exit(main())
