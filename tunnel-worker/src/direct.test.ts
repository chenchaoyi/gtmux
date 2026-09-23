// Direct per-device accounts (openspec/changes/direct-per-device-accounts). The shared
// DIRECT_SECRET let any holder bind any Mac's port and reach the VPS's loopback; these pin
// that an account can bind its own port and nothing else. node --test, no network.
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  crc32, preferredPort, assignPort, redeem, authfile, revokeCode, loadRegistry, sameSecret,
  loadServers, offered, serverByToken, serverOf, move,
  REGISTRY_KEY, SERVERS_KEY, LEGACY_SERVER, PORT_BASE, PORT_SPAN, type KV, type Registry,
} from "./direct.ts";

class MemKV implements KV {
  m = new Map<string, string>();
  failPut = false;
  async get(k: string) { return this.m.has(k) ? this.m.get(k)! : null; }
  async put(k: string, v: string) { if (this.failPut) throw new Error("kv down"); this.m.set(k, v); }
}

const CODE = "gtd-0123456789abcdef01234567";
const OTHER = "gtd-fedcba9876543210fedcba98";
const dev = (n: number) => "device-" + String(n).padStart(12, "0");

function fresh(): MemKV {
  const kv = new MemKV();
  kv.m.set(CODE, JSON.stringify({ label: "test" }));
  kv.m.set(OTHER, JSON.stringify({ label: "other" }));
  return kv;
}
const deps = (kv: KV, cap = 3) => ({ kv, url: "https://tunnel.example.dev", cap, now: 1_000 });

// chisel matches a reverse remote as "R:<host>:<port>" against the account's regexes.
function allows(file: Record<string, string[]>, secret: string, remote: string): boolean {
  return (file[secret] ?? []).some((re) => new RegExp(re).test(remote));
}

test("the port formula is the client's: Go's crc32.ChecksumIEEE over the id", () => {
  // Computed with Go's hash/crc32, the implementation the client derives its port with.
  for (const [id, crc, port] of [
    ["a", 3904355907, 55907],
    ["0123456789abcdef0123456789abcdef", 2002367758, 27758],
    ["device-with-dashes_and_underscores-000", 484442940, 22940],
    ["ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", 688218075, 38075],
  ] as const) {
    assert.equal(crc32(id), crc, id);
    assert.equal(preferredPort(id), port, id);
  }
});

test("each device gets its own account, allowed its own port and nothing else", async () => {
  const kv = fresh();
  const a = await redeem(CODE, dev(1), deps(kv));
  const b = await redeem(CODE, dev(2), deps(kv));
  assert.equal(a.status, 200);
  assert.equal(b.status, 200);
  assert.notEqual(a.body.secret, b.body.secret, "two devices must not share a credential");
  const file = authfile(await loadRegistry(kv));
  const [sa, sb] = [a.body.secret as string, b.body.secret as string];
  const [pa, pb] = [a.body.port as number, b.body.port as number];

  assert.ok(allows(file, sa, `R:127.0.0.1:${pa}`), "A binds its own port");
  assert.ok(!allows(file, sa, `R:127.0.0.1:${pb}`), "A must not bind B's port");
  assert.ok(!allows(file, sb, `R:127.0.0.1:${pa}`), "B must not bind A's port");
  // The loopback services the shared user could reach, and a public bind on the VPS.
  for (const remote of ["127.0.0.1:2019", "localhost:8080", `127.0.0.1:${pb}`, `R:0.0.0.0:${pa}`, `R:127.0.0.1:${pa}0`, "socks"]) {
    assert.ok(!allows(file, sa, remote), `A must not open ${remote}`);
  }
});

test("a device that redeems again gets the same account back", async () => {
  const kv = fresh();
  const first = await redeem(CODE, dev(1), deps(kv));
  const again = await redeem(CODE, dev(1), deps(kv));
  assert.deepEqual(again.body, first.body);
  assert.equal(Object.keys((await loadRegistry(kv)).accounts).length, 1);
});

test("a code mints accounts for at most `cap` devices", async () => {
  const kv = fresh();
  for (let i = 1; i <= 3; i++) assert.equal((await redeem(CODE, dev(i), deps(kv))).status, 200);
  const fourth = await redeem(CODE, dev(4), deps(kv));
  assert.equal(fourth.status, 409);
  assert.match(String(fourth.body.error), /in use on 3 devices/);
  assert.equal(Object.keys((await loadRegistry(kv)).accounts).length, 3, "no account for the refused device");
  // A device already on the code is not a new device.
  assert.equal((await redeem(CODE, dev(2), deps(kv))).status, 200);
});

