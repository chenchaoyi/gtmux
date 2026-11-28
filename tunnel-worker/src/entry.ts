import { DurableObject } from "cloudflare:workers";
import { MAX_JSON_BODY_BYTES, readJSONObject, jsonBodyStatus } from "../../shared/worker-json.mts";
import { REGISTRY_KEY, sameSecret, type Account, type Registry } from "./direct.ts";
import worker, { reconcileActiveCount, reapDevice, type Env, type TunnelRecord } from "./index.ts";

export interface CoordinatedEnv extends Env {
  STANDARD_DEVICES: DurableObjectNamespace<StandardDevice>;
  DIRECT_REGISTRY: DurableObjectNamespace<DirectRegistry>;
}

// One strongly consistent owner per device. The explicit queue covers network
// awaits, which Durable Objects' automatic storage gates alone do not serialize.
export class StandardDevice extends DurableObject<CoordinatedEnv> {
  private tail: Promise<void> = Promise.resolve();

  private serial<T>(work: () => Promise<T>): Promise<T> {
    const result = this.tail.then(work);
    this.tail = result.then(() => {}, () => {});
    return result;
  }

  private async deviceEnv(deviceId: string): Promise<Env> {
    let initialized = await this.ctx.storage.get<boolean>("initialized");
    if (!initialized) {
      const legacy = await this.env.TUNNELS.get<TunnelRecord>(deviceId, "json");
      // Store the import marker with the value: no stale re-import after restart.
      await this.ctx.storage.put({ initialized: true, ...(legacy ? { record: JSON.stringify(legacy), shadow: JSON.stringify(legacy) } : {}) });
      initialized = true;
    }
    await this.ctx.storage.put("deviceId", deviceId);
    const kv = this.env.TUNNELS;
    return { ...this.env, TUNNELS: {
      get: async (key: string, format?: string) => {
        if (key !== deviceId) return kv.get(key);
        const value = await this.ctx.storage.get<string>("record");
        return value === undefined ? null : format === "json" ? JSON.parse(value) : value;
      },
      put: async (key: string, value: string, options?: KVNamespacePutOptions) => {
        if (key === deviceId) {
          await this.ctx.storage.transaction(async tx => {
            await tx.put("record", value);
            if (await tx.getAlarm() === null) await tx.setAlarm(Date.now() + 5000);
          });
        } else await kv.put(key, value, options);
      },
    } as unknown as KVNamespace };
  }

  private async flushShadow(deviceId: string): Promise<void> {
    const value = await this.ctx.storage.get<string>("record");
    const shadow = await this.ctx.storage.get<string>("shadow");
    if (!value || value === shadow) { await this.ctx.storage.deleteAlarm(); return; }
    const lastWrite = await this.ctx.storage.get<number>("shadowAt") || 0;
    if (Date.now() - lastWrite < 1100) { await this.ctx.storage.setAlarm(lastWrite + 1100); return; }
    try {
      await this.env.TUNNELS.put(deviceId, value);
      await this.ctx.storage.put({ shadow: value, shadowAt: Date.now() });
      await this.ctx.storage.deleteAlarm();
    } catch {
      // KV's one-write/second/key limit must not roll back strongly consistent state.
      await this.ctx.storage.setAlarm(Date.now() + 5000);
    }
  }

  provision(deviceId: string, body: string, ip: string): Promise<Response> {
    return this.serial(async () => {
      try {
        return await worker.fetch(new Request("https://coordinator/provision", {
          method: "POST", headers: { "x-gtmux-reg": this.env.REG_SECRET, "CF-Connecting-IP": ip }, body,
        }), await this.deviceEnv(deviceId));
      } finally { await this.flushShadow(deviceId); }
    });
  }

  reap(deviceId: string): Promise<boolean> {
    return this.serial(async () => {
      try { return await reapDevice(await this.deviceEnv(deviceId), deviceId); }
      finally { await this.flushShadow(deviceId); }
    });
  }

  alarm(): Promise<void> {
    return this.serial(async () => {
      const deviceId = await this.ctx.storage.get<string>("deviceId");
      if (deviceId) await this.flushShadow(deviceId);
    });
  }
}

