// Incoming control-plane and push intents are small objects. Count the actual
// stream before parsing; Content-Length is only an early rejection hint.
export const MAX_JSON_BODY_BYTES = 64 * 1024;

export class JSONBodyError extends Error {
  readonly status: 400 | 413;
  constructor(status: 400 | 413) {
    super(status === 413 ? 'request too large' : 'bad request');
    this.status = status;
  }
}

export async function readJSONObject<T extends object>(request: Request, allowEmpty = false, maxBytes = MAX_JSON_BODY_BYTES): Promise<T> {
  const length = request.headers.get('Content-Length');
  if (length !== null) {
    if (!/^\d+$/.test(length)) throw new JSONBodyError(400);
    if (Number(length) > maxBytes) throw new JSONBodyError(413);
  }
  const reader = request.body?.getReader();
  const parts: Uint8Array[] = [];
  let bytes = 0;
  if (reader) {
    try {
      while (true) {
        const part = await reader.read();
        if (part.done) break;
        bytes += part.value.byteLength;
        if (bytes > maxBytes) {
          try { reader.cancel().catch(() => undefined); } catch { /* The oversized input stays rejected. */ }
          throw new JSONBodyError(413);
        }
        parts.push(part.value);
      }
    } finally {
      reader.releaseLock();
    }
  }
  const body = new Uint8Array(bytes);
  let offset = 0;
  for (const part of parts) { body.set(part, offset); offset += part.byteLength; }
  try {
    const text = new TextDecoder('utf-8', {fatal: true, ignoreBOM: false}).decode(body);
    if (allowEmpty && !text.trim()) return {} as T;
    const value: unknown = JSON.parse(text);
    if (!value || typeof value !== 'object' || Array.isArray(value)) throw new JSONBodyError(400);
    return value as T;
  } catch (error) {
    if (error instanceof JSONBodyError) throw error;
    throw new JSONBodyError(400);
  }
}

export function jsonBodyStatus(error: unknown): 400 | 413 {
  return error instanceof JSONBodyError ? error.status : 400;
}
