// gtmux push relay Worker. Receives a PushIntent from `gtmux serve`
// (Authorization: Bearer <RELAY_TOKEN>), signs an APNs provider JWT (ES256 via
// Web Crypto from the .p8 key), and POSTs the alert to APNs. Mirrors relay/apns.go
// — same payload {aps:{alert:{title,body},sound:default}, pane, kind} + headers.

export interface Env {
  RELAY_TOKEN: string;
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
  subtitle?: string; // the sending Mac's name — shown as the notification subtitle (which server)
  // Live Activity update (push-to-update): when set, `token` is the activity push
  // token and contentState replaces the activity's state on the lock screen.
  liveActivity?: boolean;
  event?: string; // "update" | "end"
  contentState?: Record<string, unknown>;
  // Silent badge/dismiss sync (6a): a content-available push with no alert, used to
  // keep the app-icon badge correct across ALL devices and collapse a resolved
  // agent's banner. badge = absolute waiting count; collapseId = the agent's pane.
  silent?: boolean;
  badge?: number;
  collapseId?: string;
}

let jwtCache: {token: string; at: number} | null = null;

export default {
  async fetch(req: Request, env: Env): Promise<Response> {
    const url = new URL(req.url);
    if (req.method === 'GET' && url.pathname === '/health') return json({ok: true});
    if (req.method !== 'POST') return json({error: 'method not allowed'}, 405);
    if (req.headers.get('Authorization') !== 'Bearer ' + env.RELAY_TOKEN) {
      return json({error: 'unauthorized'}, 401);
    }
    let intent: PushIntent;
    try {
      intent = (await req.json()) as PushIntent;
    } catch {
      return json({error: 'bad json'}, 400);
    }
    if (!intent.token) return json({error: 'no token'}, 400);

    const jwt = await providerJWT(env);
    // Per-token env wins (the device told us at registration); APNS_ENV is the
    // fallback for older tokens registered before the app reported an env.
    const apnsEnv = intent.env || env.APNS_ENV;
    const base = apnsEnv === 'sandbox'
      ? 'https://api.sandbox.push.apple.com'
      : 'https://api.push.apple.com';

    // Live Activity push-to-update: a different topic + push-type, and an
    // aps.content-state that replaces the activity's state even with the app killed.
    if (intent.liveActivity) {
      const laPayload = JSON.stringify({
        aps: {
          timestamp: Math.floor(Date.now() / 1000),
          event: intent.event ?? 'update',
          'content-state': intent.contentState ?? {},
        },
      });
      const lr = await fetch(base + '/3/device/' + intent.token, {
        method: 'POST',
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
      const d = await lr.text();
      return json({error: 'apns', status: lr.status, detail: d}, 502);
    }

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
      // `waiting` pushes carry the AGENT_WAITING category so iOS shows quick-reply
      // actions (1 Yes / 2 Always / 3 No) the app answers without being opened.
      if (intent.kind === 'waiting') aps.category = 'AGENT_WAITING';
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
      'apns-topic': env.APNS_TOPIC,
      // A silent push must be a low-priority background type or APNs throttles it.
      'apns-push-type': intent.silent ? 'background' : 'alert',
      'content-type': 'application/json',
    };
    if (intent.silent) headers['apns-priority'] = '5';
    if (intent.collapseId) headers['apns-collapse-id'] = intent.collapseId;
    const r = await fetch(base + '/3/device/' + intent.token, {
      method: 'POST',
      headers,
      body: payload,
    });
    if (r.status === 200) return json({ok: true});
    const detail = await r.text();
    return json({error: 'apns', status: r.status, detail}, 502);
  },
};

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
