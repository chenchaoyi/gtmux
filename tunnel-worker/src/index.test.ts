// Unit test for the reaper's decision function (the abuse-cleanup logic). Pure, no
// network: pins WHICH tunnels the daily cron deletes so a threshold edit can't silently
// start reaping live tunnels. Zero deps: node's built-in test runner with type-strip —
//   node --experimental-strip-types --test src/index.test.ts
import { test, type TestContext } from "node:test";
import assert from "node:assert/strict";
import worker, { shouldReap, reconcileActiveCount, reapDevice, type CFTunnel, type Env } from "./index.ts";

const NOW = Date.parse("2026-08-01T00:00:00Z");
const hoursAgo = (h: number) => new Date(NOW - h * 3600_000).toISOString();
const daysAgo = (d: number) => new Date(NOW - d * 86400_000).toISOString();

const offline = (extra: Partial<CFTunnel> = {}): CFTunnel => ({
  id: "1", name: "gtmux-abc", created_at: daysAgo(200), status: "down",
  conns_active_at: null, conns_inactive_at: daysAgo(3), connections: [], ...extra,
});

test("provider null active timestamp means offline, not never connected", () => {
  assert.equal(shouldReap(offline({ conns_inactive_at: hoursAgo(17) }), NOW, 24, 90), false);
  assert.equal(shouldReap(offline({ conns_inactive_at: daysAgo(91) }), NOW, 24, 90), true);
  assert.equal(shouldReap(offline({ conns_inactive_at: daysAgo(90) }), NOW, 24, 90), false);
});
test("only explicitly never-run instances use the 24-hour grace", () => {
  assert.equal(shouldReap(offline({ status: "inactive", conns_inactive_at: null, created_at: hoursAgo(48) }), NOW, 24, 90), true);
  assert.equal(shouldReap(offline({ status: "inactive", conns_inactive_at: null, created_at: hoursAgo(24) }), NOW, 24, 90), false);
});
test("long-running healthy/degraded tunnels and live connections are never reaped", () => {
  for (const status of ["healthy", "degraded", "unknown"]) {
    assert.equal(shouldReap(offline({ status, conns_active_at: daysAgo(150) }), NOW, 24, 90), false);
    assert.equal(shouldReap(offline({ status, conns_inactive_at: daysAgo(150) }), NOW, 24, 90), false);
  }
  assert.equal(shouldReap(offline({ connections: [{}], conns_inactive_at: daysAgo(150) }), NOW, 24, 90), false);
});
test("missing, inconsistent, invalid and future fields fail closed", () => {
  for (const extra of [
    { status: undefined }, { conns_active_at: undefined }, { connections: undefined },
    { conns_inactive_at: undefined }, { conns_inactive_at: "garbage" },
    { conns_inactive_at: daysAgo(-1) }, { conns_inactive_at: daysAgo(300) },
    { created_at: "garbage" }, { created_at: daysAgo(-1) }, { name: "other" },
  ]) assert.equal(shouldReap(offline(extra), NOW, 24, 90), false, JSON.stringify(extra));
});

const DEVICE = "device-standard-0001";
const record = { tunnelId: "existing-tunnel", label: "abcdefgh", hostname: "gtmux-abcdefgh.example.dev" };

