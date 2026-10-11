import { env, exports } from "cloudflare:workers";
import { runInDurableObject, runDurableObjectAlarm } from "cloudflare:test";
import { afterEach, expect, it, vi } from "vitest";
import type { CoordinatedEnv, StandardDevice } from "../src/entry.ts";

const bindings = env as unknown as CoordinatedEnv;
const hostname = "gtmux-original.example.dev";
const oldRecord = { tunnelId: "old-id", hostname, label: "original" };

afterEach(() => vi.unstubAllGlobals());

function provider(options: { failToken?: boolean; createResponseLost?: boolean; idle?: boolean; pendingIdle?: boolean } = {}) {
  const created: { id: string; name: string }[] = [];
  const mutations: string[] = [];
  let route = "old-id.cfargotunnel.com";
  const mock = vi.fn(async (input: string, init: RequestInit) => {
    const u = new URL(input); const method = init.method || "GET";
    const p = u.pathname;
    if (method !== "GET") mutations.push(`${method} ${p}`);
    if (p.endsWith("/cfd_tunnel") && method === "GET") return Response.json({ success: true, result: created.filter(t => t.name === u.searchParams.get("name")) });
    if (p.endsWith("/cfd_tunnel") && method === "POST") {
      const t = { id: `new-id-${created.length}`, name: JSON.parse(String(init.body)).name }; created.push(t);
      // An uncertain response must not lead to a second creation on retry.
      if (options.createResponseLost) { options.createResponseLost = false; throw new Error("connection reset after provider commit"); }
      await new Promise(resolve => setTimeout(resolve, 15));
      return Response.json({ success: true, result: t });
    }
    if (p.endsWith("/old-id") && method === "GET") return Response.json({ success: true, result: options.idle ? {
      id: "old-id", name: "gtmux-original", status: "down", created_at: "2025-01-01T00:00:00Z",
      conns_active_at: null, conns_inactive_at: "2025-02-01T00:00:00Z", connections: [],
    } : { id: "old-id", deleted_at: "2026-10-09T00:00:00Z" } });
    if (p.endsWith("/old-id") && method === "DELETE") { options.idle = false; return Response.json({ success: true }); }
    if (/\/new-id-\d+$/.test(p) && method === "GET") return Response.json({ success: true, result: options.pendingIdle ? { id: p.split("/").pop(), name: "gtmux-original-pending", deleted_at: null, status: "inactive", created_at: "2025-01-01T00:00:00Z", conns_active_at: null, conns_inactive_at: null, connections: [] } : { id: p.split("/").pop(), deleted_at: null, status: "healthy", created_at: new Date().toISOString(), conns_active_at: new Date().toISOString(), conns_inactive_at: null, connections: [{}] } });
    if (/\/new-id-\d+$/.test(p) && method === "DELETE") return Response.json({ success: true });
    if (p.endsWith("/token")) return Response.json({ success: !options.failToken, result: options.failToken ? undefined : "replacement-connector" }, { status: options.failToken ? 503 : 200 });
    if (p.endsWith("/configurations")) return Response.json({ success: true });
    if (p.endsWith("/dns_records") && method === "GET") return Response.json({ success: true, result: route ? [{ id: "route", name: hostname, type: "CNAME", content: route, proxied: true }] : [] });
    if (p.endsWith("/route") && method === "PUT" || p.endsWith("/dns_records") && method === "POST") { route = JSON.parse(String(init.body)).content; return Response.json({ success: true }); }
    if (p.endsWith("/route") && method === "DELETE") { route = ""; return Response.json({ success: true }); }
    throw new Error(`unexpected provider call ${method} ${p}`);
  });
  vi.stubGlobal("fetch", mock);
  return { created, mutations, mock, options, route: () => route };
}
async function seed(device: string) {
  await bindings.TUNNELS.put(device, JSON.stringify(oldRecord));
}
function request(device: string) {
  return exports.default.fetch("https://control/standard/recover", { method: "POST", headers: { "x-gtmux-reg": "test-gate" }, body: JSON.stringify({ deviceId: device, recover: true }) });
}

