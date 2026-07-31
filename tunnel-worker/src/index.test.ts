// Unit test for the reaper's decision function (the abuse-cleanup logic). Pure, no
// network: pins WHICH tunnels the daily cron deletes so a threshold edit can't silently
// start reaping live tunnels. Zero deps: node's built-in test runner with type-strip —
//   node --experimental-strip-types --test src/index.test.ts
import { test } from "node:test";
import assert from "node:assert/strict";
import { shouldReap } from "./index.ts";

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
