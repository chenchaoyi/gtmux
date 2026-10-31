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
  if (!parser || !source.trim()) return source;
  try {
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
