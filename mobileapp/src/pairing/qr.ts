// Pairing QR schema (SPEC §6). Two versions:
//   v1 (legacy): { v:1, url, token, name } — the QR carries the Bearer token.
//   v2 (6d):     { v:2, url, enrollCode, name } — the QR carries only a SHORT-LIVED
//        single-use enroll code; we redeem it (POST /api/enroll) for THIS device's
//        own token, so the QR is never a lasting credential. Parser stays tolerant
//        of unknown fields (a future revision may add a TLS cert fingerprint).

export interface PairedMac {
  url: string; // reachable base (scheme+host+port)
  token: string; // the Bearer token
  name: string; // display label
}

// PairResult is what a scanned QR means: either a ready-to-use token (v1) or an
// enroll code we must redeem for a token first (v2).
export type PairResult =
  | {kind: 'paired'; url: string; token: string; name: string}
  | {kind: 'enroll'; url: string; enrollCode: string; name: string};

// labelFromUrl makes a friendly server label from a base URL when the QR omits
// `name`: the host's first DNS label (or the bare IP), stripped of scheme/port.
export function labelFromUrl(url: string): string {
  const host = url.replace(/^https?:\/\//, '').replace(/[/:].*$/, '');
  if (!host) return 'Server';
  // keep the leading label for a hosted/quick tunnel; keep the whole IP/host otherwise.
  return /^\d+\.\d+\.\d+\.\d+$/.test(host) ? host : host.split('.')[0] || host;
}

export function parsePairingQR(raw: string): PairResult {
  let obj: any;
  try {
    obj = JSON.parse(raw);
  } catch {
    throw new Error('Not a gtmux pairing code.');
  }
  const url = String(obj?.url || '').replace(/\/+$/, '');
  if (!/^https?:\/\/.+/.test(url)) throw new Error('Pairing code has no valid url.');
  // v2 QRs omit `name` to stay small; derive a label from the URL host instead
  // (`gtmux-7a3f.ccy.dev` → `gtmux-7a3f`, `1.2.3.4:8765` → `1.2.3.4`).
  const name = String(obj?.name || '') || labelFromUrl(url);
  if (obj?.v === 2) {
    const enrollCode = String(obj.enrollCode || '');
    if (!enrollCode) throw new Error('Pairing code has no enroll code.');
    return {kind: 'enroll', url, enrollCode, name};
  }
  if (obj?.v === 1) {
    const token = String(obj.token || '');
    if (!token) throw new Error('Pairing code has no token.');
    return {kind: 'paired', url, token, name};
  }
  throw new Error('Unsupported pairing-code version.');
}

// EnrollFailure names WHY enrollment failed, so the UI can point at the right fix
// instead of always blaming an "expired code":
//   unreachable — the request never reached an HTTP responder (DNS/TLS/offline/
//                 wrong address). Nothing answered.
//   tunnelDown  — an edge/proxy answered 5xx (Cloudflare 530/1033 "tunnel error",
//                 or 502/503/504): we reached Cloudflare but NOT the gtmux serve
//                 behind it — the Mac's serve or tunnel is offline, code is fine.
//   codeInvalid — the gtmux serve itself rejected the code (4xx): expired/used/typo.
//   noToken     — serve accepted the code but the response carried no token.
export type EnrollFailure = 'unreachable' | 'tunnelDown' | 'codeInvalid' | 'noToken';

// EnrollError carries the classified failure so the screen can localize a precise,
// actionable message (see PairingScreen). The .message stays a plain-English detail
// (with the HTTP status) for logs.
export class EnrollError extends Error {
  kind: EnrollFailure;
  constructor(kind: EnrollFailure, message: string) {
    super(message);
    this.kind = kind;
    this.name = 'EnrollError';
  }
}

// enrollDevice redeems a v2 one-time code for this device's own per-device token
// (POST /api/enroll — unauthenticated; the code is the credential). name labels
// this phone in the Mac's device roster. On failure it throws an EnrollError whose
// .kind distinguishes a dead link/tunnel from a genuinely expired code, so the user
// gets a troubleshooting direction rather than a misleading "expired".
export async function enrollDevice(
  base: string,
  enrollCode: string,
  name: string,
): Promise<string> {
  let r: Response;
  try {
    r = await fetch(`${base}/api/enroll`, {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({enrollCode, name}),
    });
  } catch {
    // fetch rejects only when NOTHING answered — DNS/TLS failure, no route, offline,
    // or the address/port is wrong. Never an expired code.
    throw new EnrollError('unreachable', 'Could not reach the server (no response).');
  }
  if (!r.ok) {
    // 5xx means a proxy/edge answered but the gtmux serve behind it did not — the
    // Mac's serve or tunnel is down (Cloudflare surfaces a dead tunnel as HTTP 530 /
    // error 1033, gateways as 502/503/504). 4xx is the serve rejecting the code.
    if (r.status >= 500) {
      throw new EnrollError('tunnelDown', `Server or tunnel offline (HTTP ${r.status}).`);
    }
    throw new EnrollError('codeInvalid', `Pairing code rejected (HTTP ${r.status}).`);
  }
  let j: any;
  try {
    j = await r.json();
  } catch {
    throw new EnrollError('noToken', 'Enrollment response was not valid JSON.');
  }
  if (!j?.token) throw new EnrollError('noToken', 'Enrollment returned no token.');
  return String(j.token);
}

// Normalize a manually-typed host into a base URL (defaults http:// and port 8765).
export function normalizeHost(input: string): string {
  let h = input.trim().replace(/\/+$/, '');
  if (!h) return '';
  if (!/^https?:\/\//.test(h)) h = `http://${h}`;
  if (!/:\d+$/.test(h.replace(/^https?:\/\//, ''))) h = `${h}:8765`;
  return h;
}
