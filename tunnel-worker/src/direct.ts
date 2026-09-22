// Direct: one chisel account per device, each allowed to bind only its own port.
//
// Every code used to redeem for the same thing: the one DIRECT_SECRET, a single chisel
// user shared by every buyer, which the server let bind ANY reverse port and open ANY
// forward tunnel. So one holder could take another Mac's /p<port> while it slept and
// receive that Mac's phone's bearer token, then drive the Mac with it; and could reach
// every loopback service on the VPS, Caddy's admin API included. Reproduced locally
// 2026-09-22. See openspec/changes/direct-per-device-accounts.
//
// Now a redeem mints (or returns) an account for THAT device, and the server's authfile,
// served from here, lets each account bind exactly `R:127.0.0.1:<its port>` and nothing
// else. Pure logic over a small KV interface, so it is tested without Cloudflare.

export const PORT_BASE = 20000;
export const PORT_SPAN = 40000;
/** The whole account registry lives under ONE key: the authfile is one read per sync. */
export const REGISTRY_KEY = "registry:v1";

export interface KV {
  get(key: string): Promise<string | null>;
  put(key: string, value: string): Promise<void>;
}

export interface Account {
  user: string;
  pass: string;
  port: number;
  code: string;
  at: number; // unix ms, when minted
}

export interface Registry {
  accounts: Record<string, Account>; // deviceId → its account
}

const DEVICE_ID = /^[a-zA-Z0-9_-]{16,128}$/;
const CODE = /^[A-Za-z0-9-]{8,64}$/;

let crcTable: Uint32Array | null = null;

/** crc32 is IEEE CRC-32 over the UTF-8 bytes: Go's crc32.ChecksumIEEE, which the client uses. */
export function crc32(s: string): number {
  if (!crcTable) {
    crcTable = new Uint32Array(256);
    for (let n = 0; n < 256; n++) {
      let c = n;
      for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
      crcTable[n] = c >>> 0;
    }
  }
  let crc = 0xffffffff;
  for (const b of new TextEncoder().encode(s)) crc = crcTable[(crc ^ b) & 0xff] ^ (crc >>> 8);
  return (crc ^ 0xffffffff) >>> 0;
}

/** The port a device derives for itself, the same formula the client has always used. */
export function preferredPort(deviceId: string): number {
  return PORT_BASE + (crc32(deviceId) % PORT_SPAN);
}

/**
 * assignPort keeps a device on the port it derives unless another device already holds
 * it, then walks to the next free one. Two devices on one port is exactly the exposure
 * this exists to remove, so a derived port is never shared.
 */
export function assignPort(reg: Registry, deviceId: string): number {
  const taken = new Set<number>();
  for (const [dev, a] of Object.entries(reg.accounts)) if (dev !== deviceId) taken.add(a.port);
  const start = preferredPort(deviceId) - PORT_BASE;
  for (let i = 0; i < PORT_SPAN; i++) {
    const p = PORT_BASE + ((start + i) % PORT_SPAN);
    if (!taken.has(p)) return p;
  }
  throw new Error("no free Direct port");
}

export async function loadRegistry(kv: KV): Promise<Registry> {
  const raw = await kv.get(REGISTRY_KEY);
  if (!raw) return { accounts: {} };
  const reg = JSON.parse(raw) as Registry;
  return { accounts: reg.accounts ?? {} };
}

function hex(bytes: number): string {
  const b = new Uint8Array(bytes);
  crypto.getRandomValues(b);
  return Array.from(b, (x) => x.toString(16).padStart(2, "0")).join("");
}

export interface RedeemDeps {
  kv: KV;
  url: string; // the Direct server base
  cap: number; // devices a code may mint accounts for
  now: number; // unix ms
}

export interface RedeemResult {
  status: number;
  body: Record<string, unknown>;
}