// Global SQLite coordinator for Direct per-device accounts (registry:v1).
// Eliminates cross-PoP KV read-modify-write overwrites and 1-write/sec KV 429 failures.
export class DirectRegistry extends DurableObject<CoordinatedEnv> {
  private tail: Promise<void> = Promise.resolve();

  private serial<T>(work: () => Promise<T>): Promise<T> {
    const result = this.tail.then(work);
    this.tail = result.then(() => {}, () => {});
    return result;
  }

  private ensureSchema(): void {
    this.ctx.storage.sql.exec("CREATE TABLE IF NOT EXISTS direct_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)");
    this.ctx.storage.sql.exec("CREATE TABLE IF NOT EXISTS direct_accounts (device_id TEXT PRIMARY KEY, account_json TEXT NOT NULL)");
  }

  private metaGet(key: string): string | null {
    const rows = [...this.ctx.storage.sql.exec<{ value: string }>("SELECT value FROM direct_meta WHERE key = ?", key)];
    return rows[0]?.value ?? null;
  }

  private async ensureHydrated(): Promise<void> {
    this.ensureSchema();
    if (this.metaGet("initialized") === "1") return;
    const legacy = await this.env.DIRECT_CODES.get(REGISTRY_KEY);
    let accounts: Record<string, Account> | null = null;
    if (legacy) {
      try {
        const parsed = JSON.parse(legacy) as { accounts?: unknown } | null;
        if (parsed && typeof parsed.accounts === "object" && parsed.accounts !== null && !Array.isArray(parsed.accounts)) {
          accounts = parsed.accounts as Record<string, Account>;
        }
      } catch {
        accounts = null;
      }
    }
    this.ctx.storage.transactionSync(() => {
      this.ctx.storage.sql.exec("INSERT OR REPLACE INTO direct_meta (key, value) VALUES ('initialized', '1')");
      this.ctx.storage.sql.exec("INSERT OR REPLACE INTO direct_meta (key, value) VALUES ('present', ?)", accounts !== null ? "1" : "0");
      if (accounts) {
        for (const [dev, acct] of Object.entries(accounts)) {
          this.ctx.storage.sql.exec("INSERT OR REPLACE INTO direct_accounts (device_id, account_json) VALUES (?, ?)", dev, JSON.stringify(acct));
        }
        this.ctx.storage.sql.exec("INSERT OR REPLACE INTO direct_meta (key, value) VALUES ('shadow', ?)", JSON.stringify({ accounts }));
      }
    });
  }

  private readRegistryJSON(): string | null {
    if (this.metaGet("present") !== "1") return null;
    const accounts: Record<string, Account> = {};
    for (const row of this.ctx.storage.sql.exec<{ device_id: string; account_json: string }>(
      "SELECT device_id, account_json FROM direct_accounts ORDER BY device_id",
    )) {
      accounts[row.device_id] = JSON.parse(row.account_json) as Account;
    }
    return JSON.stringify({ accounts });
  }

  private async writeRegistryJSON(raw: string): Promise<void> {
    const parsed = JSON.parse(raw) as Partial<Registry>;
    const accounts = parsed.accounts ?? {};
    this.ctx.storage.transactionSync(() => {
      this.ctx.storage.sql.exec("INSERT OR REPLACE INTO direct_meta (key, value) VALUES ('present', '1')");
      this.ctx.storage.sql.exec("DELETE FROM direct_accounts");
      for (const [dev, acct] of Object.entries(accounts)) {
        this.ctx.storage.sql.exec("INSERT INTO direct_accounts (device_id, account_json) VALUES (?, ?)", dev, JSON.stringify(acct));
      }
    });
    if ((await this.ctx.storage.getAlarm()) === null) {
      await this.ctx.storage.setAlarm(Date.now() + 5000);
    }
  }

