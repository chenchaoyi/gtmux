// gtmux push relay Worker. Receives a PushIntent from `gtmux serve`
// (Authorization: Bearer <RELAY_TOKEN>), signs an APNs provider JWT (ES256 via
// Web Crypto from the .p8 key), and POSTs the alert to APNs. Mirrors relay/apns.go
// — same payload {aps:{alert:{title,body},sound:default}, pane, kind} + headers.

import {sendFCM} from './fcm.ts';
import {readJSONObject, jsonBodyStatus} from '../../shared/worker-json.mts';

export interface Env {
  RELAY_TOKEN: string;
  RELAY_IP_RPM?: string; // optional per-IP (/64 IPv6) sliding-window ceiling (default 120/min)
  FCM_SERVICE_ACCOUNT?: string; // private HTTP v1 service account; absent means Android unavailable
  APNS_KEY: string; // the AuthKey_*.p8 PEM text
  APNS_KEY_ID: string;
  APNS_TEAM_ID: string;
  APNS_TOPIC: string; // com.gtmux.app
  APNS_ENV: string; // "sandbox" | "production"
}

interface PushIntent {
  token: string;
  platform?: string;
  // The APNs environment THIS token belongs to ("sandbox" for a dev-signed build,
  // "production" for App Store / TestFlight). Lets ONE relay serve both — the Mac
  // forwards the env the device reported at registration. Falls back to APNS_ENV.
  env?: 'sandbox' | 'production';
  title?: string;
  body?: string;
  pane?: string;
  kind?: string;
  // How many numbered choices the waiting pane offers, so the quick-reply category can
  // have that many buttons. Absent = an older Mac that cannot count (keep the legacy
  // category); 0 = nothing parseable (offer no buttons). See waitingCategory.
  options?: number;
  subtitle?: string; // the sending Mac's name — shown as the notification subtitle (which server)
  // Live Activity update (push-to-update): when set, `token` is the activity push
  // token and contentState replaces the activity's state on the lock screen.
  liveActivity?: boolean;
  event?: string; // "update" | "end"
  contentState?: Record<string, unknown>;
  // staleDate (unix seconds): iOS marks the activity stale after this, so the widget
  // dims to "offline". Refreshed on every push + a 30m heartbeat; a dead server stops
  // refreshing it and the card goes stale on its own instead of freezing forever.
  staleDate?: number;
  // Silent badge/dismiss sync (6a): a content-available push with no alert, used to
  // keep the app-icon badge correct across ALL devices and collapse a resolved
  // agent's banner. badge = absolute waiting count; collapseId = the agent's pane.
  silent?: boolean;
  badge?: number;
  collapseId?: string;
}

const APNS_TOKEN_RE = /^[0-9a-fA-F]{32,512}$/;
const APNS_TIMEOUT_MS = 8000;
const RATE_WINDOW_MS = 60_000;
const DEFAULT_IP_RPM = 120;
const MAX_TRACKED_IPS = 4096;

let jwtCache: {token: string; at: number} | null = null;
const ipWindows = new Map<string, number[]>();

export function resetRelayRateLimit(): void {
  ipWindows.clear();
}

