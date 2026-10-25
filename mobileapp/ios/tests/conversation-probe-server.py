"""Bounded, synthetic loopback HTTP/UDP fixture for script-consent acceptance."""
import argparse
import json
import pathlib
import signal
import socket
import subprocess
import time

parser = argparse.ArgumentParser()
parser.add_argument('--output', required=True, type=pathlib.Path)
args = parser.parse_args()
root = args.output
root.mkdir(mode=0o700, parents=True, exist_ok=False)
listener = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
listener.bind(('127.0.0.1', 0))
listener.settimeout(1)
port = listener.getsockname()[1]
(root / 'udp-port.txt').write_text(str(port))
server = subprocess.Popen(['python3', str(pathlib.Path(__file__).with_name('conversation-fixture-server.py')), '--output', str(root), '--rtc-probe-port', str(port)])
packets = []
(root / 'packets.json').write_text('[]')

def stop(*_args):
    raise SystemExit()

signal.signal(signal.SIGTERM, stop)
signal.signal(signal.SIGINT, stop)
try:
    deadline = time.monotonic() + 180
    while time.monotonic() < deadline:
        try:
            data, _ = listener.recvfrom(2048)
            packets.append({'at': time.time(), 'bytes': len(data)})
            (root / 'packets.json').write_text(json.dumps(packets))
        except socket.timeout:
            pass
finally:
    listener.close()
    server.terminate()
    server.wait(timeout=5)
