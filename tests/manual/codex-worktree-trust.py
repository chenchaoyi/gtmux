#!/usr/bin/env python3
"""Opt-in installed Codex trust contrast. Synthetic HOME, no prompt/model input.

Usage: python3 tests/manual/codex-worktree-trust.py --codex /absolute/codex --output /task/tmp/new-dir
The fixture exercises production forwarding/observer/adapter through a Go test
seam. It does not certify production PID admission, real panes or persisted trust.
"""
import argparse
import fcntl
import faulthandler
import hashlib
import json
import os
from pathlib import Path
import pty
import select
import signal
import struct
import subprocess
import termios
import time
import tomllib


def stop_process(process):
    if process.poll() is not None:
        return
    process.terminate()
    try:
        process.wait(timeout=2)
    except subprocess.TimeoutExpired:
        process.kill()
        process.wait(timeout=2)


def tui(binary, argv, env, cwd, output):
    pid, fd = pty.fork()
    if pid == 0:
        os.chdir(cwd)
        os.execve(str(binary), [str(binary), *argv], env)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack('HHHH', 40, 160, 0, 0))
    data = bytearray()
    start = time.monotonic()
    exited = False
    try:
        while time.monotonic() - start < 12 and len(data) < 512 * 1024:
            ready, _, _ = select.select([fd], [], [], .1)
            if ready:
                try:
                    chunk = os.read(fd, 65536)
                except OSError:
                    break
                if not chunk:
                    break
                data.extend(chunk)
                if b'Trust this folder?' in data:
                    break
            found, _ = os.waitpid(pid, os.WNOHANG)
            if found:
                exited = True
                break
    finally:
        output.write_bytes(data)
        output.with_suffix(".pid.json").write_text(json.dumps({"pid": pid, "fixture_child": True}))
        # Signal only the exact child, never an entire process group.
        # The fixture sends no text, confirmation key, or prompt.
        os.close(fd)
        if not exited:
            try:
                os.kill(pid, signal.SIGTERM)
                until = time.monotonic() + 2
                while time.monotonic() < until:
                    found, _ = os.waitpid(pid, os.WNOHANG)
                    if found:
                        exited = True
                        break
                    time.sleep(.05)
                if not exited:
                    os.kill(pid, signal.SIGKILL)
                    until = time.monotonic() + 2
                    while time.monotonic() < until:
                        found, _ = os.waitpid(pid, os.WNOHANG)
                        if found:
                            exited = True
                            break
                        time.sleep(.05)
                    if not exited:
                        raise RuntimeError('fixture child did not exit within cleanup budget')
            except (ProcessLookupError, ChildProcessError):
                pass
    output.write_bytes(data)
    return {'gate': b'Trust this folder?' in data, 'composer': b'Ask Codex to do anything' in data,
            'bytes': len(data), 'sha256': hashlib.sha256(data).hexdigest(), 'elapsed': time.monotonic() - start}