  private directEnv(): Env {
    const kv = this.env.DIRECT_CODES;
    return {
      ...this.env,
      DIRECT_CODES: {
        get: async (key: string, format?: string) => {
          if (key !== REGISTRY_KEY) return format === "json" ? kv.get(key, "json") : kv.get(key);
          const raw = this.readRegistryJSON();
          return raw === null ? null : format === "json" ? JSON.parse(raw) : raw;
        },
        put: async (key: string, value: string, options?: KVNamespacePutOptions) => {
          if (key === REGISTRY_KEY) await this.writeRegistryJSON(value);
          else await kv.put(key, value, options);
        },
        delete: async (key: string) => {
          if (key === REGISTRY_KEY) {
            this.ctx.storage.transactionSync(() => {
              this.ctx.storage.sql.exec("DELETE FROM direct_accounts");
              this.ctx.storage.sql.exec("INSERT OR REPLACE INTO direct_meta (key, value) VALUES ('present', '0')");
            });
          } else {
            await kv.delete(key);
          }
        },
      } as unknown as KVNamespace,
    };
  }

  private async flushShadow(): Promise<void> {
    const current = this.readRegistryJSON();
    const shadow = this.metaGet("shadow");
    if (current === null || current === shadow) {
      await this.ctx.storage.deleteAlarm();
      return;
    }
    const lastWrite = Number(this.metaGet("shadowAt") ?? "0") || 0;
    if (Date.now() - lastWrite < 1100) {
      await this.ctx.storage.setAlarm(lastWrite + 1100);
      return;
    }
    try {
      await this.env.DIRECT_CODES.put(REGISTRY_KEY, current);
      this.ctx.storage.transactionSync(() => {
        this.ctx.storage.sql.exec("INSERT OR REPLACE INTO direct_meta (key, value) VALUES ('shadow', ?)", current);
        this.ctx.storage.sql.exec("INSERT OR REPLACE INTO direct_meta (key, value) VALUES ('shadowAt', ?)", String(Date.now()));
      });
      await this.ctx.storage.deleteAlarm();
    } catch {
      // KV rate-limiting or transient errors never roll back committed SQLite state.
      await this.ctx.storage.setAlarm(Date.now() + 5000);
    }
  }

  override fetch(req: Request): Promise<Response> {
    return this.serial(async () => {
      try {
        await this.ensureHydrated();
        return await worker.fetch(req, this.directEnv());
      } finally {
        await this.flushShadow();
      }
    });
  }

  override alarm(): Promise<void> {
    return this.serial(async () => {
      await this.ensureHydrated();
      await this.flushShadow();
    });
  }
}

const DIRECT_POST_PATHS = ["/direct/redeem", "/direct/servers", "/direct/move", "/direct/revoke"];
const DIRECT_GET_PATHS = ["/direct/authfile", "/direct/users"];