// Exercise the actual HTTP handler with a fake Cloudflare API, not live resources.
function repairFixture(t: TestContext, options: {
  records?: unknown[]; fail?: "token" | "lookup" | "ingress" | "dns"; existing?: boolean;
  gone?: "missing" | "deleted" | "healthy" | "unavailable"; failNewToken?: boolean;
} = {}) {
  const calls: { method: string; path: string; body: any }[] = [];
  const writes: string[] = [];
  const store = new Map<string, string>(options.existing === false ? [] : [[DEVICE, JSON.stringify(record)]]);
  const env = {
    TUNNELS: {
      async get(key: string, format?: string) {
        const value = store.get(key) ?? null;
        return value && format === "json" ? JSON.parse(value) : value;
      },
      async put(key: string, value: string) { writes.push(key); store.set(key, value); },
    },
    REG_SECRET: "test-gate", CF_API_TOKEN: "test-provider-token", CF_ACCOUNT_ID: "account",
    CF_ZONE_ID: "zone", ZONE_NAME: "example.dev", LOCAL_SERVICE: "http://localhost:8765",
  } as unknown as Env;
  t.mock.method(globalThis, "fetch", async (input: string, init: RequestInit) => {
    const url = new URL(input);
    const method = init.method || "GET";
    calls.push({ method, path: url.pathname + url.search, body: init.body ? JSON.parse(String(init.body)) : undefined });
    let stage: string;
    let result: unknown;
    if (url.pathname.endsWith("/existing-tunnel") && method === "GET") {
      const missing = options.gone === "missing";
      const unavailable = options.gone === "unavailable";
      return Response.json({ success: !missing && !unavailable,
        result: { id: record.tunnelId, deleted_at: options.gone === "deleted" ? "2026-10-01T00:00:00Z" : null } },
        { status: missing ? 404 : unavailable ? 503 : 200 });
    }
    if (method === "GET" && url.pathname.endsWith("/cfd_tunnel")) { stage = "list"; result = []; }
    else if (method === "POST" && url.pathname.endsWith("/cfd_tunnel")) { stage = "create"; result = { id: "replacement-tunnel" }; }
    else if (url.pathname.endsWith("/token")) { stage = "token"; result = "connector"; }
    else if (url.pathname.endsWith("/configurations")) { stage = "ingress"; result = {}; }
    else if (method === "GET" && url.pathname.endsWith("/dns_records")) { stage = "lookup"; result = options.records || []; }
    else if (["POST", "PUT"].includes(method) && url.pathname.includes("/dns_records")) { stage = "dns"; result = {}; }
    else { throw new Error(`unexpected Cloudflare mutation: ${method} ${url.pathname}`); }
    const failed = options.fail === stage ||
      (stage === "token" && !!options.gone && url.pathname.includes("/existing-tunnel/")) ||
      (stage === "token" && options.failNewToken === true && url.pathname.includes("/replacement-tunnel/"));
    return Response.json({ success: !failed, result: failed ? undefined : result, errors: failed ? ["provider failed"] : [] }, { status: failed ? 503 : 200 });
  });
  const request = (force: boolean | string = true, recover = false) => new Request("https://control.example/provision", {
    method: "POST", headers: { "x-gtmux-reg": "test-gate", "Content-Type": "application/json" },
    body: JSON.stringify({ deviceId: DEVICE, ...(recover ? { recover: true } : { force }) }),
  });
  return { env, calls, writes, request, store, options };
}

test("force repairs missing DNS and ingress without replacing identity or hostname", async t => {
  const f = repairFixture(t);
  const response = await worker.fetch(f.request(), f.env);
  assert.equal(response.status, 200);
  assert.deepEqual(await response.json(), { hostname: record.hostname, url: `https://${record.hostname}`, token: "connector", repaired: true });
  assert.deepEqual(f.calls.map(c => c.method), ["GET", "GET", "PUT", "POST"]);
  assert.deepEqual(f.calls[2].body.config.ingress, [
    { hostname: record.hostname, service: "http://localhost:8765" }, { service: "http_status:404" },
  ]);
  assert.deepEqual(f.calls[3].body, { type: "CNAME", name: record.hostname, content: "existing-tunnel.cfargotunnel.com", proxied: true });
  assert.deepEqual(f.writes, [], "repair must not rotate the registry or count a new tunnel");
});

test("force refuses another tunnel CNAME even within Cloudflare", async t => {
  const f = repairFixture(t, { records: [{ id: "dns-id", name: record.hostname, type: "CNAME", content: "old-tunnel.cfargotunnel.com", proxied: false }] });
  assert.equal((await worker.fetch(f.request(), f.env)).status, 409);
  assert.deepEqual(f.calls.map(c => c.method), ["GET", "GET"]);
});

test("force leaves a healthy DNS record in place", async t => {
  const f = repairFixture(t, { records: [{ id: "dns-id", name: record.hostname, type: "CNAME", content: "existing-tunnel.cfargotunnel.com", proxied: true }] });
  assert.equal((await worker.fetch(f.request(), f.env)).status, 200);
  assert.deepEqual(f.calls.map(c => c.method), ["GET", "GET", "PUT"]);
});

test("force refuses unrelated DNS before changing ingress", async t => {
  const f = repairFixture(t, { records: [{ id: "foreign", name: record.hostname, type: "CNAME", content: "other.example.dev" }] });
  assert.equal((await worker.fetch(f.request(), f.env)).status, 409);
  assert.deepEqual(f.calls.map(c => c.method), ["GET", "GET"]);
});

