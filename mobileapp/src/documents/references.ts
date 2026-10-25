// A local destination is a document reference, never an OS URL to launch.
export interface DocumentReference {
  reference: string;
  name: string;
  path: string;
}

export function documentReference(href: string): DocumentReference | null {
  if (!href || href.length > 4096 || /[\u0000-\u001f\u007f]/.test(href)) return null;
  if (/^gtmux-file:[a-f0-9]{32}$/.test(href)) return {reference: href, name: '', path: ''};
  let path = href;
  if (/^file:/i.test(path)) {
    try {
      const url = new URL(path);
      if (url.hostname && url.hostname !== 'localhost' || url.search || url.hash) return null;
      path = decodeURIComponent(url.pathname);
    } catch { return null; }
  }
  if (/^[a-z][a-z\d+.-]*:/i.test(path) || path.startsWith('//')) return null;
  if (!(path.startsWith('/') || path.startsWith('./') || path.startsWith('../') || path.startsWith('~/'))) return null;
  if (/[\u0000-\u001f\u007f]/.test(path)) return null;
  const name = path.split('/').filter(Boolean).pop() ?? path;
  return {reference: path, name, path};
}

export interface ConversationDocument {
  id: string;
  name: string;
  kind: 'html' | 'markdown' | 'text' | 'pdf' | 'image';
  media_type: string;
  bytes: number;
  sha256: string;
  created_at: number;
  session_id: string;
  home_id?: string;
  source: string;
}

export const MAX_DOCUMENT_BYTES = 30 * 1024 * 1024;

export function parseDocument(value: unknown, session: string, home = ''): ConversationDocument | null {
  if (!value || typeof value !== 'object') return null;
  const v = value as Partial<ConversationDocument>;
  if (typeof v.id !== 'string' || !/^[a-f0-9]{32}$/.test(v.id) || typeof v.sha256 !== 'string' || !/^[a-f0-9]{64}$/.test(v.sha256)
    || typeof v.name !== 'string' || !v.name || v.name.length > 1024 || /[\u0000-\u001f]/.test(v.name)
    || !Number.isSafeInteger(v.bytes) || v.bytes! <= 0 || v.bytes! > MAX_DOCUMENT_BYTES
    || !['html', 'markdown', 'text', 'pdf', 'image'].includes(v.kind ?? '') || typeof v.media_type !== 'string'
    || v.session_id !== session || (v.home_id ?? '') !== home || typeof v.source !== 'string' || !v.source.startsWith('/')
    || typeof v.created_at !== 'number' || !Number.isSafeInteger(v.created_at) || v.created_at <= 0) return null;
  return v as ConversationDocument;
}

export class DocumentError extends Error {
  constructor(public reason: 'unsupported' | 'unpublished' | 'forbidden' | 'failed') { super(reason); }
}

export function documentErrorMessage(reason: DocumentError['reason'], zh: boolean): string {
  if (reason === 'unsupported') return zh ? '当前 App 暂不支持文件预览，可以复制路径。' : 'File preview is unavailable in this app version. You can copy the path.';
  if (reason === 'unpublished') return zh ? '此文件尚未提供预览，或已被移除。原始路径仍可复制。' : 'This file has not been made available, or was removed. You can still copy its original path.';
  if (reason === 'forbidden') return zh ? '此连接没有文件查看权限。' : 'This connection cannot view files.';
  return zh ? '暂时无法读取文件。请检查连接后重试。' : 'Could not read the file. Check the connection and retry.';
}
