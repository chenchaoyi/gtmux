import {readJSONObject} from '../../shared/worker-json.mts';
// FCM HTTP v1. Provider credentials stay in the relay; never in a client/package.
export interface FCMIntent {
  token: string; title?: string; body?: string; subtitle?: string; pane?: string; kind?: string;
  options?: number; silent?: boolean; badge?: number; collapseId?: string; liveActivity?: boolean;
}
export interface FCMCredentials {project_id: string; client_email: string; private_key: string; type: string}
let cached: {identity: string; token: string; until: number} | null = null;
const encoder = new TextEncoder();
const base64 = (bytes: Uint8Array) => btoa(Array.from(bytes, x => String.fromCharCode(x)).join('')).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
export function fcmMessage(intent: FCMIntent) {
  if (intent.liveActivity) throw new Error('Live Activity is iOS-only');
  if (!intent.token || intent.token.length > 4096) throw new Error('Invalid FCM token');
  const data: Record<string, string> = {
    title: intent.title ?? '', body: intent.body ?? '', server: intent.subtitle ?? '',
    pane: intent.pane ?? '', kind: intent.kind ?? '', silent: String(!!intent.silent),
  };
  if (intent.options !== undefined) data.options = String(intent.options);
  if (intent.badge !== undefined) data.badge = String(intent.badge);
  if (intent.collapseId) data.collapseId = intent.collapseId;
  const result = {message: {token: intent.token, data, android: {priority: intent.silent ? 'NORMAL' : 'HIGH', ttl: intent.silent ? '300s' : '120s'}}};
  if (encoder.encode(JSON.stringify(result)).length > 4096) throw new Error('FCM payload is too large');
  return result;
}
async function accessToken(credentials: FCMCredentials): Promise<string> {
  if (credentials.type !== 'service_account' || !/^[a-z][a-z0-9-]{4,62}$/.test(credentials.project_id) || !/^[^\s@]+@[^\s@]+\.gserviceaccount\.com$/.test(credentials.client_email)) throw new Error('Invalid FCM configuration');
  const identity = base64(new Uint8Array(await crypto.subtle.digest('SHA-256', encoder.encode(credentials.project_id + '\0' + credentials.client_email + '\0' + credentials.private_key))));
  const now = Math.floor(Date.now() / 1000);
  if (cached?.identity === identity && cached.until > now + 60) return cached.token;
  const input = base64(encoder.encode(JSON.stringify({alg: 'RS256', typ: 'JWT'}))) + '.' + base64(encoder.encode(JSON.stringify({iss: credentials.client_email, scope: 'https://www.googleapis.com/auth/firebase.messaging', aud: 'https://oauth2.googleapis.com/token', iat: now, exp: now + 3600})));
  const der = Uint8Array.from(atob(credentials.private_key.replace(/-----BEGIN PRIVATE KEY-----|-----END PRIVATE KEY-----|\s+/g, '')), c => c.charCodeAt(0));
  const key = await crypto.subtle.importKey('pkcs8', der, {name: 'RSASSA-PKCS1-v1_5', hash: 'SHA-256'}, false, ['sign']);
  const signature = await crypto.subtle.sign('RSASSA-PKCS1-v1_5', key, encoder.encode(input));
  const response = await fetch('https://oauth2.googleapis.com/token', {
    method: 'POST', redirect: 'error', headers: {'content-type': 'application/x-www-form-urlencoded'},
    body: new URLSearchParams({grant_type: 'urn:ietf:params:oauth:grant-type:jwt-bearer', assertion: input + '.' + base64(new Uint8Array(signature))}),
    signal: AbortSignal.timeout(8000),
  });
  if (!response.ok) throw new Error('FCM authorization unavailable');
  const value = await readJSONObject<{access_token?: unknown; expires_in?: unknown; token_type?: unknown}>(response as unknown as Request, false, 16384);
  if (typeof value.access_token !== 'string' || value.access_token.length > 8192 || value.token_type !== 'Bearer' || typeof value.expires_in !== 'number' || value.expires_in < 60) throw new Error('Invalid OAuth response');
  cached = {identity, token: value.access_token, until: now + Math.min(3600, value.expires_in)};
  return cached.token;
}
export async function sendFCM(intent: FCMIntent, configuration?: string): Promise<Response> {
  const json = (value: unknown, status: number) => Response.json(value, {status});
  if (!configuration) return json({error: 'android push is not configured'}, 503);
  try {
    const message = fcmMessage(intent);
    const credentials = JSON.parse(configuration) as FCMCredentials;
    const token = await accessToken(credentials);
    const response = await fetch(`https://fcm.googleapis.com/v1/projects/${credentials.project_id}/messages:send`, {
      method: 'POST', redirect: 'error', signal: AbortSignal.timeout(8000),
      headers: {authorization: 'Bearer ' + token, 'content-type': 'application/json'}, body: JSON.stringify(message),
    });
    if (response.ok) return json({ok: true}, 200);
    const value = await readJSONObject<{error?: {details?: {errorCode?: string}[]}}>(response as unknown as Request, false, 16384).catch(() => ({})) as {error?: {details?: {errorCode?: string}[]}};
    const unregistered = value.error?.details?.some(detail => detail.errorCode === 'UNREGISTERED') ?? false;
    return json({error: 'fcm', status: response.status, code: unregistered ? 'UNREGISTERED' : 'REJECTED'}, 502);
  } catch { return json({error: 'android push unavailable'}, 502); }
}
