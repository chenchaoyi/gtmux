// gtmux tunnel control-plane Worker.
//
// One endpoint that matters: POST /provision. It idempotently creates (or reuses)
// a Cloudflare *named* tunnel for the caller's Mac plus a stable
// `<id>.gtmux.ccy.dev` hostname, and returns the connector token the Mac runs
// `cloudflared tunnel run --token <token>` with. Cloudflare carries the data; this
// Worker only drives the CF API.
//
// Why named (not quick) tunnels: a stable hostname means the phone pairs ONCE and
// keeps reaching the Mac across reboots / URL rotations — the thing quick tunnels
// can't give (see openspec remote-access spec).

export interface Env {
  TUNNELS: KVNamespace;
  CF_API_TOKEN: string; // secret: zone DNS:Edit + account Cloudflare Tunnel:Edit
  REG_SECRET: string; // secret: soft anti-abuse gate, baked into the CLI build
  ZONE_NAME: string; // e.g. "gtmux.ccy.dev"
  LOCAL_SERVICE: string; // e.g. "http://localhost:8765"
  CF_ACCOUNT_ID: string;
  CF_ZONE_ID: string;
  // Abuse caps + reaper thresholds (strings from wrangler [vars]; parsed with defaults).
  PROVISION_IP_CAP?: string;
  PROVISION_GLOBAL_CAP?: string;
  REAP_NEVER_CONNECTED_H?: string;
  REAP_IDLE_DAYS?: string;
  // Paid "Direct" tunnel unlock. A code the user bought/received is validated HERE
  // (not in the open client), and only on a hit do we hand back the Direct server
  // config — so the chisel secret never ships in the (public) binary.
  DIRECT_CODES: KVNamespace; // key = code, value = JSON {label, ...}; membership = valid
  DIRECT_URL: string; // secret: the Direct server base, e.g. https://tunnel.ccy.dev
  DIRECT_SECRET: string; // secret: the Direct chisel auth (user:pass)
}

const CF_API = "https://api.cloudflare.com/client/v4";

interface ProvisionReq {
  deviceId: string; // stable random id the CLI persists per Mac
  name?: string; // display label (the Mac's hostname)
}

interface TunnelRecord {
  tunnelId: string;
  label: string; // the random subdomain label
  hostname: string; // "<label>.gtmux.ccy.dev"
}

export default {
  async fetch(req: Request, env: Env): Promise<Response> {
    const url = new URL(req.url);
    if (req.method === "GET" && url.pathname === "/health") {
      return json({ ok: true });
    }
    if (req.method === "POST" && url.pathname === "/provision") {
      return provision(req, env);
    }
    if (req.method === "POST" && url.pathname === "/direct/redeem") {
      return redeemDirect(req, env);
    }
    return json({ error: "not found" }, 404);
  },

  // Daily reaper (cron in wrangler.toml): delete inactive/never-connected tunnels +
  // their DNS so abuse (mass-create via the binary-extractable REG_SECRET) can't
  // accumulate, and refresh the global active count so its ceiling self-corrects.
  async scheduled(_event: ScheduledEvent, env: Env, ctx: ExecutionContext): Promise<void> {
    ctx.waitUntil(reapTunnels(env));
  },
};

