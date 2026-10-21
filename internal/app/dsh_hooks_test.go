package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDSHNativePluginAndOwnedOverlay(t *testing.T) {
	t.Setenv("DSH_HOME", t.TempDir())
	inst := agentInstallers["dsh"]
	for range 2 {
		if err := updateDSHPlugin(inst, "/opt/my tools/gtmux", true); err != nil {
			t.Fatal(err)
		}
	}
	if !agentHooksInstalled(inst) {
		t.Fatal("native plugin not recognized")
	}
	y, _ := os.ReadFile(dshOverlayPath(inst))
	if !strings.Contains(string(y), "- insert:") || !strings.Contains(string(y), "file://") {
		t.Fatal(string(y))
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("DSH_HOME"), "profiles", "web", "cordis.patch.yml")); !os.IsNotExist(err) {
		t.Fatal("user profile changed")
	}
	// A foreign file is neither overwritten nor removed.
	foreign := []byte("user plugin")
	if err := os.WriteFile(inst.configPath(), foreign, 0600); err != nil {
		t.Fatal(err)
	}
	if updateDSHPlugin(inst, "/opt/gtmux", true) == nil || updateDSHPlugin(inst, "", false) == nil {
		t.Fatal("foreign plugin changed")
	}
	b, _ := os.ReadFile(inst.configPath())
	if string(b) != string(foreign) {
		t.Fatal("foreign plugin lost")
	}
}
func TestDSHNativeEventsUseNoCompatibilityHook(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node unavailable")
	}
	module := filepath.Join(t.TempDir(), "gtmux.mjs")
	if err := os.WriteFile(module, []byte(dshPluginJS("/opt/gtmux")), 0600); err != nil {
		t.Fatal(err)
	}
	script := `
import assert from 'node:assert/strict';
import {pathToFileURL} from 'node:url';
const {observe} = await import(pathToFileURL(process.argv[2]).href);
const listeners = {}, calls = [];
observe({on:(name,fn,options)=>{assert.equal(options.global,true);listeners[name]=fn;}},(token,payload)=>calls.push({token,payload}));
const root = {id:'root',header:{version:4,cwd:'/work'}};
const event = (type,data={},session=root)=>listeners['session/event'](session,{type,data});
event('user/message',{role:'user',source:{kind:'plugin'},content:[{type:'text',text:'private injected context'}]});
event('user/message',{role:'user',source:{kind:'user'},content:[{type:'text',text:'child'}]},{id:'child',header:{version:4,origin:'subagent'}});
listeners['session/event'](root,{type:'user/message',surfaceOp:{op:'replace',startSeq:0,endSeq:0},data:{role:'user',source:{kind:'user'},content:[{type:'text',text:'old user copy'}]}});
event('approval/asked',{toolName:'bash'});
event('tool/call',{name:'bash'});
assert.equal(calls.length,0);
event('user/message',{role:'user',source:{kind:'user'},content:[{type:'text',text:'Inspect'},{type:'reasoning',text:'private'}]});
event('turn/start',{turn:1});event('turn/end',{turn:1,reason:{kind:'completed'}});
event('turn/end',{turn:2,reason:{kind:'error'}});event('turn/end',{turn:3,reason:{kind:'aborted'}});
assert.deepEqual(calls.map(c=>c.token),['UserPromptSubmit','TurnStarted','Stop','StopFailure','Interrupt']);
assert.equal(calls[0].payload.prompt,'Inspect');assert.equal(calls[1].payload.turn_id,'1');
assert.ok(calls.every(c=>c.payload.session_id==='root'&&c.payload.cwd==='/work'));
let answered=false;
const answer={answers:[]};
assert.equal(await listeners['user-questions/request']({agent:{session:root}},async()=>{answered=true;return answer;}),answer);
assert.ok(answered);assert.equal(calls.length,5);
assert.equal(await listeners['user-questions/request']({agent:{session:root}},async()=>{await new Promise(r=>setTimeout(r,300));return answer;}),answer);
assert.deepEqual(calls.slice(-2).map(c=>c.token),['Question','Resumed']);
let finishA, finishB;
const first = listeners['user-questions/request']({agent:{session:root}},()=>new Promise(r=>{finishA=r;}));
const second = listeners['user-questions/request']({agent:{session:root}},()=>new Promise(r=>{finishB=r;}));
await new Promise(r=>setTimeout(r,300));
assert.equal(calls.at(-1).token,'Question');
finishA(answer);await first;assert.equal(calls.at(-1).token,'Question','another human question is still pending');
finishB(answer);await second;assert.equal(calls.at(-1).token,'Resumed');
`
	cmd := exec.Command(node, "--input-type=module", "-", module)
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("native contract: %v\n%s", err, out)
	}
}

