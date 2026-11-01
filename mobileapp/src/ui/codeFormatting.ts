// Read-only formatting; parsers never execute recorded code. Unsupported or
// incomplete input falls back verbatim, including whitespace-sensitive scripts.
const prettier: {
  format: (source: string, options: Record<string, unknown>) => string;
} = require('prettier/standalone');
const plugins = [
  require('prettier/parser-babel'),
  require('prettier/parser-html'),
  require('prettier/parser-postcss'),
  require('prettier/parser-yaml'),
];
const pythonParser: typeof import('@lezer/python').parser =
  require('@lezer/python').parser;

type PythonNode = import('@lezer/common').SyntaxNode;

// Compare syntax and token values, including literal contents and suite ownership.
// StatementGroup and semicolons are layout only; moving a statement out of a suite
// changes this signature and rejects the proposed display transform.
function pythonShape(node: PythonNode, source: string, depth = 0): unknown[] {
  if (depth > 160 || node.type.isError) throw new Error('Unsupported Python');
  if (node.name === ';') return [];
  if (/^(?:String|FormatString|Comment)$/.test(node.name))
    return [[node.name, source.slice(node.from, node.to)]];
  const children: unknown[] = [];
  for (
    let child: PythonNode | null = node.firstChild;
    child;
    child = child.nextSibling
  )
    children.push(...pythonShape(child, source, depth + 1));
  return node.name === 'StatementGroup'
    ? children
    : [
        [
          node.name,
          children.length ? children : source.slice(node.from, node.to),
        ],
      ];
}