async function provision(req: Request, env: Env): Promise<Response> {
  // Soft gate: the CLI sends a shared registration secret. Not a hard guarantee
  // (it ships in the binary), but it keeps casual abuse off the endpoint; pair
  // with a KV-backed cap + unused-tunnel reaping (TODO) for real protection.
  if (req.headers.get("x-gtmux-reg") !== env.REG_SECRET) {
    return json({ error: "unauthorized" }, 401);
  }

  let body: ProvisionReq;
  try {
    body = (await req.json()) as ProvisionReq;
  } catch {
    return json({ error: "bad json" }, 400);
  }
  const deviceId = (body.deviceId || "").trim();
  if (!/^[a-zA-Z0-9_-]{16,128}$/.test(deviceId)) {
    return json({ error: "bad deviceId" }, 400);
  }
  const name = (body.name || "Mac").slice(0, 64);

  // Idempotent: reuse the device's existing tunnel, just hand back a fresh token.
  // This path creates NOTHING, so it is never rate-capped — a legit Mac re-provisions
  // freely, only the FIRST provision for a new deviceId can be gated below.
  const existing = await env.TUNNELS.get<TunnelRecord>(deviceId, "json");
  if (existing) {
    const token = await getTunnelToken(env, existing.tunnelId);
    if (token) {
      return json({ hostname: existing.hostname, url: `https://${existing.hostname}`, token });
    }
    // Token fetch failed (tunnel deleted out-of-band?) — fall through and recreate.
  }

  // We're about to CREATE real Cloudflare resources. Cap it: REG_SECRET ships in the
  // public binary, so anyone can call this — bound mass-create abuse per IP + globally.
  const ip = req.headers.get("CF-Connecting-IP") || "unknown";
  const capped = await overCap(env, ip);
  if (capped) return capped;

  // 1) Create a remotely-managed named tunnel.
  const label = randomLabel();
  const tunnelName = `gtmux-${label}`;
  const created = await cf<{ id: string }>(env, "POST", `/accounts/${env.CF_ACCOUNT_ID}/cfd_tunnel`, {
    name: tunnelName,
    config_src: "cloudflare",
  });
  if (!created.ok || !created.result) {
    return json({ error: "tunnel create failed", detail: created.errors }, 502);
  }
  const tunnelId = created.result.id;
  // Single-level host so the zone's free Universal SSL (*.ccy.dev) covers it —
  // a 3rd-level *.gtmux.ccy.dev would need paid Advanced Cert Manager. The
  // `gtmux-` prefix keeps the namespace.
  const hostname = `gtmux-${label}.${env.ZONE_NAME}`;

  // 2) Point the tunnel's ingress at the Mac's local gtmux serve.
  const cfg = await cf(env, "PUT", `/accounts/${env.CF_ACCOUNT_ID}/cfd_tunnel/${tunnelId}/configurations`, {
    config: {
      ingress: [
        { hostname, service: env.LOCAL_SERVICE },
        { service: "http_status:404" },
      ],
    },
  });
  if (!cfg.ok) {
    return json({ error: "ingress config failed", detail: cfg.errors }, 502);
  }

  // 3) Create the proxied DNS route: <label>.gtmux.ccy.dev -> <tunnelId>.cfargotunnel.com
  const dns = await cf(env, "POST", `/zones/${env.CF_ZONE_ID}/dns_records`, {
    type: "CNAME",
    name: hostname,
    content: `${tunnelId}.cfargotunnel.com`,
    proxied: true,
  });
  if (!dns.ok) {
    return json({ error: "dns route failed", detail: dns.errors }, 502);
  }

  // 4) Remember it for idempotent re-provision, count it against the caps, then return
  //    the connector token.
  const rec: TunnelRecord = { tunnelId, label, hostname };
  await env.TUNNELS.put(deviceId, JSON.stringify(rec));
  await bumpCap(env, ip);

  const token = await getTunnelToken(env, tunnelId);
  if (!token) {
    return json({ error: "token fetch failed" }, 502);
  }
  return json({ hostname, url: `https://${hostname}`, token });
}

async function getTunnelToken(env: Env, tunnelId: string): Promise<string | null> {
  const r = await cf<string>(env, "GET", `/accounts/${env.CF_ACCOUNT_ID}/cfd_tunnel/${tunnelId}/token`);
  return r.ok && typeof r.result === "string" ? r.result : null;
}

// --- abuse caps ----------------------------------------------------------------
// KV counters (approximate is fine for a soft gate). Keys use a "cap:" prefix, which
// is disjoint from deviceId keys (deviceId = [A-Za-z0-9_-], no colon), so they share
// the TUNNELS namespace safely.