for (const stage of ["token", "lookup", "ingress", "dns"] as const) {
  test(`force reports ${stage} failure without creating a replacement`, async t => {
    const f = repairFixture(t, { fail: stage });
    const response = await worker.fetch(f.request(), f.env);
    assert.equal(response.status, 502);
    assert.equal((await response.json() as { repaired?: boolean }).repaired, undefined);
    assert.deepEqual(f.writes, []);
    assert.ok(!f.calls.some(c => c.method === "POST" && c.path.endsWith("/cfd_tunnel")));
  });
}

test("force on an unregistered device does not provision new resources", async t => {
  const f = repairFixture(t, { existing: false });
  assert.equal((await worker.fetch(f.request(), f.env)).status, 409);
  assert.deepEqual(f.calls, []);
});

test("ordinary provisioning keeps the existing fast path", async t => {
  const f = repairFixture(t);
  const response = await worker.fetch(f.request(false), f.env);
  assert.equal(response.status, 200);
  assert.equal((await response.json() as { repaired?: boolean }).repaired, undefined);
  assert.deepEqual(f.calls.map(c => c.method), ["GET"]);
});

test("force must be boolean", async t => {
  const f = repairFixture(t);
  assert.equal((await worker.fetch(f.request("true"), f.env)).status, 400);
  assert.deepEqual(f.calls, []);
});

test("recovery repairs in place and returns its own acknowledgement", async t => {
  const f = repairFixture(t);
  const response = await worker.fetch(f.request(false, true), f.env);
  assert.equal(response.status, 200);
  const body = await response.json() as { recovered: boolean; repaired?: boolean; url: string };
  assert.equal(body.recovered, true);
  assert.equal(body.repaired, undefined);
  assert.equal(body.url, `https://${record.hostname}`);
  assert.deepEqual(f.writes, []);
});

for (const gone of ["missing", "deleted"] as const) {
  test(`recovery replaces a confirmed ${gone} tunnel with the same device identity`, async t => {
    const f = repairFixture(t, { gone });
    const response = await worker.fetch(f.request(false, true), f.env);
    assert.equal(response.status, 200);
    const body = await response.json() as { recovered: boolean; url: string };
    assert.equal(body.recovered, true);
    assert.equal(body.url, `https://${record.hostname}`, "the original pairing address survives old-bug deletion");
    assert.equal(JSON.parse(f.store.get(DEVICE)!).tunnelId, "replacement-tunnel");
    assert.equal(JSON.parse(f.store.get(DEVICE)!).pending, undefined);
    const create = f.calls.findIndex(c => c.method === "POST" && c.path.endsWith("/cfd_tunnel"));
    assert.ok(create > f.calls.findIndex(c => c.path.endsWith("/existing-tunnel")), "confirm deletion before creating anything");
  });
}

for (const gone of ["healthy", "unavailable"] as const) {
  test(`recovery never replaces a ${gone} tunnel after a token error`, async t => {
    const f = repairFixture(t, { gone });
    assert.equal((await worker.fetch(f.request(false, true), f.env)).status, 502);
    assert.ok(!f.calls.some(c => c.method === "POST"));
    assert.deepEqual(f.writes, []);
  });
}

test("a replacement without a usable token never overwrites the device registration", async t => {
  const f = repairFixture(t, { gone: "missing", failNewToken: true });
  const response = await worker.fetch(f.request(false, true), f.env);
  assert.equal(response.status, 502);
  const pending = JSON.parse(f.store.get(DEVICE)!);
  assert.equal(pending.tunnelId, record.tunnelId);
  assert.equal(pending.hostname, record.hostname);
  assert.equal(pending.pending.id, "replacement-tunnel");
  assert.ok(!f.calls.some(c => c.method === "POST" && c.path.includes("dns_records")));
  f.options.failNewToken = false;
  assert.equal((await worker.fetch(f.request(false, true), f.env)).status, 200);
  assert.equal(f.calls.filter(c => c.method === "POST" && c.path.endsWith("cfd_tunnel")).length, 1, "retry resumes the pending instance");
});

test("ordinary provisioning also refuses to replace a tunnel during a provider outage", async t => {
  const f = repairFixture(t, { gone: "unavailable" });
  assert.equal((await worker.fetch(f.request(false), f.env)).status, 502);
  assert.ok(!f.calls.some(c => c.method === "POST"));
});