test("two devices whose derived ports collide never share one", async () => {
  // Find two ids with the same derived port, deterministically.
  const seen = new Map<number, string>();
  let pair: [string, string] | null = null;
  for (let i = 0; !pair; i++) {
    const id = "collide-" + String(i).padStart(10, "0");
    const p = preferredPort(id);
    if (seen.has(p)) pair = [seen.get(p)!, id];
    else seen.set(p, id);
  }
  const kv = fresh();
  const a = await redeem(CODE, pair[0], deps(kv));
  const b = await redeem(CODE, pair[1], deps(kv));
  assert.equal(a.body.port, preferredPort(pair[0]));
  assert.notEqual(b.body.port, a.body.port, "the second device got the first one's port");
  assert.ok((b.body.port as number) >= PORT_BASE && (b.body.port as number) < PORT_BASE + PORT_SPAN);
});

test("if a race ever puts two accounts on one port, only the older can bind it", () => {
  const reg: Registry = { accounts: {
    [dev(1)]: { user: "dold", pass: "aa", port: 30000, code: CODE, at: 1 },
    [dev(2)]: { user: "dnew", pass: "bb", port: 30000, code: CODE, at: 2 },
  } };
  const file = authfile(reg);
  assert.ok(allows(file, "dold:aa", "R:127.0.0.1:30000"));
  assert.equal(file["dnew:bb"], undefined, "the newer claimant must not be emitted at all");
});

test("revoking a code removes the accounts it minted, and only those", async () => {
  const kv = fresh();
  const a = await redeem(CODE, dev(1), deps(kv));
  const other = await redeem(OTHER, dev(2), deps(kv));
  const { reg, removed } = revokeCode(await loadRegistry(kv), CODE);
  assert.equal(removed, 1);
  const file = authfile(reg);
  assert.equal(file[a.body.secret as string], undefined, "the revoked device can still connect");
  assert.ok(file[other.body.secret as string], "another code's device was removed too");
});

test("a device moving to another code keeps its account and port", async () => {
  const kv = fresh();
  const first = await redeem(CODE, dev(1), deps(kv));
  const moved = await redeem(OTHER, dev(1), deps(kv));
  assert.equal(moved.body.secret, first.body.secret);
  assert.equal(moved.body.port, first.body.port);
  assert.equal((await loadRegistry(kv)).accounts[dev(1)].code, OTHER);
});

test("refusals mint nothing and say why", async () => {
  const kv = fresh();
  assert.equal((await redeem("short", dev(1), deps(kv))).status, 403);
  assert.equal((await redeem("gtd-000000000000000000000000", dev(1), deps(kv))).status, 403, "unknown code");
  const old = await redeem(CODE, "", deps(kv));
  assert.equal(old.status, 400, "a client that sends no device id");
  assert.match(String(old.body.error), /update/);
  assert.equal(kv.m.has(REGISTRY_KEY), false, "a refusal wrote an account");
});

test("an account that could not be saved is never handed out", async () => {
  const kv = fresh();
  kv.failPut = true;
  await assert.rejects(redeem(CODE, dev(1), deps(kv)));
});

test("the sync token comparison does not short-circuit", () => {
  assert.ok(sameSecret("abc123", "abc123"));
  assert.ok(!sameSecret("abc123", "abc124"));
  assert.ok(!sameSecret("abc123", "abc1234"));
  assert.ok(!sameSecret("", "x"));
});

// A choice of Direct servers (openspec/changes/direct-server-choice). Servers are data the
// provisioner serves, so adding one is configuration; an account belongs to one server, and
// a server's account file holds its own tenants only.

const SERVERS = {
  servers: [
    { id: "la", url: "https://la.example.dev", region: "us-west", accepting: true, sync: "tok-la" },
    { id: "sh", url: "https://sh.example.dev", region: "cn-shanghai", accepting: true, sync: "tok-sh" },
    { id: "house", url: "https://house.example.dev", region: "cn-shanghai", codes: [CODE], sync: "tok-house" },
  ],
};

function withServers(kv: MemKV): MemKV {
  kv.m.set(SERVERS_KEY, JSON.stringify(SERVERS));
  return kv;
}

test("no server list configured: one server, the configured URL, exactly as before", async () => {
  const kv = fresh();
  const list = await loadServers(kv, "https://tunnel.example.dev");
  assert.deepEqual(list, [{ id: LEGACY_SERVER, url: "https://tunnel.example.dev", accepting: true }]);
  const r = await redeem(CODE, dev(1), deps(kv));
  assert.equal(r.status, 200);
  assert.equal(r.body.url, "https://tunnel.example.dev");
  // and the account it minted is served to the server authenticating with the legacy token
  const reg = await loadRegistry(kv);
  assert.equal(Object.keys(authfile(reg, LEGACY_SERVER)).length, 1);
});