def main():
    os.umask(0o077)
    faulthandler.dump_traceback_later(90, repeat=False)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--codex', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    binary, root = args.codex.resolve(), args.output.resolve()
    if not binary.is_file():
        parser.error('installed Codex binary required')
    if len(str(root / 'backend.sock').encode()) >= 104:
        parser.error('use a shorter task temporary output path for macOS Unix sockets')
    root.mkdir(mode=0o700, parents=False, exist_ok=False)
    repo, worktree = root / 'main', root / 'worker'
    repo.mkdir()
    subprocess.run(['git', 'init', '-q', str(repo)], check=True)
    subprocess.run(['git', '-C', str(repo), '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid',
                    'commit', '--allow-empty', '-qm', 'Synthetic trust fixture'], check=True)
    subprocess.run(['git', '-C', str(repo), 'worktree', 'add', '-q', '-b', 'fixture', str(worktree)], check=True)
    source = Path(__file__).resolve().parents[2]
    goenv = os.environ.copy()
    goenv['GOMODCACHE'] = subprocess.check_output(['go', 'env', 'GOMODCACHE'], text=True).strip()
    goenv['GOPROXY'] = 'off'
    records = []
    for mode in ['transparent', 'adapted', 'untrusted', 'unregistered']:
        home, codex_home = root / (mode + '-home'), root / (mode + '-codex')
        home.mkdir(); codex_home.mkdir()
        config = ('model = "gpt-6.1-sol"\nsandbox_mode = "read-only"\ncli_auth_credentials_store = "file"\n'
                  'forced_login_method = "api"\n[analytics]\nenabled = false\n[feedback]\nenabled = false\n'
                  f'[projects.{json.dumps(str(repo))}]\ntrust_level = "trusted"\n')
        if mode == 'untrusted':
            config += f'[projects.{json.dumps(str(worktree))}]\ntrust_level = "untrusted"\n'
        config_path = codex_home / 'config.toml'
        config_path.write_text(config); config_path.chmod(0o600)
        (codex_home / 'auth.json').write_text(json.dumps({'auth_mode': 'apikey', 'OPENAI_API_KEY': 'synthetic-not-a-key'}))
        (codex_home / 'auth.json').chmod(0o600)
        before = tomllib.loads(config_path.read_text())
        if mode == 'unregistered':
            (worktree / '.git').write_text('gitdir: /synthetic/unregistered\n')
        env = {**goenv, 'HOME': str(home), 'CODEX_HOME': str(codex_home), 'OPENAI_API_KEY': 'synthetic-not-a-key',
               'OPENAI_BASE_URL': 'http://127.0.0.1:9/v1', 'TERM': 'xterm-256color', 'LANG': 'en_US.UTF-8',
               'GTMUX_WTR_FIXTURE': str(root), 'GTMUX_WTR_MODE': mode}
        backend_log = (root / (mode + '-backend.log')).open('wb')
        gateway_log = (root / (mode + '-gateway.log')).open('wb')
        backend = subprocess.Popen([str(binary), 'app-server', '--listen', 'unix://' + str(root / 'backend.sock')],
                                   cwd=worktree, env=env, stdin=subprocess.DEVNULL, stdout=backend_log, stderr=subprocess.STDOUT, start_new_session=True)
        gateway = None
        try:
            until = time.monotonic() + 5
            while not (root / 'backend.sock').exists() and backend.poll() is None and time.monotonic() < until:
                time.sleep(.05)
            if not (root / 'backend.sock').exists():
                raise RuntimeError('fixture backend unavailable')
            gateway = subprocess.Popen(['go', 'test', '-count=1', './internal/codexbridge', '-run', '^TestWTR003NativeTUIGateway$'],
                                       cwd=source, env=env, stdin=subprocess.DEVNULL, stdout=gateway_log, stderr=subprocess.STDOUT, start_new_session=True)
            until = time.monotonic() + 25
            while not (root / 'gateway.ready').exists() and gateway.poll() is None and time.monotonic() < until:
                time.sleep(.05)
            if not (root / 'gateway.ready').exists():
                raise RuntimeError('fixture forwarding gateway unavailable')
            result = tui(binary, ['--remote', 'unix://' + str(root / 'gateway.sock'), '--no-alt-screen', '-C', str(worktree)],
                         env, worktree, root / (mode + '-tui.log'))
            gateway.wait(timeout=4)
            after = tomllib.loads(config_path.read_text())
            tui_preferences = after.pop('tui', {})
            if set(tui_preferences) - {'screen_reader_detection_done', 'model_availability_nux'}:
                raise AssertionError('unexpected native preference mutation')
            observed = [json.loads(line) for line in (root / 'observed-config.jsonl').read_text().splitlines()]
            chosen = next((v for v in observed if v['changed']), observed[0])
            result.update(mode=mode, gateway_exit=gateway.returncode, trust_and_permissions_unchanged=before == after,
                          native_preference_keys=sorted(tui_preferences), forwarded_config_changed=any(v['changed'] for v in observed),
                          forwarded_cwd_trust=chosen['projects'].get(str(worktree), {}).get('trust_level'))
            records.append(result)
        finally:
            if gateway is not None:
                stop_process(gateway)
            stop_process(backend)
            backend_log.close(); gateway_log.close()
            for name in ['backend.sock', 'gateway.sock', 'gateway.ready', 'observed-config.jsonl']:
                (root / name).unlink(missing_ok=True)
    result = {'synthetic_only': True, 'codex': str(binary), 'source': str(source), 'records': records,
              'limits': ['forwarding/observer/adapter only; production PID admission not exercised', 'no real session or model input']}
    (root / 'RESULTS.json').write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(result))
    assert records[0]['gate'] and not records[1]['gate'] and records[1]['composer'] and records[3]['gate']
    assert records[1]['forwarded_config_changed'] and records[1]['forwarded_cwd_trust'] == 'trusted'
    assert not records[2]['forwarded_config_changed'] and records[2]['forwarded_cwd_trust'] == 'untrusted'
    assert not records[3]['forwarded_config_changed']
    assert all(r['trust_and_permissions_unchanged'] and r['gateway_exit'] == 0 for r in records)


if __name__ == '__main__':
    main()