// The authfile endpoint marks a complete answer, and only that (see AUTHFILE_HEADER).
test("the authfile endpoint vouches for its answer only when the registry exists", async () => {
  const store = new Map<string, string>();
  const kv = { get: async (k: string) => store.get(k) ?? null, put: async (k: string, v: string) => { store.set(k, v); } };
  const env = { DIRECT_CODES: kv, DIRECT_SYNC_TOKEN: "sync-secret", DIRECT_URL: "https://direct.example.dev" } as unknown as Env;
  const get = () => worker.fetch(new Request("https://api.example.dev/direct/authfile", { headers: { Authorization: "Bearer sync-secret" } }), env);

  let res = await get();
  assert.equal(res.status, 200);
  assert.deepEqual(await res.json(), {});
  assert.equal(res.headers.get("X-Gtmux-Authfile"), null, "no registry: an empty answer is not vouched for");

  store.set("registry:v1", JSON.stringify({ accounts: {} }));
  res = await get();
  assert.deepEqual(await res.json(), {});
  assert.equal(res.headers.get("X-Gtmux-Authfile"), "complete; server=default; accounts=0");

  store.set("registry:v1", JSON.stringify({ accounts: { "device-000000000001": { user: "u1", pass: "ab", port: 20001, code: "c", at: 1 } } }));
  res = await get();
  assert.equal(Object.keys(await res.json()).length, 1);
  assert.equal(res.headers.get("X-Gtmux-Authfile"), "complete; server=default; accounts=1");

  const denied = await worker.fetch(new Request("https://api.example.dev/direct/authfile", { headers: { Authorization: "Bearer wrong" } }), env);
  assert.equal(denied.status, 401);
  assert.equal(denied.headers.get("X-Gtmux-Authfile"), null);
});


for (const deletionSucceeds of [false, true]) {
  test(`reaper preserves identity and never drains connections; delete succeeds=${deletionSucceeds}`, async t => {
    const store = new Map([[DEVICE, JSON.stringify(record)], ["cap:active", "2"]]);
    const calls: string[] = [];
    const env = { ...repairFixture(t).env, TUNNELS: {
      get: async (key: string, format?: string) => { const v = store.get(key); return v ? format === "json" ? JSON.parse(v) : v : null; },
      put: async (key: string, value: string) => { store.set(key, value); },
    } } as unknown as Env;
    t.mock.method(globalThis, "fetch", async (input: string, init: RequestInit) => {
      const u = new URL(input); const method = init.method || "GET";
      calls.push(`${method} ${u.pathname}`);
      if (u.pathname.endsWith("existing-tunnel")) {
        if (method === "GET") return Response.json({ success: true, result: offline({ id: record.tunnelId, conns_inactive_at: daysAgo(100) }) });
        return Response.json({ success: deletionSucceeds }, { status: deletionSucceeds ? 200 : 409 });
      }
      if (u.pathname.endsWith("dns_records")) return Response.json({ success: true, result: [
        { id: "owned", type: "CNAME", name: record.hostname, content: `${record.tunnelId}.cfargotunnel.com` },
        { id: "foreign", type: "CNAME", name: record.hostname, content: "new-instance.cfargotunnel.com" },
      ] });
      if (u.pathname.endsWith("/owned")) return Response.json({ success: true });
      throw new Error("unexpected mutation");
    });
    assert.equal(await reapDevice(env, DEVICE), !deletionSucceeds);
    const saved = JSON.parse(store.get(DEVICE)!);
    assert.equal(saved.hostname, record.hostname);
    assert.equal(saved.tunnelId, record.tunnelId);
    assert.ok(!calls.some(c => c.includes("connections") || c.endsWith("foreign")));
    assert.equal(calls.some(c => c.startsWith("DELETE") && c.endsWith("owned")), deletionSucceeds);
    if (deletionSucceeds) assert.ok(calls.indexOf("DELETE /client/v4/accounts/account/cfd_tunnel/existing-tunnel") < calls.indexOf("DELETE /client/v4/zones/zone/dns_records/owned"));
  });
}

test("force never creates a replacement for a registered reclaimed instance", async t => {
  const f = repairFixture(t);
  f.store.set(DEVICE, JSON.stringify({ ...record, reaped: true }));
  assert.equal((await worker.fetch(f.request(), f.env)).status, 409);
  assert.deepEqual(f.calls, []);
});

