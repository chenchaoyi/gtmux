package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestOpenCodeV2PluginContract(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node unavailable")
	}
	dir := t.TempDir()
	capture := filepath.Join(dir, "capture")
	events := filepath.Join(dir, "events.jsonl")
	module := filepath.Join(dir, "gtmux.mjs")
	source := "#!" + node + "\nlet input='';process.stdin.on('data',d=>input+=d);process.stdin.on('end',async()=>{(await import('node:fs')).appendFileSync(" + strconv.Quote(events) + ",JSON.stringify({event:process.argv.at(-1),data:JSON.parse(input)})+'\\n')});\n"
	if err = os.WriteFile(capture, []byte(source), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(module, []byte(opencodePluginJS(capture)), 0600); err != nil {
		t.Fatal(err)
	}
	script := `
import assert from 'node:assert/strict';import fs from 'node:fs';import {pathToFileURL} from 'node:url';
const mod=await import(pathToFileURL(process.argv[2]).href);
assert.equal(mod.default.server,mod.GtmuxHook); // V1 loads one server definition; V2 calls setup.
const hooks={},toolHooks={};let push;const queue=[];let ended=false;
const stream={ [Symbol.asyncIterator](){return this},next(){if(queue.length)return Promise.resolve({value:queue.shift()});if(ended)return Promise.resolve({done:true});return new Promise(r=>push=r)},return(){ended=true;push?.({done:true});return Promise.resolve({done:true})}};
const emit=(type,data)=>{const value={type,data};if(push){const r=push;push=undefined;r({value})}else queue.push(value)};
const ctx={location:{directory:'/example'},session:{get:async({sessionID})=>{if(sessionID==='unknown')throw Error('unknown');return {id:sessionID,parentID:sessionID==='child'?'root':undefined,location:{directory:'/example'}}},hook:async(n,f)=>{hooks[n]=f}},tool:{hook:async(n,f)=>{toolHooks[n]=f}},event:{subscribe:()=>stream}};
const cleanup=await mod.default.setup(ctx);
await hooks.prompt({sessionID:'child',prompt:{text:'ignore child'}});
await hooks.prompt({sessionID:'root',prompt:{text:'work'}});
emit('session.execution.started',{sessionID:'root'});
await toolHooks['execute.before']({sessionID:'root',id:'one',tool:'read',input:{path:'a.go'}});
emit('session.tool.input.started',{sessionID:'root',id:'one',name:'read'});emit('session.tool.called',{sessionID:'root',id:'one',input:{path:'a.go'}});
emit('session.text.ended',{sessionID:'root',text:'done'});
emit('session.text.ended',{sessionID:'child',text:'child-secret'});
await hooks['experimental.idle']({sessionID:'root'});
await cleanup();await new Promise(r=>setTimeout(r,10));
const rows=fs.readFileSync(process.argv[3],'utf8').trim().split('\n').map(JSON.parse);
assert.deepEqual(rows.map(r=>r.event),['Profile','UserPromptSubmit','TurnStarted','Transcript','Transcript','Stop']);
assert.ok(rows.every(r=>r.data.session_id==='root'));assert.equal(rows.filter(r=>r.data.transcript?.kind==='tool').length,1);
assert.equal(rows.find(r=>r.data.transcript?.kind==='text').data.transcript.text,'done');
`
	cmd := exec.Command(node, "--input-type=module", "-", module, events)
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("V2 plugin: %v\n%s", err, out)
	}
}
