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
