import { env, exports } from "cloudflare:workers";
import { runInDurableObject, runDurableObjectAlarm } from "cloudflare:test";
import { afterEach, expect, it, vi } from "vitest";
import { sweep, type CoordinatedEnv, type DirectRegistry, type StandardDevice } from "../src/entry.ts";
import { REGISTRY_KEY, SERVERS_KEY } from "../src/direct.ts";

const bindings = env as unknown as CoordinatedEnv;
const hostname = "gtmux-original.example.dev";
const oldRecord = { tunnelId: "old-id", hostname, label: "original" };

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

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

 it('deployed entry preserves the smaller Standard body limit and rejects before coordination', async () => {
  const p = provider();
  for (const route of ['/provision', '/standard/recover']) {
    for (const body of ['null', '[]', '{broken']) {
      const response = await exports.default.fetch('https://control' + route, {method: 'POST', headers: {'x-gtmux-reg': 'test-gate'}, body});
      expect(response.status).toBe(400);
    }
    for (const length of [undefined, '1', '4097']) {
      const response = await exports.default.fetch('https://control' + route, {method: 'POST', headers: {'x-gtmux-reg': 'test-gate', ...(length ? {'Content-Length': length} : {})}, body: JSON.stringify({deviceId: 'entry-bounds-fixture', padding: 'x'.repeat(4096)})});
      expect(response.status).toBe(413);
    }
    const unauthorized = await exports.default.fetch('https://control' + route, {method: 'POST', body: 'x'.repeat(4097)});
    expect(unauthorized.status).toBe(401);
  }
  expect(p.mock).not.toHaveBeenCalled();
 });

it("DirectRegistry SQLite coordinator serializes concurrent redeem, move, servers, authfile, and revoke without KV 429 loss", async () => {
  const codeA = "gtd-aaaa11112222333344445555";
  const codeB = "gtd-bbbb11112222333344445555";
  await bindings.DIRECT_CODES.put(codeA, JSON.stringify({ label: "alpha" }));
  await bindings.DIRECT_CODES.put(codeB, JSON.stringify({ label: "beta" }));
  await bindings.DIRECT_CODES.put(SERVERS_KEY, JSON.stringify({
    servers: [
      { id: "la", url: "https://la.example.dev", region: "us-west", accepting: true, sync: "sync-la" },
      { id: "sh", url: "https://sh.example.dev", region: "cn-shanghai", accepting: true, sync: "sync-sh" },
      { id: "vip", url: "https://vip.example.dev", region: "us-west", codes: [codeA], sync: "sync-vip" },
    ],
  }));

  const stub = bindings.DIRECT_REGISTRY.getByName(REGISTRY_KEY);
  // Simulate KV 429 contention on registry:v1 writes while concurrent requests execute.
  let putSpy: ReturnType<typeof vi.spyOn> | undefined;
  await runInDurableObject(stub, async (instance: DirectRegistry) => {
    const rawPut = bindings.DIRECT_CODES.put.bind(bindings.DIRECT_CODES);
    putSpy = vi.spyOn(bindings.DIRECT_CODES, "put").mockImplementation(async (key: string, value: string | ArrayBuffer | ArrayBufferView | ReadableStream, options?: KVNamespacePutOptions) => {
      if (key === REGISTRY_KEY) throw new Error("KV PUT 429 Too Many Requests");
      return rawPut(key, value as string, options);
    });
    expect(instance).toBeDefined();
  });

  try {
    const redeemReq = (code: string, deviceId: string, server = "la") =>
      exports.default.fetch("https://control/direct/redeem", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ code, deviceId, server }),
      });

    const [r1, r2, r3, r4] = await Promise.all([
      redeemReq(codeA, "direct-device-0001", "la"),
      redeemReq(codeA, "direct-device-0002", "la"),
      redeemReq(codeA, "direct-device-0003", "sh"),
      redeemReq(codeB, "direct-device-0004", "la"),
    ]);
    for (const res of [r1, r2, r3, r4]) expect(res.status).toBe(200);
    const b1 = await r1.json() as { secret: string; port: number };
    const b2 = await r2.json() as { secret: string; port: number };
    const b3 = await r3.json() as { secret: string; port: number };
    const b4 = await r4.json() as { secret: string; port: number };
    expect(new Set([b1.port, b2.port, b3.port, b4.port]).size).toBe(4);

    // Unauthenticated /direct/servers omits current and code-reserved vip server.
    const publicListing = await (await exports.default.fetch("https://control/direct/servers", {
      method: "POST",
      body: JSON.stringify({ deviceId: "direct-device-0001" }),
    })).json() as { servers: { id: string }[]; current?: string };
    expect(publicListing.current).toBeUndefined();
    expect(publicListing.servers.map(s => s.id)).toEqual(["la", "sh"]);

    // Secret-authenticated /direct/servers discloses current and vip server.
    const authListing = await (await exports.default.fetch("https://control/direct/servers", {
      method: "POST",
      body: JSON.stringify({ deviceId: "direct-device-0001", secret: b1.secret }),
    })).json() as { servers: { id: string }[]; current?: string };
    expect(authListing.current).toBe("la");
    expect(authListing.servers.map(s => s.id)).toEqual(["la", "sh", "vip"]);

    // Concurrent move + revoke serialize cleanly with authfile reads.
    const [moveRes, revokeRes] = await Promise.all([
      exports.default.fetch("https://control/direct/move", {
        method: "POST",
        body: JSON.stringify({ deviceId: "direct-device-0004", secret: b4.secret, server: "sh" }),
      }),
      exports.default.fetch("https://control/direct/revoke", {
        method: "POST",
        headers: { Authorization: "Bearer test-admin-token" },
        body: JSON.stringify({ code: codeA }),
      }),
    ]);
    expect(moveRes.status).toBe(200);
    expect(revokeRes.status).toBe(200);
    expect(await revokeRes.json()).toEqual({ ok: true, code: codeA, removed: 3 });
    expect(await bindings.DIRECT_CODES.get(codeA)).toBeNull();

    const laAuth = await exports.default.fetch("https://control/direct/authfile", {
      headers: { Authorization: "Bearer sync-la" },
    });
    expect(laAuth.status).toBe(200);
    expect(laAuth.headers.get("X-Gtmux-Authfile")).toBe("complete; server=la; accounts=0");
    expect(await laAuth.json()).toEqual({});

    const shAuth = await exports.default.fetch("https://control/direct/users", {
      headers: { Authorization: "Bearer sync-sh" },
    });
    expect(shAuth.status).toBe(200);
    expect(shAuth.headers.get("X-Gtmux-Authfile")).toBe("complete; server=sh; accounts=1");
    const shAccounts = await shAuth.json() as Record<string, string[]>;
    expect(Object.keys(shAccounts)).toEqual([b4.secret]);
  } finally {
    putSpy?.mockRestore();
  }
});

