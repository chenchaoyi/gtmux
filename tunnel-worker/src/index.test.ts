// Unit test for the reaper's decision function (the abuse-cleanup logic). Pure, no
// network: pins WHICH tunnels the daily cron deletes so a threshold edit can't silently
// start reaping live tunnels. Zero deps: node's built-in test runner with type-strip —
//   node --experimental-strip-types --test src/index.test.ts
import { test, type TestContext } from "node:test";
import assert from "node:assert/strict";
import worker, { shouldReap, type Env } from "./index.ts";

const NOW = Date.parse("2026-08-01T00:00:00Z");
const hoursAgo = (h: number) => new Date(NOW - h * 3600_000).toISOString();
const daysAgo = (d: number) => new Date(NOW - d * 86400_000).toISOString();

test("never-connected junk is reaped once older than the grace window", () => {
  // 24h never-connected grace, 90d idle window.
  assert.equal(shouldReap({ id: "1", name: "gtmux-abc", created_at: hoursAgo(48) }, NOW, 24, 90), true);
  assert.equal(shouldReap({ id: "1", name: "gtmux-abc", created_at: hoursAgo(2) }, NOW, 24, 90), false, "fresh never-connected stays");
});

test("a connected-but-abandoned tunnel is reaped only past the idle window", () => {
  assert.equal(shouldReap({ id: "1", name: "gtmux-abc", created_at: daysAgo(200), conns_active_at: daysAgo(120) }, NOW, 24, 90), true);
  assert.equal(shouldReap({ id: "1", name: "gtmux-abc", created_at: daysAgo(200), conns_active_at: daysAgo(3) }, NOW, 24, 90), false, "recently-active stays");
});

test("non-gtmux tunnels are never touched", () => {
  assert.equal(shouldReap({ id: "1", name: "some-other-tunnel", created_at: hoursAgo(9999) }, NOW, 24, 90), false);
});

test("unparseable timestamps never trigger a reap", () => {
  assert.equal(shouldReap({ id: "1", name: "gtmux-abc", created_at: "not-a-date" }, NOW, 24, 90), false);
  assert.equal(shouldReap({ id: "1", name: "gtmux-abc", created_at: daysAgo(200), conns_active_at: "garbage" }, NOW, 24, 90), false);
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
  const env = {
    TUNNELS: {
      async get(key: string, format?: string) {
        if (key !== DEVICE || options.existing === false) return null;
        return format === "json" ? record : JSON.stringify(record);
      },
      async put(key: string) { writes.push(key); },
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
        result: { deleted_at: options.gone === "deleted" ? "2026-10-01T00:00:00Z" : null } },
        { status: missing ? 404 : unavailable ? 503 : 200 });
    }
    if (method === "POST" && url.pathname.endsWith("/cfd_tunnel")) { stage = "create"; result = { id: "replacement-tunnel" }; }
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
  return { env, calls, writes, request };
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

test("force corrects a stale tunnel CNAME in place", async t => {
  const f = repairFixture(t, { records: [{ id: "dns-id", name: record.hostname, type: "CNAME", content: "old-tunnel.cfargotunnel.com", proxied: false }] });
  assert.equal((await worker.fetch(f.request(), f.env)).status, 200);
  assert.equal(f.calls[3].method, "PUT");
  assert.equal(f.calls[3].path, "/client/v4/zones/zone/dns_records/dns-id");
  assert.equal(f.calls[3].body.content, "existing-tunnel.cfargotunnel.com");
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
    assert.notEqual(body.url, `https://${record.hostname}`);
    assert.deepEqual(f.writes, [DEVICE, "cap:ip:unknown", "cap:active"]);
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
  assert.deepEqual(f.writes, []);
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