func TestDSHNativeHookRetainsOriginForProfileCollection(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node unavailable")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "hook.mjs")
	probe := `#!/usr/bin/env node
import fs from 'node:fs';
fs.writeFileSync(process.env.GTMUX_TEST_SINK_OUT, JSON.stringify({
  args: process.argv.slice(2), parent: process.ppid, pane: process.env.TMUX_PANE,
  payload: JSON.parse(fs.readFileSync(0, 'utf8')),
}));
`
	if err := os.WriteFile(bin, []byte(probe), 0700); err != nil {
		t.Fatal(err)
	}
	module := filepath.Join(dir, "gtmux.mjs")
	if err := os.WriteFile(module, []byte(dshPluginJS(bin)), 0600); err != nil {
		t.Fatal(err)
	}
	script := `
import assert from 'node:assert/strict';
import fs from 'node:fs';
import {pathToFileURL} from 'node:url';
import {setTimeout as delay} from 'node:timers/promises';
const {hookSink} = await import(pathToFileURL(process.argv[2]).href);
process.env.TMUX_PANE = '%999';
for (const client of ['', 'dst']) {
  let dispose;
  const extra = client ? {terminal_client: client, client_pid: process.pid, client_instance: 'synthetic'} : {};
  const send = hookSink({on: (name, fn) => {assert.equal(name, 'dispose'); dispose = fn;}}, () => extra);
  try {
    send('UserPromptSubmit', {session_id: 'synthetic-session', cwd: '/synthetic', prompt: 'Inspect'});
    for (let i = 0; i < 100 && !fs.existsSync(process.env.GTMUX_TEST_SINK_OUT); i++) await delay(20);
    const result = JSON.parse(fs.readFileSync(process.env.GTMUX_TEST_SINK_OUT, 'utf8'));
    assert.deepEqual(result.args, ['hook', '--agent', 'dsh', 'UserPromptSubmit']);
    assert.equal(result.parent, process.pid, 'origin must still be the native plugin host');
    assert.equal(result.pane, '', 'shared Web ancestry must not grant pane ownership');
    assert.deepEqual(result.payload, {session_id: 'synthetic-session', cwd: '/synthetic', prompt: 'Inspect', ...extra});
  } finally { dispose(); }
  fs.unlinkSync(process.env.GTMUX_TEST_SINK_OUT);
}
`
	cmd := exec.Command(node, "--input-type=module", "-", module)
	cmd.Env = append(os.Environ(), "GTMUX_TEST_SINK_OUT="+filepath.Join(dir, "sink.json"))
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("native ancestry contract: %v\n%s", err, out)
	}
}

func TestDSHDoctorRepairsBothFilesWithoutOpenCodeMigration(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", filepath.Join(homeDir(), ".dsh"))
	if err := os.MkdirAll(os.Getenv("DSH_HOME"), 0700); err != nil {
		t.Fatal(err)
	}
	legacy := legacyOpencodePluginPath()
	foreignAgent := "// gtmux hook bridge for opencode\nexport const GtmuxHook = async () => ({});\n"
	if err := writeAgentPlugin(legacy, foreignAgent); err != nil {
		t.Fatal(err)
	}
	inst := agentInstallers["dsh"]
	s := &fixState{yes: true, bin: "/opt/gtmux"}
	if n := s.stepAgentHook("dsh"); n != 1 || s.rc != 0 {
		t.Fatalf("fresh repair: applied=%d rc=%d", n, s.rc)
	}
	if !agentHooksInstalled(inst) {
		t.Fatal("DSH doctor did not install both artifacts")
	}
	b, err := os.ReadFile(legacy)
	if err != nil || string(b) != foreignAgent {
		t.Fatal("DSH repair migrated an unrelated OpenCode plugin")
	}
	if n := s.stepAgentHook("dsh"); n != 0 {
		t.Fatal("healthy repair was not idempotent")
	}
	if err := os.Remove(dshOverlayPath(inst)); err != nil {
		t.Fatal(err)
	}
	if n := s.stepAgentHook("dsh"); n != 1 || s.rc != 0 {
		t.Fatal("missing overlay could not be repaired")
	}
}

func TestDSHDoctorRejectsStaleLaunchOverlay(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	inst := agentInstallers["dsh"]
	if err := updateDSHPlugin(inst, "/opt/gtmux", true); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dshOverlayPath(inst), []byte("# gtmux native DeepSeek Harness overlay\n# gtmux-native-dsh\n- insert: []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if rowAgentHook("dsh").status == stOK {
		t.Fatal("malformed launch overlay passed doctor")
	}
	s := &fixState{yes: true, bin: "/opt/gtmux"}
	if n := s.stepAgentHook("dsh"); n != 1 || s.rc != 0 {
		t.Fatal("owned stale overlay not repaired")
	}
}

func TestDSHDoctorPreservesForeignOverlayAndUserProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	inst := agentInstallers["dsh"]
	if err := updateDSHPlugin(inst, "/opt/old-gtmux", true); err != nil {
		t.Fatal(err)
	}
	moduleBefore, _ := os.ReadFile(inst.configPath())
	foreign := []byte("- insert: [{id: user-plugin}]\n")
	if err := os.WriteFile(dshOverlayPath(inst), foreign, 0600); err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(os.Getenv("DSH_HOME"), "profiles", "web", "cordis.patch.yml")
	if err := writeAgentPlugin(profile, "# user profile\n"); err != nil {
		t.Fatal(err)
	}
	s := &fixState{yes: true, bin: "/opt/gtmux"}
	if n := s.stepAgentHook("dsh"); n != 0 || s.rc != 1 {
		t.Fatal("foreign overlay reported repaired")
	}
	moduleAfter, _ := os.ReadFile(inst.configPath())
	overlayAfter, _ := os.ReadFile(dshOverlayPath(inst))
	userAfter, _ := os.ReadFile(profile)
	if string(moduleAfter) != string(moduleBefore) || string(overlayAfter) != string(foreign) || string(userAfter) != "# user profile\n" {
		t.Fatal("foreign/profile/module files changed on refused repair")
	}
}

func TestDSHNativeAccountUsageRejectsLateIdentityAndBoundsRequests(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node unavailable")
	}
	module := filepath.Join(t.TempDir(), "gtmux.mjs")
	if err := os.WriteFile(module, []byte(dshPluginJS("/opt/gtmux")), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, "testdata/dsh-account.mjs", module)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("native account contract: %v\n%s", err, out)
	}
}
