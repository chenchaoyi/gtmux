// gtmux tunnel control-plane Worker.
import { redeem, move, authfile, authfileClaim, AUTHFILE_HEADER, loadRegistry, readRegistry, loadServers, offered, serverByToken, serverOf } from "./direct.ts";
//
// One endpoint that matters: POST /provision. It idempotently creates (or reuses)
// a Cloudflare *named* tunnel for the caller's Mac plus a
// `gtmux-<label>.<ZONE_NAME>` hostname (a random label, kept for the device identity), and returns the connector token the Mac runs
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
  ZONE_NAME: string; // e.g. "ccy.dev"; hostnames are gtmux-<label>.<ZONE_NAME>
  LOCAL_SERVICE: string; // e.g. "http://localhost:8765"
  CF_ACCOUNT_ID: string;
  CF_ZONE_ID: string;
  // Abuse caps + reaper thresholds (strings from wrangler [vars]; parsed with defaults).
  PROVISION_IP_CAP?: string;
  PROVISION_GLOBAL_CAP?: string;
  REAP_NEVER_CONNECTED_H?: string;
  REAP_IDLE_DAYS?: string;
  // Paid "Direct" tunnel unlock. A code the user bought/received is validated HERE
  // (not in the open client), and a valid one mints that DEVICE its own chisel account
  // (src/direct.ts). There is no shared Direct secret any more.
  DIRECT_CODES: KVNamespace; // codes (key = code, value = JSON {label, ...}) + the account registry
  DIRECT_URL: string; // secret: the Direct server base used when no server list is configured
  DIRECT_SYNC_TOKEN?: string; // secret: what the ONE legacy Direct server presents to fetch its authfile
  DIRECT_DEVICES_PER_CODE?: string; // devices one code may mint accounts for (default 3)
}

const CF_API = "https://api.cloudflare.com/client/v4";

interface ProvisionReq {
  deviceId: string; // stable random id the CLI persists per Mac
  name?: string; // display label (the Mac's hostname)
  force?: boolean; // repair the registered tunnel in place, never replace it
  recover?: boolean; // repair first; replace only a confirmed missing/deleted tunnel
}

