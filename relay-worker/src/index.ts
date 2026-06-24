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
  title?: string;
  body?: string;
  pane?: string;
  kind?: string;
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
    const base = env.APNS_ENV === 'sandbox'
      ? 'https://api.sandbox.push.apple.com'
      : 'https://api.push.apple.com';
    // `waiting` pushes carry the AGENT_WAITING category so iOS shows quick-reply
    // actions (1 Yes / 2 Always / 3 No) the app answers without being opened.
    const aps: Record<string, unknown> = {
      alert: {title: intent.title ?? '', body: intent.body ?? ''},
      sound: 'default',
    };
    if (intent.kind === 'waiting') aps.category = 'AGENT_WAITING';
    const payload = JSON.stringify({
      aps,
      pane: intent.pane ?? '',
      kind: intent.kind ?? '',
    });
    const r = await fetch(base + '/3/device/' + intent.token, {
      method: 'POST',
      headers: {
        authorization: 'bearer ' + jwt,
        'apns-topic': env.APNS_TOPIC,
        'apns-push-type': 'alert',
        'content-type': 'application/json',
      },
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