export default {
  async fetch(req: Request, env: CoordinatedEnv): Promise<Response> {
    const path = new URL(req.url).pathname;
    if (req.method === "GET" && DIRECT_GET_PATHS.includes(path)) {
      try {
        return await env.DIRECT_REGISTRY.getByName(REGISTRY_KEY).fetch(req);
      } catch {
        return Response.json({ error: "Direct registry temporarily unavailable; try again" }, { status: 503 });
      }
    }
    if (req.method === "POST" && DIRECT_POST_PATHS.includes(path)) {
      if (path === "/direct/revoke") {
        const expected = env.DIRECT_ADMIN_TOKEN || env.DIRECT_SYNC_TOKEN || "";
        if (!expected) return Response.json({ error: "direct revoke not configured" }, { status: 503 });
        const auth = req.headers.get("Authorization") || "";
        const got = auth.startsWith("Bearer ") ? auth.slice(7) : "";
        if (!got || !sameSecret(got, expected)) return Response.json({ error: "unauthorized" }, { status: 401 });
      }
      let body: string;
      try {
        const maxBytes = path === "/direct/revoke" ? 4096 : MAX_JSON_BODY_BYTES;
        const parsed = await readJSONObject<Record<string, unknown>>(req, path === "/direct/servers", maxBytes);
        body = JSON.stringify(parsed);
      } catch (error) {
        return Response.json({ error: "bad request" }, { status: jsonBodyStatus(error) });
      }
      try {
        return await env.DIRECT_REGISTRY.getByName(REGISTRY_KEY).fetch(new Request(req.url, {
          method: "POST",
          headers: req.headers,
          body,
        }));
      } catch {
        return Response.json({ error: "Direct registry temporarily unavailable; try again" }, { status: 503 });
      }
    }
    if (req.method !== "POST" || !["/provision", "/standard/recover"].includes(path)) return worker.fetch(req, env);
    if (req.headers.get("x-gtmux-reg") !== env.REG_SECRET) return Response.json({ error: "unauthorized" }, { status: 401 });
    let body: string;
    let deviceId: string;
    try {
      const parsed = await readJSONObject<{ deviceId?: string; recover?: boolean; force?: boolean }>(req, false, 4096);
      body = JSON.stringify(parsed);
      if (path === "/standard/recover" && (parsed?.recover !== true || parsed?.force === true)) {
        return Response.json({ error: "stable recovery mode required" }, { status: 400 });
      }
      deviceId = typeof parsed?.deviceId === "string" ? parsed.deviceId.trim() : "";
      if (!/^[a-zA-Z0-9_-]{16,128}$/.test(deviceId)) return Response.json({ error: "bad deviceId" }, { status: 400 });
    } catch (error) {
      const status = jsonBodyStatus(error);
      return Response.json({ error: status === 413 ? "request too large" : "bad json" }, { status });
    }
    try {
      return await env.STANDARD_DEVICES.getByName(deviceId).provision(deviceId, body, req.headers.get("CF-Connecting-IP") || "unknown");
    } catch {
      // Never leak credentials or Cloudflare request bodies in errors/logs.
      return Response.json({ error: "Standard recovery temporarily unavailable; try again" }, { status: 503 });
    }
  },
  async scheduled(_event: ScheduledEvent, env: CoordinatedEnv, ctx: ExecutionContext): Promise<void> {
    ctx.waitUntil(sweep(env));
  },
};

export const SWEEP_BUDGET_MS = 20_000;
const SWEEP_MAX_KEYS = 100;
const SWEEP_PAGE_LIMIT = 25;

// Bound each invocation in both key count and wall-clock time. Persist reap:cursor
// only for completed pages so unvisited keys are never skipped, and always run
// reconcileActiveCount even if the sweep stops early on its time budget.
export async function sweep(
  env: CoordinatedEnv,
  options: { budgetMs?: number; now?: () => number; pageLimit?: number } = {},
): Promise<void> {
  const now = options.now ?? Date.now;
  const budgetMs = options.budgetMs ?? SWEEP_BUDGET_MS;
  const pageLimit = options.pageLimit ?? SWEEP_PAGE_LIMIT;
  const started = now();
  try {
    let cursor = (await env.TUNNELS.get("reap:cursor")) || undefined;
    let resumeCursor: string | undefined = cursor;
    let scanned = 0;
    let completedCycle = false;
    while (scanned < SWEEP_MAX_KEYS && now() - started < budgetMs) {
      const limit = Math.min(pageLimit, SWEEP_MAX_KEYS - scanned);
      const page = await env.TUNNELS.list({ limit, ...(cursor ? { cursor } : {}) });
      let interrupted = false;
      for (const key of page.keys) {
        if (now() - started >= budgetMs) {
          interrupted = true;
          break;
        }
        scanned++;
        if (!/^[a-zA-Z0-9_-]{16,128}$/.test(key.name)) continue;
        try { await env.STANDARD_DEVICES.getByName(key.name).reap(key.name); }
        catch { console.warn(JSON.stringify({ event: "standard.reclaim_retry" })); }
      }
      if (interrupted) break;
      if (page.list_complete) {
        completedCycle = true;
        resumeCursor = undefined;
        break;
      }
      cursor = page.cursor;
      resumeCursor = page.cursor;
    }
    if (completedCycle) await env.TUNNELS.delete("reap:cursor");
    else if (resumeCursor) await env.TUNNELS.put("reap:cursor", resumeCursor);
  } finally {
    await reconcileActiveCount(env);
  }
}
