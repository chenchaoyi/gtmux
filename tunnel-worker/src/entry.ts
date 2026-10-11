import { DurableObject } from "cloudflare:workers";
import worker, { reconcileActiveCount, reapDevice, type Env, type TunnelRecord } from "./index.ts";

export interface CoordinatedEnv extends Env {
  STANDARD_DEVICES: DurableObjectNamespace<StandardDevice>;
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

export default {
  async fetch(req: Request, env: CoordinatedEnv): Promise<Response> {
    const path = new URL(req.url).pathname;
    if (req.method !== "POST" || !["/provision", "/standard/recover"].includes(path)) return worker.fetch(req, env);
    if (req.headers.get("x-gtmux-reg") !== env.REG_SECRET) return Response.json({ error: "unauthorized" }, { status: 401 });
    let body: string;
    let deviceId: string;
    try {
      const reader = req.body?.getReader();
      const chunks: Uint8Array[] = [];
      let size = 0;
      if (reader) {
        try {
          for (;;) {
            const part = await reader.read();
            if (part.done) break;
            size += part.value.length;
            if (size > 4096) {
              await reader.cancel();
              return Response.json({ error: "request too large" }, { status: 413 });
            }
            chunks.push(part.value);
          }
        } finally { reader.releaseLock(); }
      }
      const bytes = new Uint8Array(size);
      let offset = 0;
      for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.length; }
      body = new TextDecoder().decode(bytes);
      const parsed = JSON.parse(body);
      if (path === "/standard/recover" && (parsed?.recover !== true || parsed?.force === true)) {
        return Response.json({ error: "stable recovery mode required" }, { status: 400 });
      }
      deviceId = typeof parsed?.deviceId === "string" ? parsed.deviceId.trim() : "";
      if (!/^[a-zA-Z0-9_-]{16,128}$/.test(deviceId)) return Response.json({ error: "bad deviceId" }, { status: 400 });
    } catch { return Response.json({ error: "bad json" }, { status: 400 }); }
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

// Bound each invocation. Continue discovery on the following daily tick instead
// of claiming a complete scan or resetting the cap from a partial result.
async function sweep(env: CoordinatedEnv): Promise<void> {
  const cursor = await env.TUNNELS.get("reap:cursor") || undefined;
  const page = await env.TUNNELS.list({ limit: 100, ...(cursor ? { cursor } : {}) });
  for (const key of page.keys) {
    if (!/^[a-zA-Z0-9_-]{16,128}$/.test(key.name)) continue;
    try { await env.STANDARD_DEVICES.getByName(key.name).reap(key.name); }
    catch { console.warn(JSON.stringify({ event: "standard.reclaim_retry" })); }
  }
  if (page.list_complete) await env.TUNNELS.delete("reap:cursor");
  else await env.TUNNELS.put("reap:cursor", page.cursor);
  await reconcileActiveCount(env);
}
