package agentclient

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

const gateToken = "a1b2c3d4a1b2c3d4a1b2c3d4a1b2c3d4"

func TestSPL005TerminalInputStoreGate(t *testing.T) {
	t.Run("persistent_go_records", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		root := state.Dir()
		legacy := filepath.Join(root, legacyInputDir)
		active := filepath.Join(root, activeInputDir)
		if InputDir() != legacy {
			t.Fatal("fresh installation did not select v1")
		}
		if _, err := os.Lstat(root); !os.IsNotExist(err) {
			t.Fatalf("read-only preflight created state: %v", err)
		}
		now := time.Now()
		target := Attachment{Version: ProtocolVersion, Agent: "codex", Pane: "%11", PID: 123, InputReady: true,
			Instance: strings.Repeat("b", 32), HomeID: "home", SessionID: "session", ServerEpoch: "3", BindingEpoch: 12}
		makeRequest := func(deliveryID string) Request {
			t.Helper()
			req, err := Prepare(target, "synthetic "+deliveryID, deliveryID, false, now)
			if err != nil {
				t.Fatal(err)
			}
			return req
		}
		pending, unknown, accepted := makeRequest("pending"), makeRequest("unknown"), makeRequest("accepted")
		for _, req := range []Request{unknown, accepted} {
			if err := ClaimNative(req); err != nil {
				t.Fatal(err)
			}
		}
		if err := FinishNative(accepted, "accepted", ""); err != nil {
			t.Fatal(err)
		}
		oldBytes := gateSnapshot(t, legacy)
		activate := stageGateFixture(t, root)
		if InputDir() != "" {
			t.Fatal("unactivated spool is usable")
		}
		if _, err := Prepare(target, pending.Text, "pending", false, now); !errors.Is(err, ErrInputStoreUnavailable) {
			t.Fatalf("prepared stage admitted input: %v", err)
		}
		activate("active")
		if InputDir() != active {
			t.Fatal("persisted active layout did not select moved v1 data")
		}
		if err := ClaimNative(pending); !errors.Is(err, ErrInputStoreUnavailable) {
			t.Fatalf("old pending handler crossed the cutover: %v", err)
		}
		if err := FinishNative(unknown, "accepted", ""); !errors.Is(err, ErrInputStoreUnavailable) {
			t.Fatalf("old completion crossed the cutover: %v", err)
		}

		dsh := target
		dsh.Agent = "dsh"
		if _, err := Prepare(dsh, pending.Text, "pending", false, now); !errors.Is(err, ErrInputStoreUnavailable) {
			t.Fatalf("old DSH attachment entered new generation: %v", err)
		}
		dsh.InputStoreGeneration = gateToken
		current, err := Prepare(dsh, pending.Text, "pending", false, now)
		if err != nil || current.ID != pending.ID {
			t.Fatalf("new DSH did not retain original pending identity: %+v, %v", current, err)
		}
		if _, err := Prepare(target, unknown.Text, "unknown", false, now); !errors.Is(err, ErrInputUncertain) {
			t.Fatalf("unconfirmed claim gained permission to replay: %v", err)
		}
		again := makeRequest("accepted")
		if receipt, ok := ReadReceipt(again); again.ID != accepted.ID || !ok || receipt.State != "accepted" {
			t.Fatalf("accepted action did not retain identity and outcome: %+v", receipt)
		}
		gateEqual(t, oldBytes, gateSnapshot(t, active))

		if err := ClaimNative(current); err != nil {
			t.Fatal(err)
		}
		claimed := gateSnapshot(t, active)
		activate("rolling-back")
		if err := FinishNative(current, "accepted", ""); !errors.Is(err, ErrInputStoreUnavailable) {
			t.Fatalf("completion wrote while gate was unavailable: %v", err)
		}
		gateEqual(t, claimed, gateSnapshot(t, active))
		activate("active")
		if err := FinishNative(current, "accepted", ""); err != nil {
			t.Fatal(err)
		}
		if receipt, ok := ReadReceipt(current.persisted()); !ok || receipt.State != "accepted" {
			t.Fatalf("fresh read did not find restored receipt: %+v", receipt)
		}
	})
	t.Run("go_and_shipped_dsh", func(t *testing.T) {
		node, err := exec.LookPath("node")
		if err != nil {
			t.Skip("Node is required for DSH receiver validation")
		}
		module := gatePluginFixture(t)
		python, err := exec.LookPath("python3")
		if err != nil {
			t.Skip("Python is required for preflight parity")
		}
		for _, scenario := range []string{"legacy", "active", "prepared", "missing-journal", "missing-marker",
			"bad-identity", "bad-journal", "aliased-field", "linked-marker", "duplicate-root", "escaped-nested-key",
			"duplicate-marker", "trailing-value", "bad-utf8", "bad-unicode-escape", "wrong-type"} {
			t.Run(scenario, func(t *testing.T) {
				t.Setenv("HOME", t.TempDir())
				root := state.Dir()
				legacy := filepath.Join(root, legacyInputDir)
				active := filepath.Join(root, activeInputDir)
				journal := filepath.Join(root, inputStoreJournal)
				if err := os.MkdirAll(legacy, 0700); err != nil {
					t.Fatal(err)
				}
				receiveDir := legacy
				switch scenario {
				case "legacy":
				default:
					activate := stageGateFixture(t, root)
					receiveDir = active
					if scenario != "missing-journal" && scenario != "prepared" {
						activate("active")
					}
					switch scenario {
					case "missing-journal":
						if err := os.Remove(journal); err != nil {
							t.Fatal(err)
						}
					case "missing-marker":
						if err := os.Remove(legacy); err != nil {
							t.Fatal(err)
						}
					case "bad-journal":
						if err := os.WriteFile(journal, []byte("{"), 0600); err != nil {
							t.Fatal(err)
						}
					case "bad-identity", "aliased-field":
						var record map[string]json.RawMessage
						if err := readOwnedJSON(journal, &record, 4096); err != nil {
							t.Fatal(err)
						}
						if scenario == "bad-identity" {
							record["directory"] = json.RawMessage(`{"dev":"0","ino":"0"}`)
						} else {
							record["Version"] = record["version"]
							delete(record, "version")
						}
						writeGateJSON(t, journal, record)
					case "duplicate-root", "escaped-nested-key", "duplicate-marker", "trailing-value", "bad-utf8", "bad-unicode-escape", "wrong-type":
						path := journal
						if scenario == "duplicate-marker" {
							path = legacy
						}
						data, err := os.ReadFile(path)
						if err != nil {
							t.Fatal(err)
						}
						canonical := string(data)
						var invalid string
						switch scenario {
						case "duplicate-root", "duplicate-marker":
							invalid = `{"version":1,` + canonical[1:]
						case "escaped-nested-key":
							invalid = strings.Replace(canonical, `"directory":{`, `"directory":{"d\u0065v":"duplicate",`, 1)
						case "trailing-value":
							invalid = canonical + `{}`
						case "bad-utf8":
							invalid = strings.Replace(canonical, gateToken, string([]byte{0xff}), 1)
						case "bad-unicode-escape":
							invalid = strings.Replace(canonical, gateToken, `\ud800`, 1)
						case "wrong-type":
							invalid = strings.Replace(canonical, `"version":1`, `"version":true`, 1)
						}
						if invalid == canonical {
							t.Fatal("fixture did not mutate the metadata")
						}
						if err := os.WriteFile(path, []byte(invalid), 0600); err != nil {
							t.Fatal(err)
						}
					case "linked-marker":
						if err := os.Link(legacy, filepath.Join(root, "marker-link")); err != nil {
							t.Fatal(err)
						}
					}
				}
				want := ""
				if scenario == "legacy" || scenario == "active" {
					want = receiveDir
				}
				if got := InputDir(); got != want {
					t.Fatalf("Go chose %q, want %q", got, want)
				}
				before := gateSnapshot(t, receiveDir)
				if want == "" {
					_, err := Prepare(Attachment{Version: ProtocolVersion, InputReady: true, Pane: "%53"}, "synthetic", "blocked", false, time.Now())
					if !errors.Is(err, ErrInputStoreUnavailable) {
						t.Fatalf("blocked Go layout returned %v", err)
					}
					if _, err := os.Lstat(filepath.Join(root, "lease-terminal-input.lock")); !os.IsNotExist(err) {
						t.Fatalf("preflight created a lease in a blocked store: %v", err)
					}
				}
				preflight := exec.Command(python, "-B", "../../docs/design/input-store-preflight.py", root)
				out, err := preflight.Output()
				var report struct{ Layout string }
				if decodeErr := json.Unmarshal(out, &report); decodeErr != nil {
					t.Fatalf("preflight returned no layout report: %v, %v", err, decodeErr)
				}
				if want == "" {
					var rejected *exec.ExitError
					if !errors.As(err, &rejected) || rejected.ExitCode() != 2 || report.Layout != "unverified" {
						t.Fatalf("preflight accepted blocked layout: %+v, %v", report, err)
					}
				} else if err != nil || report.Layout != scenario {
					t.Fatalf("preflight disagreed with Go on valid layout: %+v, %v", report, err)
				}
				cmd := exec.Command(node, "--input-type=module", "-e", gateReceiver, module, root, want, gateToken)
				cmd.Env = []string{"HOME=" + os.Getenv("HOME"), "PATH=" + filepath.Join(filepath.Dir(filepath.Dir(module)), "bin"),
					"TMUX_PANE=%53", "NODE_DISABLE_COMPILE_CACHE=1"}
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("shipped DSH receiver disagreed: %v\n%s", err, out)
				}
				if want == "" {
					gateEqual(t, before, gateSnapshot(t, receiveDir))
				}
			})
		}
	})
}

