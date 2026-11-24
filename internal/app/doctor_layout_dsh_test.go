package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestGSS004GroupedLayoutPreservesDSHRegistration(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node unavailable")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DSH_HOME", filepath.Join(home, ".dsh"))
	t.Setenv("TMUX_PANE", "%55")
	bin := filepath.Join(home, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	fakeCLI := filepath.Join(bin, "gtmux-fixture")
	for path, body := range map[string]string{
		filepath.Join(bin, "tmux"): "#!/bin/sh\nprintf '88\\n'\n",
		fakeCLI:                    "#!/bin/sh\nexit 0\n",
	} {
		if err := os.WriteFile(path, []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := state.ChangeStorageLayout(true, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	installer := agentInstallers["dsh"]
	if err := updateDSHPlugin(installer, fakeCLI, true); err != nil {
		t.Fatal(err)
	}
	module := filepath.Join(filepath.Dir(installer.configPath()), "terminal-dst", "bridge.mjs")
	const script = `
import {mkdirSync, writeFileSync} from 'node:fs';
import {homedir} from 'node:os';
import {join} from 'node:path';
import {pathToFileURL} from 'node:url';
const {apply} = await import(pathToFileURL(process.argv[1]));
const root = join(homedir(), '.dsh-tui');
const id = 'layout-dsh-session';
mkdirSync(join(root, 'inject'), {recursive: true, mode: 0o700});
const put = (file, value) => writeFileSync(join(root, file), JSON.stringify(value), {mode: 0o600});
put('inject/servers.json', [{pid: process.pid, sessionId: id, startedAt: Date.now()}]);
put('session-mounts.json', {version: 1, owners: [{pid: process.pid, sessionIds: [id]}]});
const host = {registerCommand() { return () => {}; }, grants: {allows() { return true; }}};
const sessions = new Map([[id, {id, header: {version: 4, origin: 'cli'}}]]);
apply({on() {}, get(service) { return service === 'tuiPluginHost' ? host : service === 'sessions' ? sessions : undefined; }});
process.stdout.write(String(process.pid));
`
	out, err := exec.Command(node, "--input-type=module", "-e", script, module).CombinedOutput()
	if err != nil {
		t.Fatalf("plugin fixture: %v\n%s", err, out)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatalf("plugin PID: %q", out)
	}
	if err := state.CheckStorageLayout(); err != nil {
		t.Fatalf("plugin must not create a layout conflict: %v", err)
	}
	attachments := agentclient.Load(agentclient.Dir(), agentclient.Inspector{
		Now:     time.Now(),
		Process: func(int) (int, string, error) { return pid, "/node/@deepseek-ai/dsh/lib/bin.js --profile dsh-tui", nil },
		Pane:    func(string) (int, string, error) { return pid, "88", nil },
		Session: func(agent, home, id string) bool { return agent == "dsh" && home != "" && id == "layout-dsh-session" },
	})
	if len(attachments) != 1 || attachments[0].PID != pid || !attachments[0].ReadyForInput() {
		t.Fatalf("Go did not read the real plugin attachment: %+v", attachments)
	}
	if _, err := os.Stat(filepath.Join(state.Dir(), "layout-v2", "clients", "terminal-clients")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected grouped registration path: %v", err)
	}
}
