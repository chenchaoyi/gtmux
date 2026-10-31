export type CodeLanguage =
  | 'javascript'
  | 'json'
  | 'shell'
  | 'python'
  | 'go'
  | 'rust'
  | 'swift'
  | 'html'
  | 'css'
  | 'yaml'
  | 'text';
export type CodeTokenKind =
  | 'plain'
  | 'keyword'
  | 'string'
  | 'number'
  | 'comment'
  | 'property';
export type CodeToken = { text: string; kind: CodeTokenKind };
const MAX_HIGHLIGHT = 32000;
const MAX_TOKENS = 1600;
const aliases: Record<string, CodeLanguage> = {
  js: 'javascript',
  jsx: 'javascript',
  ts: 'javascript',
  tsx: 'javascript',
  javascript: 'javascript',
  typescript: 'javascript',
  json: 'json',
  arguments: 'json',
  parameters: 'json',
  sh: 'shell',
  bash: 'shell',
  zsh: 'shell',
  shell: 'shell',
  py: 'python',
  python: 'python',
  go: 'go',
  golang: 'go',
  rs: 'rust',
  rust: 'rust',
  swift: 'swift',
  html: 'html',
  css: 'css',
  scss: 'css',
  yaml: 'yaml',
  yml: 'yaml',
  text: 'text',
  plaintext: 'text',
};
const words = new Set(
  'const let var await async return function if else for of in while try catch finally throw new class extends import from export default true false null undefined this typeof instanceof void yield break continue switch case None True False def with as lambda pass and or not elif then fi do done echo cd exit package func type struct interface map range go defer select chan fallthrough const mut pub fn impl trait enum mod use match self let where guard protocol extension override init deinit some optional'.split(
    ' ',
  ),
);

