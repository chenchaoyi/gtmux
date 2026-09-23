#!/usr/bin/env bash
# direct-servers.sh — the Direct server list, which is DATA: adding a server is this
# script plus standing the box up, never a Worker deploy or a client release.
#
#   ./direct-servers.sh list
#   ./direct-servers.sh add <id> <https url> [region] [label-en] [label-zh]
#   ./direct-servers.sh set <id> accepting true|false
#   ./direct-servers.sh set <id> codes gtd-aaa,gtd-bbb   # reserve it for those codes ("" = anyone)
#   ./direct-servers.sh set <id> label-zh 洛杉矶          # the name users read (label-en too)
#   ./direct-servers.sh set <id> region us-west
#   ./direct-servers.sh set <id> url https://new.host    # if a server ever changes address
#   ./direct-servers.sh remove <id>
#   KV_TARGET=--local ./direct-servers.sh list           # rehearse against wrangler's local KV
#
# `add` generates that server's own sync token and prints it ONCE: it is what the server
# presents to fetch its account file, and each server gets its own so a compromised one
# exposes only its own tenants. Put it in that box's /etc/gtmux-tunnel/sync.env.
set -euo pipefail
cd "$(dirname "$0")"
target="${KV_TARGET:---remote}"
cmd="${1:?usage: direct-servers.sh list|add|set|remove …}"; shift || true
tmp="$(mktemp -d)"
trap 'command rm -rf "$tmp"' EXIT

npx wrangler kv key get --binding DIRECT_CODES servers:v1 "$target" >"$tmp/in.json" 2>/dev/null || true

node --experimental-strip-types --input-type=module -e '
  import { readFileSync, writeFileSync } from "node:fs";
  import { randomBytes } from "node:crypto";
  const [src, dst, cmd, ...rest] = process.argv.slice(1);
  let servers = [];
  try { const j = JSON.parse(readFileSync(src, "utf8")); servers = Array.isArray(j) ? j : (j.servers ?? []); } catch {}
  const find = (id) => servers.find((s) => s.id === id);
  const show = () => {
    if (!servers.length) { console.log("(no servers configured: the provisioner serves DIRECT_URL as the one server)"); return; }
    for (const s of servers) {
      const who = s.codes?.length ? `codes:${s.codes.length}` : "anyone";
      console.log(`${s.id.padEnd(10)} ${s.url.padEnd(34)} ${(s.region ?? "-").padEnd(12)} ${s.accepting === false ? "closed" : "taking"} ${who} ${s.sync ? "sync-set" : "NO SYNC TOKEN"}`);
    }
  };
  if (cmd === "list") { show(); process.exit(0); }
  if (cmd === "add") {
    const [id, url, region, en, zh] = rest;
    if (!id || !/^https:\/\//.test(url ?? "")) { console.error("usage: add <id> <https url> [region] [label-en] [label-zh]"); process.exit(2); }
    if (find(id)) { console.error(`server ${id} already exists`); process.exit(2); }
    const sync = randomBytes(24).toString("hex");
    servers.push({ id, url: url.replace(/\/+$/, ""), region: region || undefined, label: en || zh ? { en, zh } : undefined, accepting: true, sync });
    writeFileSync(dst, JSON.stringify({ servers }));
    console.log(`added ${id}`);
    console.log(`SYNC TOKEN (shown once, put it in that box: DIRECT_SYNC_TOKEN=${sync})`);
    process.exit(0);
  }
  if (cmd === "set") {
    const [id, field, value] = rest;
    const s = find(id);
    if (!s) { console.error(`no server ${id}`); process.exit(2); }
    if (field === "accepting") s.accepting = value === "true";
    else if (field === "codes") s.codes = value ? value.split(",").filter(Boolean) : undefined;
    else if (field === "region") s.region = value || undefined;
    else if (field === "label-en" || field === "label-zh") {
      // The name a user reads. Display only: it changes nothing about accounts, ports or
      // connections, and every client renders whatever this list returns on its next
      // listing, so a rename reaches installed Macs with no release.
      s.label = { ...(s.label ?? {}), [field === "label-en" ? "en" : "zh"]: value || undefined };
      if (!s.label.en && !s.label.zh) s.label = undefined;
    } else if (field === "url") s.url = (value || "").replace(/\/+$/, "");
    else { console.error("set <id> accepting|codes|region|label-en|label-zh|url <value>"); process.exit(2); }
    writeFileSync(dst, JSON.stringify({ servers }));
    console.log(`${id}: ${field} = ${value || "(cleared)"}`);
    process.exit(0);
  }
  if (cmd === "remove") {
    const [id] = rest;
    if (!find(id)) { console.error(`no server ${id}`); process.exit(2); }
    servers = servers.filter((s) => s.id !== id);
    writeFileSync(dst, JSON.stringify({ servers }));
    console.log(`removed ${id} (devices assigned to it keep their accounts; move them first)`);
    process.exit(0);
  }
  console.error(`unknown command ${cmd}`); process.exit(2);
' "$tmp/in.json" "$tmp/out.json" "$cmd" "$@"

if [ -s "$tmp/out.json" ]; then
  npx wrangler kv key put --binding DIRECT_CODES servers:v1 --path "$tmp/out.json" "$target" >/dev/null
  echo "server list saved"
fi
