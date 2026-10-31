// Generated code formatter; source SHA256 011f96339a7f358805e4559283404833be5ec082e7cb5d2bf53eceae4773afa0
(function(){var exports={};
"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.formatCode = formatCode;
// Read-only formatting; parsers never execute recorded code. Unsupported or
// incomplete input falls back verbatim, including whitespace-sensitive scripts.
const prettier = window.prettier;
const plugins = Object.values(window.prettierPlugins);
function formatCode(source, hint = '') {
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
    if (!parser || !source.trim())
        return source;
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
    }
    catch {
        return source;
    }
}

window.GtmuxCodeFormat=exports;})();
