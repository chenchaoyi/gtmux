// Generated code formatter; source SHA256 ad1821fa51bc63df19c7db5b4bfec644b206086859b79df89f0f1fa11b2a186e
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
    // Bound parser/AST work independently of source completeness on a phone.
    if (!parser || source.length > 256000 || !source.trim())
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
