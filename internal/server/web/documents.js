// Generated from mobileapp/src/documents/references.ts; SHA256 03520d548c0b5fd769816f26b44efb582ad3e3b58f895a94583c0c9f2df09834
(function(root){var exports={};
"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.DocumentError = exports.MAX_DOCUMENT_BYTES = void 0;
exports.parseFileURI = parseFileURI;
exports.documentReference = documentReference;
exports.parseDocument = parseDocument;
exports.documentErrorMessage = documentErrorMessage;
// RFC 8089 local file URI parser. Never use runtime new URL() for file: URIs:
// React Native's Blob URL polyfill returns pathname="/" and hostname="" for file://.
function parseFileURI(raw) {
    if (!raw || raw.length > 4096 || /[\u0000-\u001f\u007f]/.test(raw) || !/^file:\/\//i.test(raw))
        return null;
    const rest = raw.slice(7);
    const hash = rest.indexOf('#');
    const body = hash >= 0 ? rest.slice(0, hash) : rest;
    if (body.includes('?'))
        return null;
    const slash = body.indexOf('/');
    if (slash < 0)
        return null;
    const authority = body.slice(0, slash);
    if (authority !== '' && authority.toLowerCase() !== 'localhost')
        return null;
    try {
        const decoded = decodeURIComponent(body.slice(slash));
        if (!decoded || decoded.startsWith('//') || /[\u0000-\u001f\u007f]/.test(decoded))
            return null;
        return decoded;
    }
    catch {
        return null;
    }
}
function documentReference(href) {
    if (!href || href.length > 4096 || /[\u0000-\u001f\u007f]/.test(href))
        return null;
    if (/^gtmux-file:[a-f0-9]{32}$/.test(href))
        return { reference: href, name: '', path: '' };
    let path;
    if (/^file:/i.test(href)) {
        const parsed = parseFileURI(href);
        if (!parsed)
            return null;
        path = parsed;
    }
    else {
        if (/^[a-z][a-z\d+.-]*:/i.test(href) || href.startsWith('//'))
            return null;
        const hash = href.indexOf('#');
        const rawPath = hash >= 0 ? href.slice(0, hash) : href;
        if (!rawPath || rawPath.includes('?'))
            return null;
        path = rawPath;
    }
    if (/^[a-z][a-z\d+.-]*:/i.test(path) || path.startsWith('//'))
        return null;
    const relativeFile = /\.(?:html?|md|markdown|pdf|png|jpe?g|txt|log|json|csv|go|rs|py|tsx?|js|css|sh|ya?ml)$/i.test(path);
    if (!(path.startsWith('/') || path.startsWith('./') || path.startsWith('../') || path.startsWith('~/') || relativeFile))
        return null;
    if (/[\u0000-\u001f\u007f]/.test(path))
        return null;
    const name = path.split('/').filter(Boolean).pop() ?? path;
    return { reference: path, name, path };
}
exports.MAX_DOCUMENT_BYTES = 30 * 1024 * 1024;
function parseDocument(value, session, home = '') {
    if (!value || typeof value !== 'object')
        return null;
    const v = value;
    if (typeof v.id !== 'string' || !/^[a-f0-9]{32}$/.test(v.id) || typeof v.sha256 !== 'string' || !/^[a-f0-9]{64}$/.test(v.sha256)
        || typeof v.name !== 'string' || !v.name || v.name.length > 1024 || /[\u0000-\u001f]/.test(v.name)
        || !Number.isSafeInteger(v.bytes) || v.bytes <= 0 || v.bytes > exports.MAX_DOCUMENT_BYTES
        || !['html', 'markdown', 'text', 'pdf', 'image'].includes(v.kind ?? '') || typeof v.media_type !== 'string'
        || v.session_id !== session || (v.home_id ?? '') !== home || typeof v.source !== 'string' || !v.source.startsWith('/')
        || typeof v.created_at !== 'number' || !Number.isSafeInteger(v.created_at) || v.created_at <= 0)
        return null;
    return v;
}
class DocumentError extends Error {
    constructor(reason) {
        super(reason);
        this.reason = reason;
    }
}
exports.DocumentError = DocumentError;
function documentErrorMessage(reason, zh) {
    if (reason === 'unsupported')
        return zh ? '当前 App 暂不支持文件预览，可以复制路径。' : 'File preview is unavailable in this app version. You can copy the path.';
    if (reason === 'unpublished')
        return zh ? '当前对话无法读取此附件。请让发送者重新提供附件链接。' : 'This attachment is not available to this conversation. Ask the sender for a new attachment link.';
    if (reason === 'forbidden')
        return zh ? '此连接没有文件查看权限。' : 'This connection cannot view files.';
    return zh ? '暂时无法读取文件。请检查连接后重试。' : 'Could not read the file. Check the connection and retry.';
}

if(typeof module==="object"&&module.exports)module.exports=exports;else root.GtmuxDocuments=exports;})(typeof window!=="undefined"?window:globalThis);