test("a server added to the list is usable with no code change here", async () => {
  const kv = withServers(fresh());
  const r = await redeem(CODE, dev(1), { ...deps(kv), server: "sh" });
  assert.equal(r.status, 200);
  assert.equal(r.body.url, "https://sh.example.dev");
  assert.equal(r.body.server, "sh");
});

test("a region preference picks a server in that region", async () => {
  const kv = withServers(fresh());
  const r = await redeem(OTHER, dev(2), { ...deps(kv), region: "cn-shanghai" });
  assert.equal(r.body.server, "sh"); // not "house": that one is reserved for another code
});

test("a server reserved for named codes is offered to those codes only", async () => {
  const kv = withServers(fresh());
  const list = await loadServers(kv, "");
  assert.deepEqual(offered(list, "").map((s) => s.id), ["la", "sh"]);
  assert.deepEqual(offered(list, CODE).map((s) => s.id), ["la", "sh", "house"]);
  // and a code that may not use it cannot be assigned to it, even by naming it
  const r = await redeem(OTHER, dev(3), { ...deps(kv), server: "house" });
  assert.equal(r.status, 404);
  const ok = await redeem(CODE, dev(4), { ...deps(kv), server: "house" });
  assert.equal(ok.body.server, "house");
});

test("what a client is told about a server never includes the server's own token", async () => {
  const list = await loadServers(withServers(fresh()), "");
  for (const s of offered(list, CODE)) {
    assert.equal("sync" in s, false);
    assert.equal("codes" in s, false);
  }
});

test("an account file holds that server's tenants and no others", async () => {
  const kv = withServers(fresh());
  await redeem(CODE, dev(1), { ...deps(kv), server: "la" });
  await redeem(CODE, dev(2), { ...deps(kv), server: "sh" });
  const reg = await loadRegistry(kv);
  const la = authfile(reg, "la");
  const sh = authfile(reg, "sh");
  assert.equal(Object.keys(la).length, 1);
  assert.equal(Object.keys(sh).length, 1);
  for (const secret of Object.keys(la)) assert.equal(secret in sh, false);
});

test("a server is named by the token it presents; an unknown token names none", async () => {
  const list = await loadServers(withServers(fresh()), "");
  assert.equal(serverByToken(list, "tok-sh"), "sh");
  assert.equal(serverByToken(list, "tok-la"), "la");
  assert.equal(serverByToken(list, "nope"), undefined);
  assert.equal(serverByToken(list, "legacy", "legacy"), LEGACY_SERVER);
});

test("accounts minted before the list belong to the server that existed", async () => {
  const kv = fresh();
  await redeem(CODE, dev(1), deps(kv)); // no list yet
  withServers(kv);
  const reg = await loadRegistry(kv);
  assert.equal(serverOf(reg.accounts[dev(1)]), LEGACY_SERVER);
  assert.equal(Object.keys(authfile(reg, "la")).length, 0);
  assert.equal(Object.keys(authfile(reg, LEGACY_SERVER)).length, 1);
});

test("moving needs the device's own account, and keeps its port", async () => {
  const kv = withServers(fresh());
  const r = await redeem(CODE, dev(1), { ...deps(kv), server: "la" });
  const secret = String(r.body.secret);
  const port = r.body.port;

  const wrong = await move(dev(1), "d0000000:deadbeef", { kv, url: "", server: "sh" });
  assert.equal(wrong.status, 403);
  assert.equal(serverOf((await loadRegistry(kv)).accounts[dev(1)]), "la");

  const ok = await move(dev(1), secret, { kv, url: "", server: "sh" });
  assert.equal(ok.status, 200);
  assert.equal(ok.body.server, "sh");
  assert.equal(ok.body.port, port); // the port is the device's, wherever it sits
  const reg = await loadRegistry(kv);
  assert.equal(Object.keys(authfile(reg, "la")).length, 0);
  assert.equal(Object.keys(authfile(reg, "sh")).length, 1);
});

test("moving to a server this code may not use is refused", async () => {
  const kv = withServers(fresh());
  const r = await redeem(OTHER, dev(2), { ...deps(kv), server: "sh" });
  const bad = await move(dev(2), String(r.body.secret), { kv, url: "", server: "house" });
  assert.equal(bad.status, 404);
  assert.equal(serverOf((await loadRegistry(kv)).accounts[dev(2)]), "sh");
});

test("a malformed server list falls back to the configured server, never to nothing", async () => {
  const kv = fresh();
  kv.m.set(SERVERS_KEY, "{not json");
  const list = await loadServers(kv, "https://tunnel.example.dev");
  assert.deepEqual(list.map((s) => s.id), [LEGACY_SERVER]);
});
