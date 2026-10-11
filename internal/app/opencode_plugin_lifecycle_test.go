package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Execute the generated module against the SDK contract, not a second copy of
// its event mapping. Child sessions and the error->idle sequence were the bugs.
func TestOpencodePluginRootLifecycle(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node unavailable for plugin contract execution")
	}
	path := filepath.Join(t.TempDir(), "gtmux.mjs")
	if err := os.WriteFile(path, []byte(opencodePluginJS("/opt/gtmux")), 0600); err != nil {
		t.Fatal(err)
	}
	script := `
import assert from "node:assert/strict";
import {pathToFileURL} from "node:url";
const {GtmuxHook} = await import(pathToFileURL(process.argv[2]).href);
const calls = [];
const shell = (_strings, ...values) => ({quiet: () => { calls.push({token: values[2], payload: JSON.parse(values[0])}); return Promise.resolve(); }});
const client = {session: {get: async ({path}) => ({data: path.id === "unknown" ? undefined : {id: path.id, parentID: path.id === "child" ? "root" : undefined}})}};
const hook = await GtmuxHook({$:shell,client,directory:"/work"});
const event = (type, properties={}) => hook.event({event:{type,properties}});
await hook.event({});
await event("session.created", {info:{id:"root"}});
await event("session.created", {info:{id:"child",parentID:"root"}});
await event("session.status", {sessionID:"child",status:{type:"busy"}});
await event("session.error", {sessionID:"child",error:{name:"MessageAbortedError"}});
assert.deepEqual(calls.map(c=>c.token), ["SessionStart"]);
await event("session.status", {sessionID:"root",status:{type:"busy"}});
await event("permission.asked", {sessionID:"root"});
await event("permission.replied", {sessionID:"root"});
await event("session.error", {sessionID:"root",error:{name:"MessageAbortedError"}});
await event("session.status", {sessionID:"root",status:{type:"idle"}});
await event("session.idle", {sessionID:"root"});
assert.deepEqual(calls.map(c=>c.token), ["SessionStart","TurnStarted","PermissionRequest","Resumed","Interrupt"]);
await hook["chat.message"]({}, {message:{sessionID:"root"},parts:[{type:"text",text:"new work"}]});
await event("message.updated", {info:{id:"msg",sessionID:"root",role:"assistant"}});
await event("message.part.updated", {part:{id:"part",messageID:"msg",sessionID:"root",type:"text",text:"done"}});
await event("session.idle", {sessionID:"root"});
await event("session.status", {sessionID:"root",status:{type:"idle"}});
assert.equal(calls.filter(c=>c.token==="Stop").length,1);
assert.equal(calls.at(-1).payload.assistant,"done");
await event("session.status", {sessionID:"root",status:{type:"busy"}});
await event("session.error", {sessionID:"root",error:{name:"APIError",data:{message:"offline"}}});
await event("session.idle", {sessionID:"root"});
assert.equal(calls.at(-1).token,"StopFailure");
assert.equal(calls.at(-1).payload.error,"offline");
await event("session.status", {sessionID:"unknown",status:{type:"busy"}});
await hook["chat.message"]({}, {message:{sessionID:"child"},parts:[{type:"text",text:"child"}]});
assert.equal(calls.at(-1).token,"StopFailure");
assert.ok(calls.every(c=>c.payload.session_id==="root" && c.payload.cwd==="/work"));
`
	cmd := exec.Command(node, "--input-type=module", "-", path)
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated plugin: %v\n%s", err, out)
	}
}
