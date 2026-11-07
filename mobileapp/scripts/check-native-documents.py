#!/usr/bin/env python3
"""Run production Foundation document and notification oracles on macOS; no app/signing/device."""
from pathlib import Path
import subprocess
import tempfile


def run(root, execute=subprocess.run):
    with tempfile.TemporaryDirectory(prefix='gtmux-document-checks-') as tmp:
        oracles = [(name, 'Documents/' + name + '.swift') for name in ('DocumentHTML', 'DocumentMarkdown')]
        oracles += [(name, 'GtmuxMobile/NotificationResponsePayload.swift') for name in
                    ('NotificationResponsePayload', 'NotificationResponseCompletion')]
        for name, source in oracles:
            output = str(Path(tmp) / name)
            execute(['xcrun', 'swiftc', '-Onone', '-module-cache-path', str(Path(tmp) / 'modules'), str(root / 'ios' / source),
                     str(root / 'ios/tests' / (name + 'Checks.swift')), '-o', output], check=True)
            execute([output], check=True)


if __name__ == '__main__':
    run(Path(__file__).resolve().parents[1])