for (const status of [401, 403, 429, 500]) {
  test(`provider HTTP ${status} is not proof of deletion`, async t => {
    const f = repairFixture(t);
    t.mock.method(globalThis, "fetch", async () => Response.json({ success: false }, { status }));
    assert.equal((await worker.fetch(f.request(false, true), f.env)).status, 502);
    assert.deepEqual(f.writes, []);
  });
}

test("an invalid deletion timestamp cannot authorize replacement", async t => {
  const f = repairFixture(t);
  t.mock.method(globalThis, "fetch", async () => Response.json({ success: true, result: { deleted_at: "not-a-date" } }));
  assert.equal((await worker.fetch(f.request(false, true), f.env)).status, 502);
  assert.deepEqual(f.writes, []);
});

test("affected existing Macs behind one NAT are not blocked by first-enrollment IP caps", async t => {
  const f = repairFixture(t, { gone: "deleted" });
  f.store.set("cap:ip:unknown", "5");
  assert.equal((await worker.fetch(f.request(false, true), f.env)).status, 200);
  assert.equal(f.store.get("cap:ip:unknown"), "5");
});

for (const complete of [true, false]) {
  test(`quota reconciliation never treats an interrupted list as an empty account; complete=${complete}`, async t => {
    const f = repairFixture(t);
    f.store.set("cap:active", "500");
    t.mock.method(globalThis, "fetch", async () => complete ?
      Response.json({ success: true, result: [{ name: "gtmux-owned" }, { name: "gtmux-unattributed" }, { name: "another-app" }] }) :
      Response.json({ success: false }, { status: 503 }));
    await reconcileActiveCount(f.env);
    assert.equal(f.store.get("cap:active"), complete ? "2" : "500");
    assert.deepEqual(f.writes, complete ? ["cap:active"] : []);
  });
}

test("a contended soft-counter write does not strand an already journaled replacement", async t => {
  const f = repairFixture(t, { gone: "deleted" });
  const put = f.env.TUNNELS.put.bind(f.env.TUNNELS);
  t.mock.method(f.env.TUNNELS, "put", async (key: string, value: string) => {
    if (key.startsWith("cap:")) throw new Error("KV write rate limit");
    await put(key, value);
  });
  assert.equal((await worker.fetch(f.request(false, true), f.env)).status, 200);
  assert.equal(JSON.parse(f.store.get(DEVICE)!).tunnelId, "replacement-tunnel");
});


test("abandoned pending replacements are reclaimed without removing the original route or identity", async t => {
  const f = repairFixture(t);
  f.store.set(DEVICE, JSON.stringify({ ...record, reaped: true,
    pending: { id: "pending-id", name: "gtmux-abcdefgh-operation", counted: true } }));
  f.store.set("cap:active", "1");
  const calls: string[] = [];
  t.mock.method(globalThis, "fetch", async (input: string, init: RequestInit) => {
    const url = new URL(input); const method = init.method || "GET";
    calls.push(`${method} ${url.pathname}`);
    if (url.pathname.endsWith("pending-id")) return Response.json({ success: true, result: offline({
      id: "pending-id", status: "inactive", conns_inactive_at: null, created_at: daysAgo(100),
    }) });
    if (url.pathname.endsWith("dns_records")) return Response.json({ success: true, result: [
      { id: "old-route", name: record.hostname, type: "CNAME", content: `${record.tunnelId}.cfargotunnel.com` },
    ] });
    throw new Error("unexpected route mutation");
  });
  assert.equal(await reapDevice(f.env, DEVICE), false);
  const saved = JSON.parse(f.store.get(DEVICE)!);
  assert.equal(saved.hostname, record.hostname);
  assert.equal(saved.tunnelId, record.tunnelId);
  assert.equal(saved.pending.id, undefined);
  assert.equal(f.store.get("cap:active"), "0");
  assert.deepEqual(calls.filter(c => c.startsWith("DELETE")), ["DELETE /client/v4/accounts/account/cfd_tunnel/pending-id"]);
});


test("a deletion response for a different instance cannot authorize replacing this device", async t => {
  const f = repairFixture(t);
  t.mock.method(globalThis, "fetch", async () => Response.json({ success: true,
    result: { id: "another-device-instance", deleted_at: "2026-01-01T00:00:00Z" } }));
  assert.equal((await worker.fetch(f.request(false, true), f.env)).status, 502);
  assert.deepEqual(f.writes, []);
});
