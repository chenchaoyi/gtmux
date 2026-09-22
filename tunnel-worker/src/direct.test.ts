// Direct per-device accounts (openspec/changes/direct-per-device-accounts). The shared
// DIRECT_SECRET let any holder bind any Mac's port and reach the VPS's loopback; these pin
// that an account can bind its own port and nothing else. node --test, no network.
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  crc32, preferredPort, assignPort, redeem, authfile, revokeCode, loadRegistry, sameSecret,
  REGISTRY_KEY, PORT_BASE, PORT_SPAN, type KV, type Registry,
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