it("DirectRegistry hydrates legacy registry:v1 once, omits authfile claim before initialization, and survives reconstruction", async () => {
  // Fresh isolated stub before any registry exists omits X-Gtmux-Authfile.
  const uninitializedStub = bindings.DIRECT_REGISTRY.getByName("uninitialized-registry-check");
  const emptyRes = await uninitializedStub.fetch(new Request("https://control/direct/authfile", {
    headers: { Authorization: "Bearer test-sync-token" },
  }));
  expect(emptyRes.status).toBe(200);
  expect(emptyRes.headers.get("X-Gtmux-Authfile")).toBeNull();
  expect(await emptyRes.json()).toEqual({});

  // Seed legacy KV before first access on a fresh DO instance and verify one-time SQLite import.
  const legacyDevice = "legacy-direct-device-01";
  const legacySecret = "dlegacy01:0123456789abcdef";
  await bindings.DIRECT_CODES.put(REGISTRY_KEY, JSON.stringify({
    accounts: {
      [legacyDevice]: { user: "dlegacy01", pass: "0123456789abcdef", port: 24567, code: "gtd-legacy00112233", at: 100 },
    },
  }));
  const hydratedStub = bindings.DIRECT_REGISTRY.getByName("hydrated-registry-check");
  const firstSync = await hydratedStub.fetch(new Request("https://control/direct/authfile", {
    headers: { Authorization: "Bearer test-sync-token" },
  }));
  expect(firstSync.headers.get("X-Gtmux-Authfile")).toBe("complete; server=default; accounts=1");
  expect(await firstSync.json()).toEqual({ [legacySecret]: ["^R:127\\.0\\.0\\.1:24567$"] });

  // Overwrite KV with empty data; reconstructing DirectRegistry from SQLite must keep the hydrated account.
  await bindings.DIRECT_CODES.put(REGISTRY_KEY, JSON.stringify({ accounts: {} }));
  await runInDurableObject(hydratedStub, async (instance: DirectRegistry, state) => {
    expect(state.storage.sql.databaseSize).toBeGreaterThan(0);
    const Reconstructed = instance.constructor as typeof DirectRegistry;
    const restored = new Reconstructed(state, bindings);
    const secondSync = await restored.fetch(new Request("https://control/direct/authfile", {
      headers: { Authorization: "Bearer test-sync-token" },
    }));
    expect(await secondSync.json()).toEqual({ [legacySecret]: ["^R:127\\.0\\.0\\.1:24567$"] });
  });
});

it("scheduled sweep honors wall-clock budget, preserves unvisited cursor progress, and still runs reconcileActiveCount", async () => {
  await bindings.TUNNELS.put("sweep-reaped-dev-0001", JSON.stringify({ ...oldRecord, reaped: true }));
  await bindings.TUNNELS.put("sweep-reaped-dev-0002", JSON.stringify({ ...oldRecord, reaped: true }));
  await bindings.TUNNELS.put("sweep-reaped-dev-0003", JSON.stringify({ ...oldRecord, reaped: true }));
  await bindings.TUNNELS.put("cap:active", "99");

  const calls: string[] = [];
  vi.stubGlobal("fetch", vi.fn(async (input: string) => {
    const u = new URL(input);
    calls.push(u.pathname + u.search);
    if (u.pathname.endsWith("/cfd_tunnel")) {
      return Response.json({ success: true, result: [{ name: "gtmux-live-a" }, { name: "gtmux-live-b" }] });
    }
    throw new Error(`unexpected provider call ${u.pathname}`);
  }));

  let fakeNow = 1_000;
  await sweep(bindings, {
    pageLimit: 1,
    budgetMs: 25,
    now: () => {
      const current = fakeNow;
      fakeNow += 10; // page 1 completes (elapsed < 25ms), then budget expires before page 2 finishes
      return current;
    },
  });

  // Tombstones skipped with zero /cfd_tunnel/<id> GETs; reconcileActiveCount still ran and refreshed cap:active.
  expect(calls.every(c => c.includes("per_page=100&page=1"))).toBe(true);
  expect(await bindings.TUNNELS.get("cap:active")).toBe("2");
  expect(await bindings.TUNNELS.get("reap:cursor")).toBeTruthy();
});
