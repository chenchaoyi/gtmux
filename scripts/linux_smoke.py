"""Linux CI smoke: real tmux + foreground server; isolated HOME and socket only."""
import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request


def main():
    if not sys.platform.startswith("linux"):
        raise SystemExit("This acceptance script requires Linux")
    binary = str(Path(sys.argv[1]).resolve())
    tmux = shutil.which("tmux")
    if not tmux:
        raise SystemExit("tmux is required (do not skip Linux acceptance)")
    with tempfile.TemporaryDirectory(prefix="gtx-linux-", dir="/tmp") as tmp:
        root = Path(tmp)
        home = root / "home"
        home.mkdir()
        sock = root / "tmux.sock"
        env = {**os.environ, "HOME": str(home), "CODEX_HOME": str(home / ".codex"),
               "TMUX": f"{sock},0,0", "GTMUX_LANG": "en", "SHELL": "/bin/sh"}
        env.pop("GTMUX_VERSION", None)
        def cli(*args):
            return subprocess.run([binary, *args], env=env, check=True,
                                  capture_output=True, text=True, timeout=25).stdout
        def mux(*args):
            return subprocess.run([tmux, "-S", str(sock), "-f", "/dev/null", *args],
                                  env=env, check=True, capture_output=True, text=True, timeout=10).stdout
        mux("new-session", "-d", "-s", "linux-smoke", "cat")
        pane = mux("display-message", "-p", "-t", "linux-smoke", "#{pane_id}").strip()
        server = None
        try:
            with socket.socket() as listener:
                listener.bind(("127.0.0.1", 0))
                port = listener.getsockname()[1]
            log = (root / "serve.log").open("w")
            server = subprocess.Popen([binary, "serve", "--bind", "127.0.0.1", "--port", str(port),
                                       "--token", "isolated-ci-token"], env=env, stdout=log, stderr=log)
            def api(path, body=None, authorized=True):
                headers = {"Content-Type": "application/json"}
                if authorized:
                    headers["Authorization"] = "Bearer isolated-ci-token"
                data = json.dumps(body).encode() if body is not None else None
                with urllib.request.urlopen(urllib.request.Request(f"http://127.0.0.1:{port}{path}",
                                           headers=headers, data=data), timeout=12) as response:
                    return json.load(response)
            deadline = time.monotonic() + 15
            while True:
                try:
                    api("/api/health", authorized=False)
                    break
                except (OSError, urllib.error.URLError):
                    if time.monotonic() >= deadline or server.poll() is not None:
                        raise AssertionError("Linux serve did not become healthy")
                    time.sleep(0.1)
            info = api("/api/host")
            assert info["os"] == "Linux" and not info["capabilities"]["desktop_focus"], info
            try:
                api("/api/panes", authorized=False)
                raise AssertionError("Unauthenticated panes were exposed")
            except urllib.error.HTTPError as error:
                assert error.code == 401, error.code
            assert any(p["pane_id"] == pane for p in api("/api/panes"))
            api("/api/send", {"id": pane, "text": "linux-input-check", "enter": True})
            assert "linux-input-check" in mux("capture-pane", "-p", "-t", pane)
            created = api("/api/sessions", {"name": "linux-created", "request_id": "linux-smoke-request-1234"})
            assert created["session"] == "linux-created", created
            assert api("/api/sessions", {"name": "linux-created", "request_id": "linux-smoke-request-1234"}) == created
            try:
                api("/api/focus?id=" + pane.replace("%", "%25"), {})
                raise AssertionError("Headless focus claimed desktop success")
            except urllib.error.HTTPError as error:
                assert error.code == 501, error.code
            cli("install", "hooks", "--agent", "codex", "--yes")
            assert "gtmux" in (home / ".codex" / "hooks.json").read_text()
            report = json.loads(cli("doctor", "--json"))
            assert report, "doctor returned no structured report"
            resources = json.loads(cli("resource", "--json"))
            assert resources["machine"]["mem_tier"] in ("normal", "warn", "critical"), resources
            assert not (home / "Applications").exists(), "Linux setup created a macOS app directory"
            print("Linux smoke passed: authentication, host capabilities, real tmux discovery/input, idempotent remote creation, headless focus refusal, Codex hook setup, doctor, resources")
        finally:
            if server is not None:
                server.terminate()
                try:
                    server.wait(timeout=8)
                except subprocess.TimeoutExpired:
                    server.kill()
                    server.wait(timeout=5)
            subprocess.run([tmux, "-S", str(sock), "kill-server"], env=env,
                           capture_output=True, timeout=5)


if __name__ == "__main__":
    main()