export interface TunnelRecord {
  tunnelId: string;
  label: string; // the random subdomain label
  hostname: string; // "gtmux-<label>.ccy.dev"
  reaped?: boolean;
  pending?: { name: string; id?: string; counted?: boolean };
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
    if (req.method === "POST" && url.pathname === "/direct/servers") {
      return directServers(req, env);
    }
    if (req.method === "POST" && url.pathname === "/direct/move") {
      return directMove(req, env);
    }
    if (req.method === "GET" && url.pathname === "/direct/authfile") {
      return directAuthfile(req, env);
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
  const deviceId = typeof body?.deviceId === "string" ? body.deviceId.trim() : "";
  if (!/^[a-zA-Z0-9_-]{16,128}$/.test(deviceId)) {
    return json({ error: "bad deviceId" }, 400);
  }
  if (body.name !== undefined && typeof body.name !== "string") return json({ error: "bad name" }, 400);
  if (body.force !== undefined && typeof body.force !== "boolean") {
    return json({ error: "bad force" }, 400);
  }
  if ((body.recover !== undefined && typeof body.recover !== "boolean") || (body.force && body.recover)) {
    return json({ error: "bad recovery mode" }, 400);
  }

  const existing = await env.TUNNELS.get<TunnelRecord>(deviceId, "json");
  if ((body.force || body.recover) && !existing) {
    return json({ error: "no existing Standard tunnel to repair; enable it without --force first" }, 409);
  }
  if (existing && !validRecord(existing, env)) {
    return json({ error: "invalid registered Standard identity" }, 409);
  }
  if (existing?.pending && body.force) {
    return json({ error: "replacement pending; use connection recovery to resume it" }, 409);
  }
  if (body.force && existing?.reaped) {
    return json({ error: "registered tunnel was reclaimed; use connection recovery" }, 409);
  }
  let confirmedMissing = existing?.reaped === true;
  if (existing?.tunnelId && body.recover && !existing.pending) {
    const tunnel = await cf<CFTunnel>(env, "GET", tunnelPath(env, existing.tunnelId));
    confirmedMissing = isMissing(tunnel, existing.tunnelId);
    if (!confirmedMissing && (!tunnel.ok || !tunnel.result)) {
      return json({ error: "could not verify the existing tunnel; try again later" }, 502);
    }
  }
  if (existing?.tunnelId && !existing.pending && !confirmedMissing) {
    const token = await getTunnelToken(env, existing.tunnelId);
    if (token) {
      if (body.force || body.recover) {
        const failed = await repairTunnelRoute(env, existing);
        if (failed) return failed;
      }
      return json({ hostname: existing.hostname, url: `https://${existing.hostname}`, token,
        ...(body.force ? { repaired: true } : {}),
        ...(body.recover ? { recovered: true, addressPreserved: true, replaced: false } : {}) });
    }
    if (body.force) return json({ error: "existing tunnel token unavailable; repair did not create a replacement" }, 502);
    const tunnel = await cf<CFTunnel>(env, "GET", tunnelPath(env, existing.tunnelId));
    if (!isMissing(tunnel, existing.tunnelId)) {
      return json({ error: "could not verify the existing tunnel; try again later" }, 502);
    }
  }

  if (existing?.tunnelId && !existing.pending) await markReaped(env, deviceId, existing);

  const ip = req.headers.get("CF-Connecting-IP") || "unknown";
  const rec: TunnelRecord = existing || (() => {
    const label = randomLabel();
    return { tunnelId: "", label, hostname: `gtmux-${label}.${env.ZONE_NAME}` };
  })();
  if (!rec.pending) {
    const capped = await overCap(env, ip, !!rec.tunnelId);
    if (capped) return capped;
    // Journal BEFORE create. Keep the old instance and stable identity until commit.
    rec.pending = { name: `gtmux-${rec.label}-${crypto.randomUUID()}` };
    await env.TUNNELS.put(deviceId, JSON.stringify(rec));
  }
  const pending = rec.pending;
  if (!pending.id) {
    // A previous POST may have succeeded even when its response was lost.
    const found = await cf<Array<{ id: string; name: string }>>(env, "GET",
      `/accounts/${env.CF_ACCOUNT_ID}/cfd_tunnel?is_deleted=false&name=${encodeURIComponent(pending.name)}&per_page=100`);
    if (!found.ok || !Array.isArray(found.result)) return json({ error: "could not resolve pending tunnel creation" }, 502);
    const matches = found.result.filter(t => t.name === pending.name);
    if (matches.length > 1) return json({ error: "ambiguous pending tunnel creation" }, 409);
    if (matches[0]) {
      if (!validTunnelID(matches[0].id)) return json({ error: "invalid pending tunnel response" }, 502);
      pending.id = matches[0].id;
    }
    else {
      const capped = await overCap(env, ip, !!rec.tunnelId);
      if (capped) return capped;
      const created = await cf<{ id: string }>(env, "POST", `/accounts/${env.CF_ACCOUNT_ID}/cfd_tunnel`, {
        name: pending.name, config_src: "cloudflare",
      });
      if (!created.ok || !validTunnelID(created.result?.id)) return json({ error: "tunnel create failed" }, 502);
      pending.id = created.result.id;
    }
    await env.TUNNELS.put(deviceId, JSON.stringify(rec));
  }
  if (!pending.counted) {
    // Soft KV quota bookkeeping must not strand an already created instance.
    // Persist the once-only marker first; the scheduled account count reconciles
    // interrupted/contended global writes without blocking connection recovery.
    pending.counted = true;
    await env.TUNNELS.put(deviceId, JSON.stringify(rec));
    try { await bumpCap(env, ip, !!rec.tunnelId); }
    catch { console.warn(JSON.stringify({ event: "standard.cap_reconcile_needed" })); }
  }
  // Token and ingress must be ready before changing the public route.
  const token = await getTunnelToken(env, pending.id);
  if (!token) {
    const state = await cf<CFTunnel>(env, "GET", tunnelPath(env, pending.id));
    if (isMissing(state, pending.id)) {
      // Even a pending replacement can be removed out of band. Retire only after
      // proof of absence, and journal a fresh operation for the next retry.
      rec.pending = { name: `gtmux-${rec.label}-${crypto.randomUUID()}` };
      await env.TUNNELS.put(deviceId, JSON.stringify(rec));
      if (pending.counted) await decrementCap(env);
    }
    return json({ error: "token fetch failed; recovery can resume" }, 502);
  }
  const failed = await repairTunnelRoute(env, { ...rec, tunnelId: pending.id }, rec.tunnelId);
  if (failed) return failed;
  const committed: TunnelRecord = { tunnelId: pending.id, label: rec.label, hostname: rec.hostname };
  await env.TUNNELS.put(deviceId, JSON.stringify(committed));
  return json({ hostname: rec.hostname, url: `https://${rec.hostname}`, token,
    ...(body.recover ? { recovered: true, addressPreserved: true, replaced: true } : {}) });
}

function validTunnelID(id: unknown): id is string {
  return typeof id === "string" && /^[a-zA-Z0-9_-]{1,128}$/.test(id);
}
function validRecord(rec: TunnelRecord, env: Env): boolean {
  return typeof rec.label === "string" && /^[a-z0-9]+$/.test(rec.label) &&
    rec.hostname === `gtmux-${rec.label}.${env.ZONE_NAME}` &&
    (validTunnelID(rec.tunnelId) || rec.tunnelId === "" && !!rec.pending) &&
    (!rec.pending || typeof rec.pending.name === "string" &&
      rec.pending.name.startsWith(`gtmux-${rec.label}-`) &&
      (rec.pending.id === undefined || validTunnelID(rec.pending.id)));
}

function tunnelPath(env: Env, id: string): string {
  return `/accounts/${env.CF_ACCOUNT_ID}/cfd_tunnel/${encodeURIComponent(id)}`;
}
function isMissing(r: CFResp<CFTunnel>, expectedId: string): boolean {
  return r.status === 404 || !!(r.ok && r.result?.id === expectedId && typeof r.result.deleted_at === "string" &&
    Number.isFinite(Date.parse(r.result.deleted_at)) && Date.parse(r.result.deleted_at) <= Date.now());
}

async function getTunnelToken(env: Env, tunnelId: string): Promise<string | null> {
  const r = await cf<string>(env, "GET", `${tunnelPath(env, tunnelId)}/token`);
  return r.ok && typeof r.result === "string" ? r.result : null;
}

function configureIngress(env: Env, tunnelId: string, hostname: string) {
  return cf(env, "PUT", `${tunnelPath(env, tunnelId)}/configurations`, {
    config: { ingress: [
      { hostname, service: env.LOCAL_SERVICE },
      { service: "http_status:404" },
    ] },
  });
}

interface DNSRecord {
  id: string;
  type: string;
  name: string;
  content: string;
  proxied?: boolean;
}

// Repair only the hostname already registered to this device. Validate DNS ownership
// before changing ingress; a foreign record is a conflict, not permission to replace it.
async function repairTunnelRoute(env: Env, rec: TunnelRecord, previousId = ""): Promise<Response | null> {
  if (rec.hostname !== `gtmux-${rec.label}.${env.ZONE_NAME}`) {
    return json({ error: "registered hostname is outside the managed namespace" }, 409);
  }
  const path = `/zones/${env.CF_ZONE_ID}/dns_records`;
  const found = await cf<DNSRecord[]>(env, "GET", `${path}?name=${encodeURIComponent(rec.hostname)}&per_page=100`);
  if (!found.ok || !Array.isArray(found.result)) {
    return json({ error: "dns lookup failed", detail: found.errors }, 502);
  }
  const records = found.result;
  if (records.length > 1 || records.some(r => r.name !== rec.hostname || r.type !== "CNAME" ||
    ![rec.tunnelId, previousId].filter(Boolean).some(id => r.content === `${id}.cfargotunnel.com`))) {
    return json({ error: "dns route conflicts with an unrelated record; repair left it intact" }, 409);
  }
  const cfg = await configureIngress(env, rec.tunnelId, rec.hostname);
  if (!cfg.ok) return json({ error: "ingress repair failed", detail: cfg.errors }, 502);
  const route = { type: "CNAME", name: rec.hostname, content: `${rec.tunnelId}.cfargotunnel.com`, proxied: true };
  const current = records[0];
  if (!current || current.content !== route.content || current.proxied !== true) {
    const dns = await cf(env, current ? "PUT" : "POST", current ? `${path}/${current.id}` : path, route);
    if (!dns.ok) return json({ error: "dns repair failed", detail: dns.errors }, 502);
  }
  return null;
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
async function overCap(env: Env, ip: string, registeredDevice = false): Promise<Response | null> {
  const ipCount = Number((await env.TUNNELS.get(`cap:ip:${ip}`)) ?? "0") || 0;
  if (!registeredDevice && ipCount >= num(env.PROVISION_IP_CAP, 5)) {
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
async function bumpCap(env: Env, ip: string, registeredDevice = false): Promise<void> {
  const ipKey = `cap:ip:${ip}`;
  const ipCount = Number((await env.TUNNELS.get(ipKey)) ?? "0") || 0;
  if (!registeredDevice) await env.TUNNELS.put(ipKey, String(ipCount + 1), { expirationTtl: 86400 });
  const active = Number((await env.TUNNELS.get("cap:active")) ?? "0") || 0;
  await env.TUNNELS.put("cap:active", String(active + 1));
}

// --- reaper --------------------------------------------------------------------
export interface CFTunnel {
  id: string;
  name: string;
  created_at: string;
  status?: string;
  deleted_at?: string | null;
  conns_active_at?: string | null;
  conns_inactive_at?: string | null;
  connections?: unknown[];
}

export function shouldReap(t: CFTunnel, nowMs: number, neverConnectedH: number, idleDays: number): boolean {
  if (!t.name?.startsWith("gtmux-") || t.deleted_at || t.conns_active_at !== null ||
    !Array.isArray(t.connections) || t.connections.length) return false;
  const created = Date.parse(t.created_at);
  if (!Number.isFinite(created) || created > nowMs) return false;
  if (t.status === "inactive" && t.conns_inactive_at === null) {
    return nowMs - created > neverConnectedH * 3600_000;
  }
  if (t.status !== "down" || typeof t.conns_inactive_at !== "string") return false;
  const inactive = Date.parse(t.conns_inactive_at);
  return Number.isFinite(inactive) && inactive >= created && inactive <= nowMs &&
    nowMs - inactive > idleDays * 86400_000;
}

// Called ONLY under the same device coordinator as provisioning. An old list or
// arbitrary prefix cannot authorize deletion. Keep identity even after reclamation.
export async function reapDevice(env: Env, deviceId: string): Promise<boolean> {
  const rec = await env.TUNNELS.get<TunnelRecord>(deviceId, "json");
  if (!rec || !validRecord(rec, env)) return true; // unknown counts conservatively
  if (rec.pending && !rec.pending.id) {
    // An uncertain create response can leave an instance known only by its opaque
    // operation name. Resolve that exact name; never infer ownership from a prefix.
    const found = await cf<Array<{ id: string; name: string }>>(env, "GET",
      `/accounts/${env.CF_ACCOUNT_ID}/cfd_tunnel?is_deleted=false&name=${encodeURIComponent(rec.pending.name)}&per_page=100`);
    if (!found.ok || !Array.isArray(found.result)) return true;
    const matches = found.result.filter(t => t.name === rec.pending!.name);
    if (matches.length !== 1) return matches.length > 0;
    if (!validTunnelID(matches[0].id)) return true;
    rec.pending.id = matches[0].id;
    await env.TUNNELS.put(deviceId, JSON.stringify(rec));
  }
  const instanceId = rec.pending?.id || rec.tunnelId;
  if (!instanceId) return false;
  const current = await cf<CFTunnel>(env, "GET", tunnelPath(env, instanceId));
  if (isMissing(current, instanceId)) { await markReaped(env, deviceId, rec); return false; }
  if (!current.ok || !current.result || current.result.id !== instanceId) return true;
  if (!shouldReap(current.result, Date.now(), num(env.REAP_NEVER_CONNECTED_H, 24), num(env.REAP_IDLE_DAYS, 90))) return true;
  // Never clear /connections: provider live-connection protection must remain intact.
  const removed = await cf(env, "DELETE", tunnelPath(env, instanceId));
  if (!removed.ok && removed.status !== 404) return true;
  await markReaped(env, deviceId, rec);
  const path = `/zones/${env.CF_ZONE_ID}/dns_records`;
  const dns = await cf<DNSRecord[]>(env, "GET", `${path}?name=${encodeURIComponent(rec.hostname)}&per_page=100`);
  if (dns.ok && Array.isArray(dns.result)) {
    for (const r of dns.result) {
      if (r.name === rec.hostname && r.type === "CNAME" && r.content === `${instanceId}.cfargotunnel.com`) {
        await cf(env, "DELETE", `${path}/${r.id}`);
      }
    }
  }
  console.log(JSON.stringify({ event: "standard.reclaimed", reason: current.result.status === "inactive" ? "never_connected" : "idle" }));
  return false;
}

async function markReaped(env: Env, deviceId: string, rec: TunnelRecord): Promise<void> {
  if (rec.pending) {
    const counted = rec.pending.counted;
    // Retire the instance, not the unfinished device identity/address. A returning
    // Mac resumes a fresh operation; no stale pending token is retried forever.
    rec.pending = { name: `gtmux-${rec.label}-${crypto.randomUUID()}` };
    await env.TUNNELS.put(deviceId, JSON.stringify(rec));
    if (counted) await decrementCap(env);
    return;
  }
  if (rec.reaped) return;
  // The marker prevents repeated decrements on an already deleted tombstone.
  rec.reaped = true;
  await env.TUNNELS.put(deviceId, JSON.stringify(rec));
  await decrementCap(env);
}

async function decrementCap(env: Env): Promise<void> {
  try {
    const active = Number(await env.TUNNELS.get("cap:active")) || 0;
    await env.TUNNELS.put("cap:active", String(Math.max(0, active - 1)));
  } catch { console.warn(JSON.stringify({ event: "standard.cap_reconcile_needed" })); }
}

// Read-only account reconciliation preserves the old quota's drift correction.
// Prefix matching authorizes counting, NEVER deletion. An incomplete/error scan
// cannot publish a false zero/lower total. Actual deletion remains device-owned.
export async function reconcileActiveCount(env: Env): Promise<void> {
  let count = 0;
  for (let page = 1; page <= 10; page++) {
    const list = await cf<Array<{ name?: string }>>(env, "GET",
      `/accounts/${env.CF_ACCOUNT_ID}/cfd_tunnel?is_deleted=false&per_page=100&page=${page}`);
    if (!list.ok || !Array.isArray(list.result)) return;
    if (list.result.some(t => typeof t.name !== "string")) return;
    count += list.result.filter(t => t.name?.startsWith("gtmux-")).length;
    if (list.result.length < 100) {
      await env.TUNNELS.put("cap:active", String(count));
      return;
    }
  }
  // Safe lower bound only when it already closes the creation gate.
  if (count >= num(env.PROVISION_GLOBAL_CAP, 500)) {
    const previous = Number(await env.TUNNELS.get("cap:active")) || 0;
    await env.TUNNELS.put("cap:active", String(Math.max(previous, count)));
  }
}

// The deployed entry point owns coordinated discovery; this Node-testable module
// deliberately cannot run the legacy uncoordinated account-wide reaper.
async function reapTunnels(_env: Env): Promise<void> {
  throw new Error("Standard reaping requires the device coordinator entry point");
}

interface CFResp<T> {
  ok: boolean;
  status: number;
  result?: T;
  errors?: unknown;
}

async function cf<T>(env: Env, method: string, path: string, body?: unknown): Promise<CFResp<T>> {
  try {
    const res = await fetch(CF_API + path, {
      method, signal: AbortSignal.timeout(10_000),
      headers: { Authorization: `Bearer ${env.CF_API_TOKEN}`, "Content-Type": "application/json" },
      body: body ? JSON.stringify(body) : undefined,
    });
    const data = await res.json() as { success?: boolean; result?: T; errors?: unknown };
    return { ok: res.ok && data.success === true, status: res.status, result: data.result, errors: data.errors };
  } catch {
    return { ok: false, status: 502 }; // never interpret transport/JSON failure as absence
  }
}

interface RedeemReq {
  code: string; // the Direct access code the user bought/received
  deviceId?: string; // the device the account is for; required (an old client may omit it)
  server?: string; // which Direct server, by id (from POST /direct/servers)
  region?: string; // or just a region preference, when the caller names no id
}

// redeemDirect trades a paid code + this device's id for the device's own chisel account
// and the reverse port assigned to it. See src/direct.ts.
async function redeemDirect(req: Request, env: Env): Promise<Response> {
  let body: RedeemReq;
  try {
    body = (await req.json()) as RedeemReq;
  } catch {
    return json({ error: "bad request" }, 400);
  }
  if (!env.DIRECT_URL) {
    return json({ error: "direct not configured" }, 503);
  }
  try {
    const r = await redeem(body.code, body.deviceId || "", {
      kv: env.DIRECT_CODES,
      url: env.DIRECT_URL,
      cap: num(env.DIRECT_DEVICES_PER_CODE, 3),
      now: Date.now(),
      server: body.server,
      region: body.region,
    });
    return json(r.body, r.status);
  } catch {
    return json({ error: "could not issue an account; try again" }, 503);
  }
}

// directServers lists the Direct servers this caller may use. The list is configuration
// (src/direct.ts), read at run time, so a server the operator adds today is selectable by
// installations that already exist — the point of the whole change. A code may be sent,
// and only then do servers reserved for that code appear; a device id, and the answer
// says which server that device is on.
async function directServers(req: Request, env: Env): Promise<Response> {
  let body: { code?: string; deviceId?: string } = {};
  try {
    body = (await req.json()) as { code?: string; deviceId?: string };
  } catch {
    body = {}; // an empty body is a fair question: "which servers are open to anyone?"
  }
  const servers = await loadServers(env.DIRECT_CODES, env.DIRECT_URL);
  let current: string | undefined;
  let code = (body.code || "").trim();
  if (body.deviceId) {
    const reg = await loadRegistry(env.DIRECT_CODES);
    const acct = reg.accounts[body.deviceId.trim()];
    if (acct) {
      current = serverOf(acct);
      if (!code) code = acct.code; // the device's own code, so its reserved servers show
    }
  }
  return json({ servers: offered(servers, code), current });
}

// directMove reassigns a device to another server, authenticated by the account that
// device already holds. See move() in src/direct.ts.
async function directMove(req: Request, env: Env): Promise<Response> {
  let body: { deviceId?: string; secret?: string; server?: string };
  try {
    body = (await req.json()) as { deviceId?: string; secret?: string; server?: string };
  } catch {
    return json({ error: "bad request" }, 400);
  }
  if (!body.server) return json({ error: "no server named" }, 400);
  try {
    const r = await move(body.deviceId || "", body.secret || "", {
      kv: env.DIRECT_CODES,
      url: env.DIRECT_URL,
      server: body.server,
    });
    return json(r.body, r.status);
  } catch {
    return json({ error: "could not move this device; try again" }, 503);
  }
}

// directAuthfile serves ONE Direct server's chisel authfile: the accounts assigned to
// THAT server, each allowed only its own reverse port. A server never receives the
// credentials of devices that are not on it, so a compromised server exposes only its own
// tenants. The token identifies which server is asking; the single legacy token still
// answers for the one server a deployment has before a list is configured.
async function directAuthfile(req: Request, env: Env): Promise<Response> {
  const got = (req.headers.get("Authorization") || "").replace(/^Bearer /, "");
  const servers = await loadServers(env.DIRECT_CODES, env.DIRECT_URL);
  const id = got ? serverByToken(servers, got, env.DIRECT_SYNC_TOKEN) : undefined;
  if (!id) {
    if (!env.DIRECT_SYNC_TOKEN && !servers.some((s) => s.sync)) {
      return json({ error: "direct sync not configured" }, 503);
    }
    return json({ error: "unauthorized" }, 401);
  }
  const { reg, present } = await readRegistry(env.DIRECT_CODES);
  const file = authfile(reg, id);
  const res = json(file);
  // Lets the server's sync tell "no accounts left" from a fault (see AUTHFILE_HEADER).
  const claim = authfileClaim(present, id, file);
  if (claim) res.headers.set(AUTHFILE_HEADER, claim);
  return res;
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
