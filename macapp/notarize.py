#!/usr/bin/env python3
"""Submit once; retry only an observed transient ticket lookup, then validate."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import time


def notarize(archive, bundle, env, run=subprocess.run, pause=time.sleep):
    args = ['xcrun', 'notarytool', 'submit', str(archive)]
    if env.get('GTMUX_NOTARY_PROFILE'):
        args += ['--keychain-profile', env['GTMUX_NOTARY_PROFILE']]
    elif all(env.get(k) for k in ('GTMUX_NOTARY_KEY', 'GTMUX_NOTARY_KEY_ID', 'GTMUX_NOTARY_ISSUER')):
        args += ['--key', env['GTMUX_NOTARY_KEY'], '--key-id', env['GTMUX_NOTARY_KEY_ID'], '--issuer', env['GTMUX_NOTARY_ISSUER']]
    else:
        raise ValueError('Notarization credentials are incomplete')
    result = run(args + ['--wait', '--output-format', 'json'], capture_output=True, text=True)
    if result.returncode != 0:
        raise ValueError('Notarization submission failed; inspect Apple status before another submission')
    try:
        status = json.loads(result.stdout)['status']
    except (KeyError, ValueError, TypeError):
        raise ValueError('Notarization did not return a complete status') from None
    if status != 'Accepted':
        raise ValueError('Apple did not accept this package; publication stopped')
    print('Notarization accepted; retrieving the ticket.', flush=True)
    for attempt in range(3):
        result = run(['xcrun', 'stapler', 'staple', str(bundle)], capture_output=True, text=True, timeout=90)
        if result.returncode == 0:
            break
        # Exit 68 alone also covers permanent failures. Only the observed Apple
        # CloudKit transport loss qualifies; no rejection/signature is retried.
        lost_connection = 'NSURLErrorDomain Code=-1005' in (result.stdout + result.stderr)
        if result.returncode != 68 or not lost_connection or attempt == 2:
            raise ValueError('Could not staple the accepted ticket; publication stopped')
        print('Apple ticket connection was lost; retrying the same package lookup.', flush=True)
        pause((2, 5)[attempt])
    result = run(['xcrun', 'stapler', 'validate', str(bundle)], capture_output=True, text=True, timeout=90)
    if result.returncode != 0:
        raise ValueError('Stapled ticket validation failed; publication stopped')
    print('Ticket stapled and independently validated.', flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--archive', type=Path, required=True)
    parser.add_argument('--bundle', type=Path, required=True)
    args = parser.parse_args()
    if not args.archive.is_file() or not args.bundle.is_dir():
        raise SystemExit('The signed package or notarization archive is missing')
    try:
        notarize(args.archive, args.bundle, os.environ)
    except (ValueError, OSError, subprocess.SubprocessError) as error:
        # Do not print subprocess argv or credential-bearing environment values.
        message = str(error) if isinstance(error, ValueError) else type(error).__name__
        raise SystemExit('Notarization stopped: ' + message) from None


if __name__ == '__main__':
    main()
