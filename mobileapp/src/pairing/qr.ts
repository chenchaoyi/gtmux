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

// enrollDevice redeems a v2 one-time code for this device's own per-device token
// (POST /api/enroll — unauthenticated; the code is the credential). name labels
// this phone in the Mac's device roster.
export async function enrollDevice(
  base: string,
  enrollCode: string,
  name: string,
): Promise<string> {
  const r = await fetch(`${base}/api/enroll`, {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({enrollCode, name}),
  });
  if (!r.ok) throw new Error('Pairing code expired — get a fresh one and rescan.');
  const j: any = await r.json();
  if (!j?.token) throw new Error('Enrollment returned no token.');
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