it("legacy misreaped device + simultaneous recovery requests create one replacement and keep its address", async () => {
  const device = "legacy-deleted-device-0001"; await seed(device);
  const p = provider();
  const responses = await Promise.all([request(device), request(device), request(device)]);
  for (const response of responses) {
    expect(response.status).toBe(200);
    expect(await response.json()).toMatchObject({ url: `https://${hostname}`, recovered: true, addressPreserved: true });
  }
  expect(p.created).toHaveLength(1);
  const stub = bindings.STANDARD_DEVICES.getByName(device);
  await runInDurableObject(stub, async (_instance: StandardDevice, state) => {
    expect(JSON.parse((await state.storage.get<string>("record"))!)).toMatchObject({ tunnelId: "new-id-0", hostname, label: "original" });
    expect(state.storage.sql.databaseSize).toBeGreaterThan(0);
  });
  expect(p.route()).toBe("new-id-0.cfargotunnel.com");
});

it("persistent pending journal survives reconstruction and stale KV, resuming without a second tunnel", async () => {
  const device = "interrupted-device-0002"; await seed(device);
  const p = provider({ failToken: true });
  expect((await request(device)).status).toBe(502);
  const stub = bindings.STANDARD_DEVICES.getByName(device);
  // Deliberately overwrite the index with old data. Reconstruct the class using the
  // real durable storage, as runtime eviction would, instead of just repeated calls.
  await bindings.TUNNELS.put(device, JSON.stringify(oldRecord));
  p.options.failToken = false;
  await runInDurableObject(stub, async (instance: StandardDevice, state) => {
    const persisted = JSON.parse((await state.storage.get<string>("record"))!);
    expect(persisted.pending.id).toBe("new-id-0");
    const Reconstructed = instance.constructor as typeof StandardDevice;
    const restored = new Reconstructed(state, bindings);
    const response = await restored.provision(device, JSON.stringify({ deviceId: device, recover: true }), "test-ip");
    expect(response.status).toBe(200);
    expect(await response.json()).toMatchObject({ url: `https://${hostname}` });
  });
  expect(p.created).toHaveLength(1);
  await new Promise(resolve => setTimeout(resolve, 1150));
  await runDurableObjectAlarm(stub);
  const shadow = await bindings.TUNNELS.get(device, "json") as typeof oldRecord;
  expect(shadow.tunnelId).toBe("new-id-0");
});

it("a lost create response is found by its persistent opaque operation name", async () => {
  const device = "lost-response-device-0003"; await seed(device);
  const p = provider({ createResponseLost: true });
  expect((await request(device)).status).toBe(502);
  expect((await request(device)).status).toBe(200);
  expect(p.created).toHaveLength(1);
});

it("reclaim and recovery share coordination; an old instance cannot delete a replacement route", async () => {
  const device = "reclaim-race-device-0004"; await seed(device);
  const p = provider({ idle: true });
  const stub = bindings.STANDARD_DEVICES.getByName(device);
  await Promise.all([stub.reap(device), request(device)]);
  expect(p.created).toHaveLength(1);
  expect(p.route()).toBe("new-id-0.cfargotunnel.com");
  expect(p.mutations.some(m => m.includes("/connections"))).toBe(false);
  const deleteOld = p.mutations.findIndex(m => m === "DELETE /client/v4/accounts/account/cfd_tunnel/old-id");
  const routeNew = p.mutations.findIndex(m => m === "POST /client/v4/zones/zone/dns_records");
  expect(deleteOld).toBeGreaterThanOrEqual(0);
  expect(routeNew).toBeGreaterThan(deleteOld);
  await stub.reap(device);
  expect(p.route()).toBe("new-id-0.cfargotunnel.com");
});


it("an abandoned pending instance can be reclaimed and a returning legacy Mac still resumes at the original address", async () => {
  const device = "pending-reclaim-device-0005"; await seed(device);
  const p = provider({ failToken: true, pendingIdle: true });
  expect((await request(device)).status).toBe(502);
  const stub = bindings.STANDARD_DEVICES.getByName(device);
  expect(await stub.reap(device)).toBe(false);
  expect(p.mutations).toContain("DELETE /client/v4/accounts/account/cfd_tunnel/new-id-0");
  p.options.failToken = false;
  p.options.pendingIdle = false;
  const response = await request(device);
  expect(response.status).toBe(200);
  expect(await response.json()).toMatchObject({ url: `https://${hostname}`, addressPreserved: true });
  expect(p.created).toHaveLength(2);
  expect(p.route()).toBe("new-id-1.cfargotunnel.com");
});