export default {
  async fetch(req: Request, env: Env): Promise<Response> {
    const url = new URL(req.url);
    if (req.method === 'GET' && url.pathname === '/health') return json({ok: true});
    if (url.pathname !== '/' && url.pathname !== '/push') return json({error: 'not found'}, 404);
    if (req.method !== 'POST') return json({error: 'method not allowed'}, 405);
    if (!env.RELAY_TOKEN) return json({error: 'relay unavailable'}, 503);
    if (!(await sameBearer(req.headers.get('Authorization'), env.RELAY_TOKEN))) {
      return json({error: 'unauthorized'}, 401);
    }
    const rpm = parseRPM(env.RELAY_IP_RPM);
    const clientIP = normalizeRelayClientIP(req.headers.get('CF-Connecting-IP') || 'unknown');
    if (!allowIPRequest(clientIP, rpm, Date.now())) {
      return new Response(JSON.stringify({error: 'rate limited'}), {
        status: 429,
        headers: {'content-type': 'application/json', 'retry-after': '60'},
      });
    }
    let intent: PushIntent;
    try {
      intent = await readJSONObject<PushIntent>(req);
    } catch (error) {
      const status = jsonBodyStatus(error);
      return json({error: status === 413 ? 'request too large' : 'bad json'}, status);
    }
    if (typeof intent.token !== 'string' || !intent.token) return json({error: 'no token'}, 400);

    const platform = intent.platform || 'ios';
    if (platform === 'android') return sendFCM(intent, env.FCM_SERVICE_ACCOUNT);
    if (platform !== 'ios') return json({error: 'unsupported platform'}, 400);
    if (!APNS_TOKEN_RE.test(intent.token)) return json({error: 'bad token'}, 400);

    // Per-token env wins (the device told us at registration); APNS_ENV is the
    // fallback for older tokens registered before the app reported an env.
    const apnsEnv = intent.env || env.APNS_ENV;
    const base = apnsEnv === 'sandbox'
      ? 'https://api.sandbox.push.apple.com'
      : 'https://api.push.apple.com';

    try {
      const jwt = await providerJWT(env);
      // Live Activity push-to-update: a different topic + push-type, and an
      // aps.content-state that replaces the activity's state even with the app killed.
      if (intent.liveActivity) {
        const laPayload = JSON.stringify({aps: buildLiveActivityAps(intent, Math.floor(Date.now() / 1000))});
        const lr = await fetch(base + '/3/device/' + intent.token, {
          method: 'POST',
          signal: AbortSignal.timeout(APNS_TIMEOUT_MS),
          headers: {
            authorization: 'bearer ' + jwt,
            'apns-topic': env.APNS_TOPIC + '.push-type.liveactivity',
            'apns-push-type': 'liveactivity',
            'apns-priority': '10',
            'content-type': 'application/json',
          },
          body: laPayload,
        });
        if (lr.status === 200) return json({ok: true});
        const d = await lr.text().catch(() => '');
        return json({error: 'apns', status: lr.status, detail: d}, 502);
      }

      const {payload, headers} = buildApnsRequest(intent, jwt, env.APNS_TOPIC);
      const r = await fetch(base + '/3/device/' + intent.token, {
        method: 'POST',
        signal: AbortSignal.timeout(APNS_TIMEOUT_MS),
        headers,
        body: payload,
      });
      if (r.status === 200) return json({ok: true});
      const detail = await r.text().catch(() => '');
      return json({error: 'apns', status: r.status, detail}, 502);
    } catch {
      return json({error: 'apns unavailable'}, 502);
    }
  },
};

// Constant-time bearer comparison over fixed-length SHA-256 digests so neither secret
// length nor matching prefix leaks. Uses Workers native crypto.subtle.timingSafeEqual
// when present and a constant-time byte XOR loop under Node's built-in test runner.
async function sameBearer(supplied: string | null, expected: string): Promise<boolean> {
  if (!expected || !supplied || supplied.length > 1024) return false;
  const [a, b] = await Promise.all([
    crypto.subtle.digest('SHA-256', enc(supplied)),
    crypto.subtle.digest('SHA-256', enc('Bearer ' + expected)),
  ]);
  const subtle = crypto.subtle as SubtleCrypto & {timingSafeEqual?: (x: ArrayBuffer, y: ArrayBuffer) => boolean};
  if (typeof subtle.timingSafeEqual === 'function') {
    return subtle.timingSafeEqual(a, b);
  }
  const aa = new Uint8Array(a), bb = new Uint8Array(b);
  let diff = aa.length ^ bb.length;
  for (let i = 0; i < aa.length; i++) diff |= aa[i] ^ bb[i];
  return diff === 0;
}

function parseRPM(raw: string | undefined): number {
  const n = Number(raw);
  return Number.isFinite(n) && n > 0 ? Math.min(Math.floor(n), 10_000) : DEFAULT_IP_RPM;
}

function allowIPRequest(ipKey: string, limit: number, nowMs: number): boolean {
  const cutoff = nowMs - RATE_WINDOW_MS;
  let stamps = ipWindows.get(ipKey);
  if (stamps) {
    let idx = 0;
    while (idx < stamps.length && stamps[idx] <= cutoff) idx++;
    if (idx > 0) stamps = stamps.slice(idx);
  } else {
    stamps = [];
  }
  if (stamps.length >= limit) {
    ipWindows.set(ipKey, stamps);
    return false;
  }
  if (!ipWindows.has(ipKey) && ipWindows.size >= MAX_TRACKED_IPS) {
    for (const [k, v] of ipWindows) {
      if (!v.length || v[v.length - 1] <= cutoff) ipWindows.delete(k);
    }
    if (ipWindows.size >= MAX_TRACKED_IPS) {
      const oldest = ipWindows.keys().next().value;
      if (oldest !== undefined) ipWindows.delete(oldest);
    }
  }
  stamps.push(nowMs);
  ipWindows.set(ipKey, stamps);
  return true;
}