// Simulate the durable exchange result, with old writers already drained.
// The production selectors have no authority to exchange or update these paths.
func stageGateFixture(t *testing.T, root string) func(phase string) {
	t.Helper()
	legacy := filepath.Join(root, legacyInputDir)
	active := filepath.Join(root, activeInputDir)
	if err := os.Rename(legacy, active); err != nil {
		t.Fatal(err)
	}
	writeGateJSON(t, legacy, map[string]any{"version": 1, "token": gateToken})
	identity := func(path string) map[string]string {
		t.Helper()
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		id, _, err := privateInputIdentity(info)
		if err != nil {
			t.Fatal(err)
		}
		return map[string]string{"dev": id.dev, "ino": id.ino}
	}
	dir, marker := identity(active), identity(legacy)
	activate := func(phase string) {
		t.Helper()
		writeGateJSON(t, filepath.Join(root, inputStoreJournal), map[string]any{"version": 1,
			"phase": phase, "token": gateToken, "directory": dir, "marker": marker})
	}
	activate("prepared")
	return activate
}

func writeGateJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := atomicPrivate(path, data); err != nil {
		t.Fatal(err)
	}
}

func gateSnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]string, len(entries))
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		result[entry.Name()] = string(data)
	}
	return result
}

func gateEqual(t *testing.T, before, after map[string]string) {
	t.Helper()
	if len(before) != len(after) {
		t.Fatalf("file count changed: %d to %d", len(before), len(after))
	}
	for name, data := range before {
		if after[name] != data {
			t.Fatalf("unexpected change: %s", name)
		}
	}
}