/**
 * redeem answers a code + device with that device's own account. A device that already
 * has one gets it back unchanged; a new device is refused once the code has minted
 * accounts for `cap` devices. The account is written before it is returned: credentials
 * the server will never learn would only fail later, and less clearly.
 */
export async function redeem(code: string, deviceId: string, d: RedeemDeps): Promise<RedeemResult> {
  code = (code || "").trim();
  deviceId = (deviceId || "").trim();
  if (!CODE.test(code)) return { status: 403, body: { error: "invalid code" } };
  if (!DEVICE_ID.test(deviceId)) {
    return { status: 400, body: { error: "this gtmux is too old to redeem Direct; update it first (gtmux update)" } };
  }
  const rec = await d.kv.get(code);
  if (rec === null) return { status: 403, body: { error: "invalid or revoked code" } };

  const reg = await loadRegistry(d.kv);
  let acct = reg.accounts[deviceId];
  if (!acct || acct.code !== code) {
    const devices = Object.entries(reg.accounts).filter(([dev, a]) => a.code === code && dev !== deviceId);
    if (devices.length >= d.cap) {
      return {
        status: 409,
        body: { error: `this code is already in use on ${devices.length} devices, the most it can be` },
      };
    }
    acct = acct
      ? { ...acct, code } // a device moving to another code keeps its account and port
      : { user: "d" + hex(8), pass: hex(16), port: assignPort(reg, deviceId), code, at: d.now };
    reg.accounts[deviceId] = acct;
    await d.kv.put(REGISTRY_KEY, JSON.stringify(reg));
  }

  // Bookkeeping on the code itself, as before: best-effort, never fatal.
  try {
    const meta = rec ? JSON.parse(rec) : {};
    meta.redemptions = (meta.redemptions || 0) + 1;
    meta.lastDevice = deviceId;
    await d.kv.put(code, JSON.stringify(meta));
  } catch {
    // the account is already written; the counter is not worth failing over
  }
  return { status: 200, body: { url: d.url, secret: `${acct.user}:${acct.pass}`, port: acct.port } };
}

/**
 * authfile is the chisel server's users file: each account may bind exactly its own
 * reverse port on loopback, which no forward tunnel matches either. If two accounts ever
 * hold one port (two redeems racing through eventually consistent KV), only the older
 * is emitted: a collision fails closed, never shared.
 */
export function authfile(reg: Registry): Record<string, string[]> {
  const byPort = new Map<number, [string, Account]>();
  for (const [dev, a] of Object.entries(reg.accounts)) {
    if (!Number.isInteger(a.port) || a.port < PORT_BASE || a.port >= PORT_BASE + PORT_SPAN) continue;
    if (!/^[0-9a-z]+$/.test(a.user) || !/^[0-9a-f]+$/.test(a.pass)) continue;
    const held = byPort.get(a.port);
    if (!held || a.at < held[1].at || (a.at === held[1].at && dev < held[0])) byPort.set(a.port, [dev, a]);
  }
  const out: Record<string, string[]> = {};
  for (const [, a] of byPort.values()) out[`${a.user}:${a.pass}`] = [`^R:127\\.0\\.0\\.1:${a.port}$`];
  return out;
}

/** revokeCode drops every account a code minted, so its devices stop connecting. */
export function revokeCode(reg: Registry, code: string): { reg: Registry; removed: number } {
  const accounts: Record<string, Account> = {};
  let removed = 0;
  for (const [dev, a] of Object.entries(reg.accounts)) {
    if (a.code === code) removed++;
    else accounts[dev] = a;
  }
  return { reg: { accounts }, removed };
}

/** sameSecret compares two strings in time independent of where they first differ. */
export function sameSecret(a: string, b: string): boolean {
  const x = new TextEncoder().encode(a);
  const y = new TextEncoder().encode(b);
  let diff = x.length ^ y.length;
  for (let i = 0; i < Math.max(x.length, y.length); i++) diff |= (x[i] ?? 0) ^ (y[i] ?? 0);
  return diff === 0;
}
