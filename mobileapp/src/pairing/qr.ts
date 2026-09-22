// Pairing QR schema (SPEC §6). Two versions:
//   v1 (legacy): { v:1, url, token, name } — the QR carries the Bearer token.
//   v2 (6d):     { v:2, url, enrollCode, name } — the QR carries only a SHORT-LIVED
//        single-use enroll code; we redeem it (POST /api/enroll) for THIS device's
//        own token, so the QR is never a lasting credential. Parser stays tolerant
//        of unknown fields (a future revision may add a TLS cert fingerprint).

import {Diag} from '../diag';
import {PAIR_STEP_MS} from './deadline';

export interface PairedMac {
  url: string; // reachable base (scheme+host+port)
  token: string; // the Bearer token
  name: string; // display label
  // How this Mac was paired / what the token can do:
  //   'owner' (default) — a device token (full: the owner's own phone);
  //   'guest'           — a `gtmux share` guest token (scope-restricted; see the
  //                        view/input allowlists resolved from GET /api/share).
  // Absent on old stored blobs → treated as 'owner'.
  scope?: 'owner' | 'guest';
}

// PairResult is what a scanned QR / entered credential means: a ready-to-use device
// token (v1), an enroll code we must redeem first (v2), or a GUEST share token taken
// straight from a `gtmux share` link (scope-restricted, no enroll).
export type PairResult =
  | {kind: 'paired'; url: string; token: string; name: string}
  | {kind: 'enroll'; url: string; enrollCode: string; name: string}
  | {kind: 'guest'; url: string; token: string; name: string};

// labelFromUrl makes a friendly server label from a base URL when the QR omits
// `name`: the host's first DNS label (or the bare IP), stripped of scheme/port.
export function labelFromUrl(url: string): string {
  const host = url.replace(/^https?:\/\//, '').replace(/[/:].*$/, '');
  if (!host) return 'Server';
  // keep the leading label for a hosted/quick tunnel; keep the whole IP/host otherwise.
  return /^\d+\.\d+\.\d+\.\d+$/.test(host) ? host : host.split('.')[0] || host;
}

// parseShareLink recognizes a `gtmux share` GUEST link — `<base>/#g=<token>` (what
// `gtmux share new` mints + encodes in its QR; the legacy `#t=` form is still
// accepted so old links keep working). The app uses that token directly as a
// scope-restricted GUEST bearer (no enroll). Returns null for anything that isn't a
// share link, so the caller falls through to the pair-link / JSON pairing-QR path.
export function parseShareLink(raw: string): (PairResult & {kind: 'guest'}) | null {
  const m = /^(https?:\/\/[^#]+?)\/*#(.*)$/.exec(raw.trim());
  if (!m) return null;
  const base = m[1].replace(/\/+$/, '');
  const tm = /(?:^|[?&])[gt]=([^&]+)/.exec(m[2]);
  if (!tm) return null; // a fragment without a g=/t= token (e.g. #c=<enroll> is not a guest link)
  let token = tm[1];
  try {
    token = decodeURIComponent(token);
  } catch {
    /* keep raw */
  }
  if (!token) return null;
  return {kind: 'guest', url: base, token, name: labelFromUrl(base)};
}

// parsePairLink recognizes a PAIR link — `<base>/#c=<code>` (what `gtmux pair` /
// the menu-bar pairing sheet mint as the browser medium). It carries the same
// short-lived enroll code as the JSON v2 QR, so it joins the enroll path: the code
// is redeemed (POST /api/enroll) for this device's own OWNER token. This makes the
// three pairing media equivalent — scanning the browser link works like the QR.
export function parsePairLink(raw: string): (PairResult & {kind: 'enroll'}) | null {
  const m = /^(https?:\/\/[^#]+?)\/*#(.*)$/.exec(raw.trim());
  if (!m) return null;
  const base = m[1].replace(/\/+$/, '');
  const cm = /(?:^|[?&])c=([^&]+)/.exec(m[2]);
  if (!cm) return null;
  let enrollCode = cm[1];
  try {
    enrollCode = decodeURIComponent(enrollCode);
  } catch {
    /* keep raw */
  }
  if (!enrollCode) return null;
  return {kind: 'enroll', url: base, enrollCode, name: labelFromUrl(base)};
}

export function parsePairingQR(raw: string): PairResult {
  // A guest share link / pair link is a URL (not JSON) — check those first.
  const guest = parseShareLink(raw);
  if (guest) return guest;
  const pair = parsePairLink(raw);
  if (pair) return pair;
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
  timeoutMs: number = PAIR_STEP_MS,
): Promise<string> {
  // The pairing, whatever its outcome, goes to the diagnostics buffer: the record the
  // phone did not keep on 2026-09-19. The code is a credential and never written.
  Diag.secret(enrollCode);
  const host = base.replace(/^https?:\/\//, '').split('/')[0];
  const refuse = (kind: EnrollFailure, status?: number, error?: string): never => {
    Diag.act('act.pair', host, kind === 'codeInvalid' ? 'refused' : 'failed', 'pairing with a Mac did not complete',
      {reason: kind, status, error});
    throw new EnrollError(kind, enrollMessage(kind, status));
  };
  // Bounded, and actually cancelled: a request nothing answers (a phone VPN swallowed it,
  // 2026-09-22) otherwise waits out iOS's own idle timeout while the scan spins.
  const ctl = new AbortController();
  const timer = setTimeout(() => ctl.abort(), timeoutMs);
  let r: Response;
  try {
    r = await fetch(`${base}/api/enroll`, {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({enrollCode, name}),
      signal: ctl.signal,
    });
  } catch (e: any) {
    // fetch rejects only when NOTHING answered — DNS/TLS failure, no route, offline,
    // or the address/port is wrong. Never an expired code.
    return refuse('unreachable', undefined,
      ctl.signal.aborted ? `no answer within ${Math.round(timeoutMs / 1000)}s` : String(e?.message || e));
  } finally {
    clearTimeout(timer);
  }
  if (!r.ok) {
    // 5xx means a proxy/edge answered but the gtmux serve behind it did not — the
    // Mac's serve or tunnel is down (Cloudflare surfaces a dead tunnel as HTTP 530 /
    // error 1033, gateways as 502/503/504). 4xx is the serve rejecting the code.
    return refuse(r.status >= 500 ? 'tunnelDown' : 'codeInvalid', r.status);
  }
  let j: any;
  try {
    j = await r.json();
  } catch {
    return refuse('noToken', r.status);
  }
  if (!j?.token) return refuse('noToken', r.status);
  Diag.secret(String(j.token));
  Diag.act('act.pair', host, 'ok', 'paired with a Mac', {status: r.status});
  return String(j.token);
}

// enrollMessage is the EnrollError text for each way a pairing fails.
function enrollMessage(kind: EnrollFailure, status?: number): string {
  switch (kind) {
    case 'unreachable':
      return 'Could not reach the server (no response).';
    case 'tunnelDown':
      return `Server or tunnel offline (HTTP ${status}).`;
    case 'codeInvalid':
      return `Pairing code rejected (HTTP ${status}).`;
    default:
      return status === undefined || status === 200 ? 'Enrollment returned no token.' : 'Enrollment response was not valid JSON.';
  }
}

// Normalize a manually-typed host into a base URL (defaults http:// and port 8765).
export function normalizeHost(input: string): string {
  let h = input.trim().replace(/\/+$/, '');
  if (!h) return '';
  if (!/^https?:\/\//.test(h)) h = `http://${h}`;
  if (!/:\d+$/.test(h.replace(/^https?:\/\//, ''))) h = `${h}:8765`;
  return h;
}
