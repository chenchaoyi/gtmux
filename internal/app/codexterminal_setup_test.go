package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexTerminalShellBlockPreservesAliasesFunctionsAndArguments(t *testing.T) {
	for _, shell := range []string{"/bin/bash", "/bin/zsh"} {
		if _, err := os.Stat(shell); err != nil {
			continue
		}
		for _, mode := range []string{"native", "alias", "function"} {
			t.Run(filepath.Base(shell)+"/"+mode, func(t *testing.T) {
				t.Setenv("HOME", t.TempDir())
				bin := t.TempDir()
				capture := filepath.Join(t.TempDir(), "args")
				t.Setenv("CAPTURE", capture)
				t.Setenv("TMUX", "synthetic-server")
				t.Setenv("PROMPT", "literal $(touch SHOULD_NOT_EXIST) "+string(rune(96))+"words"+string(rune(96))+" with spaces")
				t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
				for _, name := range []string{"gtmux", "codex", "native-codex"} {
					body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$CAPTURE\"\n"
					if name != "gtmux" {
						body = "#!/bin/sh\nprintf 'native\\n' > \"$CAPTURE\"\n"
					}
					if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0700); err != nil {
						t.Fatal(err)
					}
				}
				original := "# user configuration\n"
				if mode == "alias" {
					original += "alias codex='native-codex'\n"
				}
				if mode == "function" {
					original += "function codex { printf 'native\\n' > \"$CAPTURE\"; }\n"
				}
				rc := filepath.Join(t.TempDir(), "bashrc")
				if err := os.WriteFile(rc, []byte(original), 0600); err != nil {
					t.Fatal(err)
				}
				if err := configureCodexTerminal(rc, filepath.Join(bin, "gtmux"), true); err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command(shell, "--noprofile", "--rcfile", rc, "-ic", "codex \"$PROMPT\"")
				if filepath.Base(shell) == "zsh" {
					cmd = exec.Command(shell, "-f", "-ic", "source \"$TEST_RC\"; codex \"$PROMPT\"")
					t.Setenv("TEST_RC", rc)
				}
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatal(err, string(out))
				}
				got, err := os.ReadFile(capture)
				if err != nil {
					t.Fatal(err)
				}
				want := "native\n"
				if mode == "native" {
					want = "codex\n--\n" + os.Getenv("PROMPT") + "\n"
				}
				if string(got) != want {
					t.Fatalf("shell launch changed arguments/override: %q", got)
				}
				if err := configureCodexTerminal(rc, "", false); err != nil {
					t.Fatal(err)
				}
				after, _ := os.ReadFile(rc)
				if string(after) != original {
					t.Fatal("uninstall changed user shell configuration", string(after))
				}
			})
		}
	}
}

func TestCodexTerminalSetupRefusesBrokenManagedBlock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	p := filepath.Join(t.TempDir(), "rc")
	original := "keep user settings\n" + codexTerminalBegin + "\nkeep incomplete section\n"
	if err := os.WriteFile(p, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := configureCodexTerminal(p, "/synthetic/gtmux", true); err == nil {
		t.Fatal("broken section was overwritten")
	}
	after, _ := os.ReadFile(p)
	if string(after) != original {
		t.Fatal("user settings changed")
	}
}

func TestCodexColdStartUsesOnlyOfficialStartAndPreservesForeignSocket(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	capture := filepath.Join(t.TempDir(), "args")
	t.Setenv("CAPTURE", capture)
	bin := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$CAPTURE\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	old := codexDaemonProbe
	t.Cleanup(func() { codexDaemonProbe = old })
	n := 0
	codexDaemonProbe = func() bool { n++; return n > 1 }
	if !ensureCodexDaemon(bin) {
		t.Fatal("cold start failed")
	}
	got, _ := os.ReadFile(capture)
	if string(got) != "app-server\ndaemon\nstart\n" {
		t.Fatal("daemon operation changed", string(got))
	}
	if err := os.Remove(capture); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(codexHome(), "app-server-control")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/synthetic/forwarded.sock", filepath.Join(dir, "app-server-control.sock")); err != nil {
		t.Fatal(err)
	}
	codexDaemonProbe = func() bool { return false }
	if ensureCodexDaemon(bin) {
		t.Fatal("foreign socket adopted")
	}
	if _, err := os.Stat(capture); !os.IsNotExist(err) {
		t.Fatal("foreign socket caused daemon mutation")
	}
}

func TestCodexNativeFallbackPreservesOriginalArgv(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	bin := t.TempDir()
	capture := filepath.Join(t.TempDir(), "args")
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$CAPTURE\"\nexit 23\n"), 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestCodexNativeFallbackHelper$")
	cmd.Env = append(os.Environ(), "GTMUX_TEST_CODEX_FALLBACK=1", "CAPTURE="+capture, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	err := cmd.Run()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 23 {
		t.Fatal("native exit behavior changed", err)
	}
	got, _ := os.ReadFile(capture)
	if string(got) != "--no-daemon\nresume\nthread with spaces\n--\n--lang=en\nliteral $(touch SHOULD_NOT_EXIST)\n" {
		t.Fatal("native arguments changed", string(got))
	}
}

func TestCodexNativeFallbackHelper(t *testing.T) {
	if os.Getenv("GTMUX_TEST_CODEX_FALLBACK") != "1" {
		return
	}
	os.Exit(Run([]string{"--lang=zh", "codex", "--", "--no-daemon", "resume", "thread with spaces", "--", "--lang=en", "literal $(touch SHOULD_NOT_EXIST)"}))
}

func TestShellBlockIsIdempotent(t *testing.T) {
	block := codexTerminalBlock("/synthetic path/gtmux")
	once, err := replaceCodexTerminalBlock("user settings\n", block)
	if err != nil {
		t.Fatal(err)
	}
	twice, err := replaceCodexTerminalBlock(once, block)
	if err != nil || twice != once || strings.Count(twice, codexTerminalBegin) != 1 {
		t.Fatal("managed block duplicated")
	}
}