export function codeLanguage(source: string, hint = ''): CodeLanguage {
  const known = aliases[String(hint || '').toLowerCase()];
  if (known) return known;
  const head = source.slice(0, 600).trimStart();
  if (/^package\s+\w+/.test(head)) return 'go';
  if (/^(?:pub\s+)?(?:fn|use|impl)\s/.test(head)) return 'rust';
  if (/^import\s+(?:SwiftUI|Foundation|UIKit)\b/.test(head)) return 'swift';
  if (/^<(?!!--)/.test(head)) return 'html';
  if (/^[\[{]/.test(head)) return 'json';
  if (
    /^(?:const|let|var|await|async|function|import|export)\b/.test(head) ||
    /\b(?:tools\.|Promise\.)/.test(head)
  )
    return 'javascript';
  if (
    /^(?:#!.*(?:bash|sh)|(?:cd|git|rg|ls|pwd|echo|python3?|npm|gtmux)\s)/.test(
      head,
    )
  )
    return 'shell';
  if (/^(?:#!.*python|def\s|from\s\S+\simport\s)/.test(head)) return 'python';
  return 'text';
}

export function toolCodeHint(
  title = '',
  inputKind = '',
  operation = '',
): string {
  if (['json', 'arguments', 'parameters'].includes(inputKind)) return 'json';
  if (inputKind === 'code' || operation === 'code') {
    if (/python/i.test(title)) return 'python';
    if (/(?:^|[._])(?:exec|js|javascript)$/.test(title)) return 'javascript';
    return title;
  }
  if (inputKind === 'command' || operation === 'command') return 'shell';
  return title;
}

/** A bounded lexer only: every source character survives, and no input is executed. */
export function codeTokens(
  source: string,
  language: CodeLanguage,
): CodeToken[] {
  if (!source || language === 'text') return [{ text: source, kind: 'plain' }];
  const out: CodeToken[] = [];
  let i = 0,
    plain = 0;
  let cap = Math.min(source.length, MAX_HIGHLIGHT);
  if (cap < source.length && /[\uD800-\uDBFF]/.test(source[cap - 1])) cap--;
  const add = (end: number, kind: CodeTokenKind) => {
    if (i > plain) out.push({ text: source.slice(plain, i), kind: 'plain' });
    out.push({ text: source.slice(i, end), kind });
    i = end;
    plain = end;
  };
  while (i < cap && out.length < MAX_TOKENS) {
    const c = source[i],
      next = source[i + 1];
    const hashComment =
      c === '#' && ['shell', 'python', 'yaml'].includes(language);
    const slashComment =
      c === '/' &&
      ['javascript', 'go', 'rust', 'swift', 'css'].includes(language) &&
      (next === '/' || next === '*');
    if (hashComment || slashComment) {
      const block = slashComment && next === '*';
      let end = block ? source.indexOf('*/', i + 2) : source.indexOf('\n', i);
      end = end < 0 ? cap : Math.min(cap, end + (block ? 2 : 0));
      add(end, 'comment');
      continue;
    }
    if (c === '"' || c === "'" || c.charCodeAt(0) === 96) {
      let end = i + 1;
      while (end < cap) {
        if (source[end] === '\\') {
          end = Math.min(cap, end + 2);
          continue;
        }
        if (source[end++] === c) break;
      }
      const rest = source.slice(end, Math.min(cap, end + 80));
      add(
        end,
        language === 'json' && /^\s*:/.test(rest) ? 'property' : 'string',
      );
      continue;
    }
    if (/[A-Za-z_$]/.test(c)) {
      let end = i + 1;
      while (end < cap && /[\w$]/.test(source[end])) end++;
      if (words.has(source.slice(i, end))) {
        add(end, 'keyword');
        continue;
      }
      if (
        ['javascript', 'yaml', 'go', 'rust', 'swift'].includes(language) &&
        /^\s*:/.test(source.slice(end, Math.min(cap, end + 80)))
      ) {
        add(end, 'property');
        continue;
      }
      if (
        ['javascript', 'python', 'go', 'rust', 'swift'].includes(language) &&
        /^\s*\(/.test(source.slice(end, Math.min(cap, end + 30)))
      ) {
        add(end, 'property');
        continue;
      }
      i = end;
      continue;
    }
    if (/\d/.test(c) && (i === 0 || !/[\w$]/.test(source[i - 1]))) {
      const number =
        /^(?:0[xX][0-9a-fA-F]+|\d+(?:\.\d*)?(?:[eE][+-]?\d+)?)/.exec(
          source.slice(i, cap),
        )!;
      add(i + number[0].length, 'number');
      continue;
    }
    i++;
  }
  if (plain < source.length)
    out.push({ text: source.slice(plain), kind: 'plain' });
  return out;
}

export const syntaxColors = {
  light: {
    plain: '#273242',
    keyword: '#6F3B93',
    string: '#376139',
    number: '#8F4B16',
    comment: '#606C76',
    property: '#245E82',
  },
  dark: {
    plain: '#D8DEE9',
    keyword: '#C4A7E7',
    string: '#A8C997',
    number: '#E6B98D',
    comment: '#A1A7B2',
    property: '#9FC7E9',
  },
};
export function darkCodeBackground(background: string, foreground = '') {
  const channels = (value: string): number[] | null => {
    const hex = /^#([0-9a-f]{6}|[0-9a-f]{3})$/i.exec(value);
    if (hex) {
      const full =
        hex[1].length === 3 ? [...hex[1]].map(c => c + c).join('') : hex[1];
      return [0, 2, 4].map(n => parseInt(full.slice(n, n + 2), 16));
    }
    const rgb = /^rgba?\(\s*(\d+),\s*(\d+),\s*(\d+)/.exec(value);
    if (rgb) return rgb.slice(1, 4).map(Number);
    return value === 'black'
      ? [0, 0, 0]
      : value === 'white'
      ? [255, 255, 255]
      : null;
  };
  const alpha = /^rgba\([^,]+,[^,]+,[^,]+,\s*([\d.]+)\s*\)$/.exec(background);
  const bg = !alpha || Number(alpha[1]) === 1 ? channels(background) : null;
  if (bg) return bg.reduce((a, b) => a + b, 0) < 384;
  const fg = channels(foreground);
  return fg ? fg.reduce((a, b) => a + b, 0) >= 384 : false;
}

export function codeLabel(source: string, hint = '') {
  if (/^(ts|tsx|typescript)$/i.test(hint)) return 'TypeScript';
  const language = codeLanguage(source, hint);
  return {
    javascript: 'JavaScript',
    json: 'JSON',
    shell: 'Shell',
    python: 'Python',
    yaml: 'YAML',
    go: 'Go',
    rust: 'Rust',
    swift: 'Swift',
    html: 'HTML',
    css: 'CSS',
    text: 'Text',
  }[language];
}
