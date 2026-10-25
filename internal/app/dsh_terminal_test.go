package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDSHTerminalNativeRequestContract(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node unavailable")
	}
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("GTMUX_LANG", "en")
	inst := agentInstallers["dsh"]
	t.Setenv("DSH_HOME", filepath.Join(root, ".dsh"))
	if err := updateDSHPlugin(inst, "/opt/gtmux", true); err != nil {
		t.Fatal(err)
	}
	module := filepath.Join(filepath.Dir(inst.configPath()), "terminal-dst", "bridge.mjs")
	script := `
import assert from 'node:assert/strict';
import {pathToFileURL} from 'node:url';
import {mkdirSync,writeFileSync,readFileSync,existsSync,symlinkSync,chmodSync,statSync} from 'node:fs';
import {join} from 'node:path';
const {receive,foreground,apply}=await import(pathToFileURL(process.argv[2]));
const root=process.argv[3];
process.umask(0);
const dir=join(root,'requests');mkdirSync(dir,{mode:0o700});
const now=1900000000000;
const target={instance:'a'.repeat(32),pid:100,home_id:'home',session_id:'expected',binding_epoch:7,server_epoch:'9',pane:'%44'};
const queued=[];
const agent=id=>({session:{id,header:{version:4}},followup:message=>queued.push({id,message})});
const make=(id,extra={})=>{
 const request={version:1,request_id:id,...target,created_at:now,text:'Inspect the synthetic fixture',...extra};
 writeFileSync(join(dir,id+'.pending.json'),JSON.stringify(request),{mode:0o600});return request;
};
const invoke=(id,receiving='expected',expected=target)=>receive(dir,expected,{rawInput:' '+id,agent:agent(receiving),signal:new AbortController().signal},now);
const wrong='1'.repeat(32);make(wrong);
assert.equal(invoke(wrong,'different').kind,'error');assert.equal(queued.length,0);
assert.equal(existsSync(join(dir,wrong+'.claimed.json')),false);
const good='2'.repeat(32);make(good);
assert.equal(invoke(good).kind,'success');assert.equal(queued.length,1);
for (const suffix of ['.claimed.json','.receipt.json']) assert.equal(statSync(join(dir,good+suffix)).mode & 0o777,0o600);
assert.equal(queued[0].id,'expected');assert.equal(queued[0].message.id,good);
assert.deepEqual(queued[0].message.source,{kind:'user'});
assert.equal(queued[0].message.content[0].text,'Inspect the synthetic fixture');
assert.equal(existsSync(join(dir,good+'.pending.json')),false);
assert.equal(invoke(good).kind,'success');assert.equal(queued.length,1,'retry must not enqueue twice');
for(const [n,extra] of [[3,{binding_epoch:6}],[4,{instance:'b'.repeat(32)}],[5,{home_id:'other'}],[6,{pid:101}],[7,{created_at:now-16000}]]){
 const id=String(n).repeat(32);make(id,extra);assert.equal(invoke(id).kind,'error');
}
assert.equal(queued.length,1);
const claimed='8'.repeat(32);make(claimed);writeFileSync(join(dir,claimed+'.claimed.json'),'',{mode:0o600});
assert.equal(invoke(claimed).kind,'error');assert.equal(queued.length,1,'unknown claim must not retry');
assert.equal(receive(dir,target,{rawInput:'../escape',agent:agent('expected')},now).kind,'error');
const linked='9'.repeat(32), foreign=join(root,'foreign.json');writeFileSync(foreign,'{}',{mode:0o600});
symlinkSync(foreign,join(dir,linked+'.pending.json'));
assert.equal(invoke(linked).kind,'error');assert.equal(readFileSync(foreign,'utf8'),'{}');
const meta=join(root,'metadata');mkdirSync(join(meta,'inject'),{recursive:true,mode:0o700});
const put=(path,value)=>writeFileSync(path,JSON.stringify(value),{mode:0o600});
put(join(meta,'inject','servers.json'),[{pid:100,sessionId:'expected',startedAt:7}]);
put(join(meta,'session-mounts.json'),{version:1,owners:[{pid:100,sessionIds:['expected','background']}]});
assert.equal(foreground(meta,100).session_id,'expected');
put(join(meta,'session-mounts.json'),{version:1,owners:[{pid:100,sessionIds:['expected','codex:expected']}]});
assert.equal(foreground(meta,100),undefined,'backend collision must not borrow DSH identity');
// A shipped plugin is a gtmux writer even though an agent hosts its process.
// Refuse a wide managed root before making bridge directories or running tmux.
const home=join(root,'plugin-home'), managed=join(home,'.local','share','gtmux');
mkdirSync(managed,{recursive:true,mode:0o700});chmodSync(managed,0o755);
process.env.HOME=home;process.env.TMUX_PANE='%44';process.env.PATH='/no-fixture-executables';
apply({});
assert.equal(existsSync(join(managed,'terminal-clients')),false,'wide root must refuse publication');
assert.equal(existsSync(join(managed,'terminal-input')),false);

`
	cmd := exec.Command(node, "--input-type=module", "-", module, root)
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("native terminal contract: %v\n%s", err, out)
	}
}

func TestDSHTerminalArtifactPreflightPreservesForeignFiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	inst := agentInstallers["dsh"]
	if err := updateDSHPlugin(inst, "/opt/gtmux", true); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(inst.configPath())
	path := filepath.Join(filepath.Dir(inst.configPath()), "terminal-dst", "dsh-plugin.json")
	if err := os.WriteFile(path, []byte(`{"id":"foreign"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if updateDSHPlugin(inst, "/different/gtmux", true) == nil || updateDSHPlugin(inst, "", false) == nil {
		t.Fatal("foreign native component was replaced or removed")
	}
	after, _ := os.ReadFile(inst.configPath())
	if string(before) != string(after) {
		t.Fatal("preflight modified another artifact before refusal")
	}
}