function num(v: string | undefined, dflt: number): number {
  const n = Number(v);
  return Number.isFinite(n) && n > 0 ? n : dflt;
}

// overCap returns a 429 Response when this IP's 24h new-tunnel count or the global
// active count is at its ceiling, else null. ONLY the create path calls it (idempotent
// re-provision is free), so a legit one-tunnel-per-Mac user never hits it.
async function overCap(env: Env, ip: string): Promise<Response | null> {
  const ipCount = Number((await env.TUNNELS.get(`cap:ip:${ip}`)) ?? "0") || 0;
  if (ipCount >= num(env.PROVISION_IP_CAP, 5)) {
    return json({ error: "rate limited: too many new tunnels from your network today" }, 429);
  }
  const active = Number((await env.TUNNELS.get("cap:active")) ?? "0") || 0;
  if (active >= num(env.PROVISION_GLOBAL_CAP, 500)) {
    return json({ error: "tunnel service at capacity, try again later" }, 429);
  }
  return null;
}

// bumpCap records one newly-created tunnel: a per-IP 24h counter + the global active
// count. Best-effort (KV is eventually consistent; the daily reaper resets cap:active
// to the true count, so drift self-corrects).
async function bumpCap(env: Env, ip: string): Promise<void> {
  const ipKey = `cap:ip:${ip}`;
  const ipCount = Number((await env.TUNNELS.get(ipKey)) ?? "0") || 0;
  await env.TUNNELS.put(ipKey, String(ipCount + 1), { expirationTtl: 86400 });
  const active = Number((await env.TUNNELS.get("cap:active")) ?? "0") || 0;
  await env.TUNNELS.put("cap:active", String(active + 1));
}

// --- reaper --------------------------------------------------------------------
interface CFTunnel {
  id: string;
  name: string;
  created_at: string; // ISO
  conns_active_at?: string | null; // ISO of the last active connection; null if never connected
}

// shouldReap: reap a `gtmux-` tunnel that either NEVER connected and is older than
// neverConnectedH (abuse junk — the mass-create attacker never runs cloudflared), OR
// whose last connection was more than idleDays ago (a truly abandoned real tunnel).
// Pure, so it is unit-tested without the CF API.
export function shouldReap(t: CFTunnel, nowMs: number, neverConnectedH: number, idleDays: number): boolean {
  if (!t.name?.startsWith("gtmux-")) return false;
  if (!t.conns_active_at) {
    const createdMs = Date.parse(t.created_at);
    return Number.isFinite(createdMs) && nowMs - createdMs > neverConnectedH * 3600_000;
  }
  const lastMs = Date.parse(t.conns_active_at);
  return Number.isFinite(lastMs) && nowMs - lastMs > idleDays * 86400_000;
}

async function reapTunnels(env: Env): Promise<void> {
  const neverH = num(env.REAP_NEVER_CONNECTED_H, 24);
  const idleDays = num(env.REAP_IDLE_DAYS, 90);
  const now = Date.now();
  let page = 1;
  let active = 0;
  for (let guard = 0; guard < 100; guard++) {
    const list = await cf<CFTunnel[]>(
      env, "GET",
      `/accounts/${env.CF_ACCOUNT_ID}/cfd_tunnel?is_deleted=false&per_page=100&page=${page}`,
    );
    if (!list.ok || !list.result || list.result.length === 0) break;
    for (const t of list.result) {
      if (!t.name?.startsWith("gtmux-")) continue;
      if (shouldReap(t, now, neverH, idleDays)) {
        await deleteTunnelAndDNS(env, t);
      } else {
        active++;
      }
    }
    if (list.result.length < 100) break;
    page++;
  }
  // Refresh the global active count so the create-path ceiling self-corrects.
  await env.TUNNELS.put("cap:active", String(active));
}