func gatePluginFixture(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	source, err := os.ReadFile(filepath.Join("..", "app", "dsh_terminal_plugin.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	module := filepath.Join(base, "dst", "bridge.mjs")
	for _, name := range []string{"dst", "bin"} {
		if err := os.Mkdir(filepath.Join(base, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string][]byte{
		module:                             source,
		filepath.Join(base, "gtmux.mjs"):   []byte("export const observe = () => {}; export const hookSink = () => () => {};"),
		filepath.Join(base, "bin", "tmux"): []byte("#!/bin/sh\nprintf '317\\n'\n"),
	}
	for name, contents := range files {
		if err := os.WriteFile(name, contents, 0700); err != nil {
			t.Fatal(err)
		}
	}
	return module
}

const gateReceiver = `
import assert from 'node:assert/strict';
import { mkdirSync, readFileSync, writeFileSync, existsSync } from 'node:fs';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
const [module, root, route, token] = process.argv.slice(1);
mkdirSync(join(process.env.HOME, '.dsh'), {mode:0o700});
const data = join(process.env.HOME, '.dsh-tui');
mkdirSync(join(data, 'inject'), {recursive:true, mode:0o700});
const write = (path, value) => writeFileSync(path, JSON.stringify(value), {mode:0o600});
const session = {id:'fixture-dsh', header:{version:4}};
write(join(data, 'inject', 'servers.json'), [{pid:process.pid,sessionId:session.id,startedAt:Date.now()-1000}]);
write(join(data, 'session-mounts.json'), {version:1,owners:[{pid:process.pid,sessionIds:[session.id]}]});
let receive, dispose, sent=0;
let hasGrant=true, hasForeground=true;
const host = {grants:{allows:()=>hasGrant}, registerCommand(_ctx,_key,config) { receive=config.handler; return ()=>{}; }};
let currentHost=host;
const ctx = {get:key=>key==='tuiPluginHost'?currentHost:key==='sessions'?(hasForeground?new Map([[session.id,session]]):new Map()):undefined,
             on:(key,cb)=>{if(key==='dispose') dispose=cb;}};
const {apply} = await import(pathToFileURL(module));
apply(ctx);
if (!route) {
  assert.equal(receive,undefined);
} else {
  assert.equal(typeof receive,'function');
  const readLease=()=>JSON.parse(readFileSync(join(root,'terminal-clients',process.pid+'.json')));
  const lease=readLease();
  const active=route===join(root,'terminal-input-v2');
  assert.equal(lease.input_store_generation,active?token:'');
  const {instance,home_id,session_id,pid,pane,server_epoch,binding_epoch}=lease;
  const prepare=id=>{
    const request={version:1,instance,home_id,session_id,pid,pane,server_epoch,binding_epoch,
      request_id:id,created_at:Date.now(),text:'合成验证 😀'};
    write(join(route,id+'.pending.json'), request);
    write(join(route,id+'.index.json'), {...request,text:''});
  };
  const invoke=id=>receive({rawInput:id,agent:{session,followup:m=>{assert.equal(m.id,id);sent++;}},signal:new AbortController().signal});
  const id='d'.repeat(32);
  prepare(id);
  assert.equal(invoke(id).kind,'success');
  assert.equal(invoke(id).kind,'success');
  assert.equal(sent,1);
  assert.equal(JSON.parse(readFileSync(join(route,id+'.receipt.json'))).state,'accepted');
  const expectBlocked=blocked=>{
    assert.equal(invoke(blocked).kind,'error');
    assert.equal(sent,1);
    assert.equal(existsSync(join(route,blocked+'.pending.json')),true);
    assert.equal(existsSync(join(route,blocked+'.claimed.json')),false);
    assert.equal(existsSync(join(route,blocked+'.receipt.json')),false);
  };
  const blocked='e'.repeat(32);
  prepare(blocked);
  currentHost=undefined;
  expectBlocked(blocked);
  assert.equal(existsSync(join(root,'terminal-clients',process.pid+'.json')),false);
  currentHost=host;
  hasGrant=false;
  expectBlocked(blocked);
  assert.equal(readLease().input_ready,false);
  hasGrant=true;
  hasForeground=false;
  expectBlocked(blocked);
  hasForeground=true;
  if (active) {
    const journal=join(root,'terminal-input-cutover.json');
    const legacy=join(root,'terminal-input');
    const record=JSON.parse(readFileSync(journal));
    const newToken='f'.repeat(32);
    write(legacy,{version:1,token:newToken});
    write(journal,{...record,token:newToken});
    expectBlocked(blocked);
    assert.equal(readLease().input_ready,false);
  }
  const oldHandler=receive;
  dispose();
  assert.equal(oldHandler({rawInput:blocked,agent:{session,followup:()=>{sent++;}}}).kind,'error');
  assert.equal(sent,1);
  dispose=undefined;
}
dispose?.();
`

func TestSPL007VerifiedAttachmentChecksCurrentInputGeneration(t *testing.T) {
	dir, a, inspect := attachmentFixture(t)
	root := state.Dir()
	ready := func(generation string, grant, want bool) {
		t.Helper()
		a.InputStoreGeneration, a.InputReady = generation, grant
		putJSON(t, filepath.Join(dir, "99.json"), a)
		attachments := Load(dir, inspect)
		if len(attachments) != 1 || attachments[0].ReadyForInput() != want {
			t.Fatalf("generation %q grant=%v ready=%v want=%v", generation, grant, attachments, want)
		}
	}
	ready("", true, false)
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatalf("readiness created state: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, legacyInputDir), 0700); err != nil {
		t.Fatal(err)
	}
	ready("", true, true)
	ready(gateToken, true, false)
	activate := stageGateFixture(t, root)
	ready("", true, false)
	activate("active")
	ready("", true, false)
	ready(strings.Repeat("d", 32), true, false)
	ready(gateToken, false, false)
	ready(gateToken, true, true)
	activate("rolling-back")
	ready(gateToken, true, false)
	if _, err := os.Lstat(filepath.Join(root, "lease-terminal-input.lock")); !os.IsNotExist(err) {
		t.Fatalf("readiness took a write lease: %v", err)
	}
}