export function normalizeRelayClientIP(raw: string): string {
  const trimmed = (raw || '').trim();
  if (!trimmed || trimmed === 'unknown') return 'unknown';
  const v4 = parseIPv4(trimmed);
  if (v4) return v4;
  const noZone = trimmed.split('%')[0];
  if (!noZone.includes(':')) return trimmed;
  const groups = parseIPv6Groups(noZone);
  if (!groups) return trimmed.toLowerCase();
  if (groups[0] === 0 && groups[1] === 0 && groups[2] === 0 && groups[3] === 0 && groups[4] === 0 && groups[5] === 0xffff) {
    return `${groups[6] >> 8}.${groups[6] & 0xff}.${groups[7] >> 8}.${groups[7] & 0xff}`;
  }
  return groups.slice(0, 4).map(g => g.toString(16).padStart(4, '0')).join(':') + '::/64';
}

function parseIPv4(s: string): string | null {
  const parts = s.split('.');
  if (parts.length !== 4) return null;
  const nums: number[] = [];
  for (const p of parts) {
    if (!/^(?:0|[1-9]\d{0,2})$/.test(p)) return null;
    const n = Number(p);
    if (n < 0 || n > 255) return null;
    nums.push(n);
  }
  return nums.join('.');
}

function parseIPv6Groups(s: string): number[] | null {
  let working = s;
  const lastColon = working.lastIndexOf(':');
  if (lastColon !== -1 && working.slice(lastColon + 1).includes('.')) {
    const v4 = parseIPv4(working.slice(lastColon + 1));
    if (!v4) return null;
    const octets = v4.split('.').map(Number);
    const hi = ((octets[0] << 8) | octets[1]).toString(16);
    const lo = ((octets[2] << 8) | octets[3]).toString(16);
    working = working.slice(0, lastColon + 1) + hi + ':' + lo;
  }
  const halves = working.split('::');
  if (halves.length > 2) return null;
  const parseSide = (part: string): number[] | null => {
    if (!part) return [];
    const out: number[] = [];
    for (const tok of part.split(':')) {
      if (!/^[0-9a-fA-F]{1,4}$/.test(tok)) return null;
      out.push(parseInt(tok, 16));
    }
    return out;
  };
  const left = parseSide(halves[0]);
  if (!left) return null;
  if (halves.length === 1) return left.length === 8 ? left : null;
  const right = parseSide(halves[1]);
  if (!right) return null;
  const missing = 8 - left.length - right.length;
  if (missing < 1) return null;
  return [...left, ...new Array<number>(missing).fill(0), ...right];
}

// waitingCategory picks the quick-reply category by how many numbered choices the pane
// is ACTUALLY offering.
//
// One fixed category was a bug, not a simplification: an iOS category's actions are
// frozen at registration, so every waiting push showed three buttons reading
// "1 · Yes / 2 · Always / 3 · No" — and Claude's common two-option prompt is
// "1. Yes / 2. No, and tell Claude what to do", where the button labelled ALWAYS sends
// the answer that means NO and the third button offers a digit that is not a choice.
//
// `options` absent = an older Mac that cannot count: keep the old category so its quick
// reply still works. `options` 0 = a new Mac saying nothing was parseable: NO category,
// so no buttons are offered for choices we cannot see.
function waitingCategory(options?: number): string | null {
  if (options === undefined || options === null) return 'AGENT_WAITING';
  if (options < 2) return null;
  return `AGENT_WAITING_${Math.min(options, 4)}`;
}

// buildLiveActivityAps builds the `aps` object for a Live Activity push-to-update. Pure
// (clock injected) so index.test.ts can pin its shape. `stale-date` is emitted only when
// the intent carries one — it tells iOS when to dim the card to "offline", so a dead
// server (no more pushes refreshing it) stops showing a frozen tally forever.
export function buildLiveActivityAps(intent: PushIntent, nowSec: number): Record<string, unknown> {
  const aps: Record<string, unknown> = {
    timestamp: nowSec,
    event: intent.event ?? 'update',
    'content-state': intent.contentState ?? {},
  };
  if (typeof intent.staleDate === 'number' && intent.staleDate > 0) {
    aps['stale-date'] = intent.staleDate;
  }
  return aps;
}