// deleteTunnelAndDNS removes a tunnel's DNS CNAME, then its connections, then the
// tunnel (CF refuses to delete a tunnel that still has live connections). Best-effort:
// one failure must not abort the whole sweep — the next run retries.
async function deleteTunnelAndDNS(env: Env, t: CFTunnel): Promise<void> {
  try {
    const host = `${t.name}.${env.ZONE_NAME}`;
    const rec = await cf<Array<{ id: string }>>(
      env, "GET",
      `/zones/${env.CF_ZONE_ID}/dns_records?type=CNAME&name=${encodeURIComponent(host)}`,
    );
    if (rec.ok && rec.result) {
      for (const r of rec.result) {
        await cf(env, "DELETE", `/zones/${env.CF_ZONE_ID}/dns_records/${r.id}`);
      }
    }
    await cf(env, "DELETE", `/accounts/${env.CF_ACCOUNT_ID}/cfd_tunnel/${t.id}/connections`);
    await cf(env, "DELETE", `/accounts/${env.CF_ACCOUNT_ID}/cfd_tunnel/${t.id}`);
  } catch {
    // best-effort; the next sweep retries
  }
}

interface CFResp<T> {
  ok: boolean;
  result?: T;
  errors?: unknown;
}

async function cf<T>(env: Env, method: string, path: string, body?: unknown): Promise<CFResp<T>> {
  const res = await fetch(CF_API + path, {
    method,
    headers: {
      Authorization: `Bearer ${env.CF_API_TOKEN}`,
      "Content-Type": "application/json",
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  const data = (await res.json()) as { success: boolean; result?: T; errors?: unknown };
  return { ok: res.ok && data.success, result: data.result, errors: data.errors };
}

interface RedeemReq {
  code: string; // the Direct access code the user bought/received
  deviceId?: string; // optional: for redemption bookkeeping
}

// redeemDirect validates a paid Direct code against the DIRECT_CODES KV and, only on
// a hit, returns the Direct server config ({url, secret}). The client writes that to
// ~/.config/gtmux/selftunnel.conf and Direct works via the normal self-tunnel path.
// The config is NEVER in the client binary — the gate + the secret live only here, so
// the repo can stay fully public. Revoke a code by deleting its KV key.
async function redeemDirect(req: Request, env: Env): Promise<Response> {
  let body: RedeemReq;
  try {
    body = (await req.json()) as RedeemReq;
  } catch {
    return json({ error: "bad request" }, 400);
  }
  const code = (body.code || "").trim();
  // Codes are opaque tokens we mint; keep the format tight so junk never hits KV.
  if (!/^[A-Za-z0-9-]{8,64}$/.test(code)) {
    return json({ error: "invalid code" }, 403);
  }
  const rec = await env.DIRECT_CODES.get(code);
  if (rec === null) {
    return json({ error: "invalid or revoked code" }, 403);
  }
  if (!env.DIRECT_URL || !env.DIRECT_SECRET) {
    return json({ error: "direct not configured" }, 503);
  }
  // Best-effort redemption bookkeeping (last device + count), non-fatal on failure.
  try {
    const meta = rec ? JSON.parse(rec) : {};
    meta.redemptions = (meta.redemptions || 0) + 1;
    meta.lastDevice = body.deviceId || meta.lastDevice || "";
    await env.DIRECT_CODES.put(code, JSON.stringify(meta));
  } catch {
    // leave the record as-is; the code is still valid
  }
  return json({ url: env.DIRECT_URL, secret: env.DIRECT_SECRET });
}

// randomLabel returns an unguessable DNS label (lowercase base32-ish, 10 chars).
function randomLabel(): string {
  const bytes = new Uint8Array(8);
  crypto.getRandomValues(bytes);
  const alphabet = "abcdefghijklmnopqrstuvwxyz234567";
  let out = "";
  for (const b of bytes) out += alphabet[b % 32];
  return out;
}

function json(obj: unknown, status = 200): Response {
  return new Response(JSON.stringify(obj), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