function formatPython(source: string): string {
  const tree = pythonParser.parse(source);
  let nodes = 0;
  tree.iterate({
    enter(node) {
      if (++nodes > 20000 || node.type.isError)
        throw new Error('Unsupported Python');
    },
  });
  const shape = JSON.stringify(pythonShape(tree.topNode, source));
  const tokens: PythonNode[] = [];
  function collect(node: PythonNode, depth = 0) {
    if (depth > 160 || tokens.length > 20000 || node.type.isError)
      throw new Error('Unsupported Python');
    if (!node.firstChild || /^(?:String|FormatString|Comment)$/.test(node.name))
      tokens.push(node);
    else
      for (
        let child: PythonNode | null = node.firstChild;
        child;
        child = child.nextSibling
      )
        collect(child, depth + 1);
  }
  collect(tree.topNode);
  function indent(node: PythonNode) {
    let body: PythonNode | null = node.parent;
    while (body && body.name !== 'Body') body = body.parent;
    const anchor = body || node;
    const line = source.lastIndexOf('\n', anchor.from - 1) + 1;
    const prefix = source.slice(line).match(/^[\t ]*/)?.[0] || '';
    return body && !source.slice(body.from, node.from).includes('\n')
      ? prefix + '    '
      : source
          .slice(source.lastIndexOf('\n', node.from - 1) + 1)
          .match(/^[\t ]*/)?.[0] || '';
  }
  function breakContainer(node: PythonNode | null): node is PythonNode {
    return (
      !!node &&
      /^(?:DictionaryExpression|ArrayExpression|ArgList|TupleExpression)$/.test(
        node.name,
      ) &&
      node.to - node.from > 100 &&
      !source.slice(node.from, node.to).includes('\n')
    );
  }
  function containerIndent(node: PythonNode, extra: number) {
    const line = source.lastIndexOf('\n', node.from - 1) + 1;
    let spaces = extra;
    for (let parent = node.parent; parent; parent = parent.parent)
      if (breakContainer(parent)) spaces += 4;
    return (
      (source.slice(line).match(/^[\t ]*/)?.[0] || '') + ' '.repeat(spaces)
    );
  }
  let out = source.slice(0, tokens[0]?.from || 0);
  for (let i = 0; i < tokens.length; i++) {
    const token = tokens[i],
      next = tokens[i + 1];
    out += token.name === ';' ? '' : source.slice(token.from, token.to);
    if (!next) {
      out += source.slice(token.to);
      break;
    }
    const gap = source.slice(token.to, next.from);
    if (token.name === ';') out += '\n' + indent(next);
    else if (
      /^[)\]}]$/.test(next.name) &&
      breakContainer(next.parent) &&
      /^\s*$/.test(gap)
    )
      out += '\n' + containerIndent(next.parent, 0);
    else if (
      (/^[([{]$/.test(token.name) || token.name === ',') &&
      breakContainer(token.parent) &&
      /^\s*$/.test(gap)
    )
      out += '\n' + containerIndent(token.parent, 4);
    else if (
      token.name === ':' &&
      token.parent?.name === 'Body' &&
      !gap.includes('\n')
    )
      out += '\n' + indent(next);
    else if (!/[\r\n\\]/.test(gap) && /^\s*$/.test(gap)) {
      if (next.name === ';' || next.name === ',') out += '';
      else if (
        token.name === ',' ||
        token.name === 'AssignOp' ||
        next.name === 'AssignOp' ||
        (token.name === 'ArithOp' &&
          token.parent?.name === 'BinaryExpression') ||
        (next.name === 'ArithOp' && next.parent?.name === 'BinaryExpression') ||
        token.name === 'CompareOp' ||
        next.name === 'CompareOp' ||
        (token.name === ':' && token.parent?.name === 'DictionaryExpression')
      )
        out += ' ';
      else out += gap;
    } else out += gap;
  }
  return JSON.stringify(pythonShape(pythonParser.parse(out).topNode, out)) ===
    shape
    ? out
    : source;
}

export type CodePart = {
  id: string;
  source: string;
  hint: string;
  path: string;
};

/** Decode only static recorded values; never evaluate JS, templates or shell. */
export function codeParts(source: string, hint = ''): CodePart[] {
  const original = { id: 'original', source, hint, path: '' };
  if (source.length > 256000) return [original];
  const found: CodePart[] = [];
  let fields = 0;
  function script(command: string, path: string) {
    const lines = command.split('\n');
    const scripts: CodePart[] = [];
    for (let i = 0; i < lines.length; i++) {
      const start = lines[i].match(
        /^\s*(python(?:\d(?:\.\d+)?)?|node(?:js)?|bash|sh|zsh)\s+(?:-\w+\s+)*(?:-\s+)?<<(-?)(['"]?)([A-Za-z_]\w*)\3\s*\r?$/,
      );
      if (!start) continue;
      let end = i + 1;
      while (
        end < lines.length &&
        (start[2] ? lines[end].replace(/^\t+/, '') : lines[end]).replace(
          /\r$/,
          '',
        ) !== start[4]
      )
        end++;
      if (end === lines.length) continue;
      const text =
        lines
          .slice(i + 1, end)
          .map(line => (start[2] ? line.replace(/^\t+/, '') : line))
          .join('\n') + '\n';
      scripts.push({
        id: path + ':' + i,
        source: text,
        hint: start[1].startsWith('python')
          ? 'python'
          : start[1].startsWith('node')
          ? 'javascript'
          : 'shell',
        path: path || 'stdin ' + (scripts.length + 1),
      });
      i = end;
    }
    // Keep each outer command before its scripts, matching source expansion.
    if (path && command.trim())
      found.push({ id: path, source: command, hint: 'shell', path });
    found.push(...scripts);
  }
  try {
    if (/^(?:shell|sh|bash|zsh|command)$/.test(hint)) script(source, '');
    else {
      const parser = (
        plugins[0] as {
          parsers: {
            babel: {
              parse: (
                value: string,
                parsers: object,
                options: object,
              ) => unknown;
            };
          };
        }
      ).parsers.babel;
      let ast: unknown;
      try {
        ast = JSON.parse(source);
      } catch {
        ast = parser.parse(source, {}, {});
      }
      let visited = 0;
      function walk(value: unknown, path: string, depth: number) {
        if (++visited > 20000 || depth > 100) throw new Error('Large wrapper');
        if (!value || typeof value !== 'object') return;
        const node = value as Record<string, unknown>;
        if (
          (node.type === 'ObjectProperty' || node.type === 'Property') &&
          !node.computed
        ) {
          const key = node.key as Record<string, unknown>;
          const name = key?.name || key?.value;
          const val = node.value as Record<string, unknown>;
          if (/^(?:cmd|command)$/.test(String(name)) && val) {
            if (
              typeof val.value === 'string' &&
              /^(?:StringLiteral|Literal)$/.test(String(val.type))
            )
              script(val.value, String(name) + ' ' + ++fields);
            else if (
              val.type === 'TemplateLiteral' &&
              !(val.expressions as unknown[])?.length
            ) {
              const text = (
                (val.quasis as Record<string, unknown>[])?.[0]?.value as Record<
                  string,
                  unknown
                >
              )?.cooked;
              if (typeof text === 'string')
                script(text, String(name) + ' ' + ++fields);
            }
          }
        }
        for (const [key, child] of Object.entries(node)) {
          if (/^(?:loc|range|tokens|comments|extra)$/.test(key)) continue;
          if (/^(?:cmd|command)$/.test(key) && typeof child === 'string')
            script(child, key + ' ' + ++fields);
          else if (typeof child === 'object' && child)
            walk(child, path, depth + 1);
        }
      }
      walk(ast, '', 0);
    }
  } catch {
    return [original];
  }
  // A large wrapper stays readable in full without mounting an unbounded tab list.
  return found.length && found.length <= 16 ? [original, ...found] : [original];
}

export function formatCode(source: string, hint = ''): string {
  const kind = String(hint || '').toLowerCase();
  const parser = /^(?:ts|tsx|typescript)$/.test(kind)
    ? 'babel-ts'
    : /^(?:js|jsx|javascript|code|exec)$/.test(kind)
    ? 'babel'
    : /^(?:json|arguments|parameters)$/.test(kind)
    ? 'json'
    : /^(?:html|vue|angular)$/.test(kind)
    ? 'html'
    : /^(?:css|scss|less)$/.test(kind)
    ? kind
    : /^(?:yaml|yml)$/.test(kind)
    ? 'yaml'
    : '';
  // Bound parser/AST work independently of source completeness on a phone.
  if (source.length > 256000 || !source.trim()) return source;
  try {
    if (/^(?:python|py)$/.test(kind)) return formatPython(source);
    if (!parser) return source;
    return prettier
      .format(source, {
        parser,
        plugins,
        tabWidth: 2,
        printWidth: 100,
        singleQuote: true,
        trailingComma: 'none',
        bracketSpacing: true,
        proseWrap: 'preserve',
      })
      .trimEnd();
  } catch {
    return source;
  }
}