// buildApnsRequest builds the {payload, headers} for a standard alert/silent push.
// This payload SHAPE is a cross-runtime contract that MUST stay in sync with the Go
// reference impl relay/apns.go (per AGENTS.md's deploy table). It is pure (no network,
// no clock, no env beyond the injected topic) so index.test.ts can pin it against a
// golden fixture — the only automated guard that a shape edit here doesn't silently
// diverge from apns.go / the app's expectations. `jwt` and `topic` are injected.
export function buildApnsRequest(
  intent: PushIntent,
  jwt: string,
  topic: string,
): {payload: string; headers: Record<string, string>} {
  const aps: Record<string, unknown> = {};
  if (intent.silent) {
    // Badge/dismiss sync: no alert/sound — just content-available + the absolute
    // badge, so a second (offline-until-now) phone clears its red dot.
    aps['content-available'] = 1;
  } else {
    const alert: Record<string, unknown> = {title: intent.title ?? '', body: intent.body ?? ''};
    // subtitle = which Mac (multi-server): the bold line between title and body.
    if (intent.subtitle) alert.subtitle = intent.subtitle;
    aps.alert = alert;
    aps.sound = 'default';
    // mutable-content wakes the app's Notification Service Extension, which
    // attaches a per-kind status badge (red stop / green ✓) to the banner.
    aps['mutable-content'] = 1;
    // `waiting` pushes carry a quick-reply category so iOS shows numbered actions the
    // app answers without being opened. WHICH category depends on how many choices the
    // pane is actually offering — see waitingCategory.
    if (intent.kind === 'waiting') {
      const cat = waitingCategory(intent.options);
      if (cat) aps.category = cat;
    }
  }
  if (typeof intent.badge === 'number') aps.badge = intent.badge;
  const payload = JSON.stringify({
    aps,
    pane: intent.pane ?? '',
    kind: intent.kind ?? '',
    // top-level (readable on tap) so the app can route to the RIGHT server; same
    // value as aps.alert.subtitle, which is display-only.
    server: intent.subtitle ?? '',
  });
  const headers: Record<string, string> = {
    authorization: 'bearer ' + jwt,
    'apns-topic': topic,
    // A silent push must be a low-priority background type or APNs throttles it.
    'apns-push-type': intent.silent ? 'background' : 'alert',
    'content-type': 'application/json',
  };
  if (intent.silent) headers['apns-priority'] = '5';
  if (intent.collapseId) headers['apns-collapse-id'] = intent.collapseId;
  return {payload, headers};
}

async function providerJWT(env: Env): Promise<string> {
  const now = Date.now();
  if (jwtCache && now - jwtCache.at < 50 * 60 * 1000) return jwtCache.token;
  const header = b64urlStr(JSON.stringify({alg: 'ES256', kid: env.APNS_KEY_ID}));
  const claims = b64urlStr(JSON.stringify({iss: env.APNS_TEAM_ID, iat: Math.floor(now / 1000)}));
  const signingInput = header + '.' + claims;
  const key = await importP8(env.APNS_KEY);
  const sig = await crypto.subtle.sign({name: 'ECDSA', hash: 'SHA-256'}, key, enc(signingInput));
  const token = signingInput + '.' + b64urlBytes(new Uint8Array(sig));
  jwtCache = {token, at: now};
  return token;
}

async function importP8(pem: string): Promise<CryptoKey> {
  const body = pem
    .replace(/-----BEGIN PRIVATE KEY-----/, '')
    .replace(/-----END PRIVATE KEY-----/, '')
    .replace(/\s+/g, '');
  const der = Uint8Array.from(atob(body), c => c.charCodeAt(0));
  return crypto.subtle.importKey('pkcs8', der, {name: 'ECDSA', namedCurve: 'P-256'}, false, ['sign']);
}

function enc(s: string): Uint8Array {
  return new TextEncoder().encode(s);
}
function b64urlStr(s: string): string {
  return b64urlBytes(enc(s));
}
function b64urlBytes(b: Uint8Array): string {
  let s = '';
  for (const x of b) s += String.fromCharCode(x);
  return btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}
function json(o: unknown, status = 200): Response {
  return new Response(JSON.stringify(o), {status, headers: {'content-type': 'application/json'}});
}
