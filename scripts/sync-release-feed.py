#!/usr/bin/env python3
"""Generate public-key consumers and the independently hosted short bootstrap."""
import argparse
import json
import os
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
START = '# BEGIN GENERATED SIGNED RELEASE FEED\n'
END = '# END GENERATED SIGNED RELEASE FEED\n'

def generated():
    key = (ROOT / 'internal/releasefeed/public.pem').read_text().strip()
    library = (ROOT / 'scripts/release-feed-lib.sh').read_text().replace('@PUBLIC_KEY@', key)
    bootstrap = '''#!/usr/bin/env bash
# Download only an authenticated, version-pinned installer. No third-party proxy.
set -euo pipefail
command -v openssl >/dev/null 2>&1 || { echo 'gtmux: OpenSSL or LibreSSL is required' >&2; exit 1; }
TMP_DIR="$(mktemp -d -t gtmux-bootstrap.XXXXXX)"
trap 'command rm -rf "$TMP_DIR"' EXIT
'''+library+r'''
requested_version="${GTMUX_VERSION:-}"
if [ -n "$requested_version" ]; then
  printf '%s\n' "$requested_version" | LC_ALL=C grep -Eq '^v?(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})([-+][0-9A-Za-z.-]+)?$' || { echo 'gtmux: invalid version pin' >&2; exit 1; }
  requested_version="v${requested_version#v}"
fi
if _feed_fetch "$requested_version"; then :; else
  result=$?
  # Older/unmirrored pins use a verified current installer, preserving the
  # original GitHub artifact channel. Invalid signed metadata never falls back.
  if [ "$result" = 1 ] && [ -n "$requested_version" ]; then
    if _feed_fetch ""; then result=0; else result=$?; fi
  fi
  if [ "$result" != 0 ]; then
  if [ "$result" = 2 ]; then echo 'gtmux: invalid release signature or metadata; stopped' >&2; else echo 'gtmux: official downloads unavailable; retry later or use the documented GitHub installation' >&2; fi
  exit 1
  fi
fi
_feed_download install.sh "$TMP_DIR/install.sh" || { echo 'gtmux: verified installer download failed; stopped' >&2; exit 1; }
GTMUX_VERSION="${requested_version:-v$FEED_VERSION}" bash "$TMP_DIR/install.sh" "$@"
'''
    installer = (ROOT / 'install.sh').read_text()
    if START in installer:
        a=installer.index(START); b=installer.index(END,a)+len(END)
        installer=installer[:a]+START+library+END+installer[b:]
    else:
        installer=installer.replace('# ---- color (TTY only) ----',START+library+END+'\n# ---- color (TTY only) ----')
    return {ROOT/'bootstrap.sh':bootstrap, ROOT/'install.sh':installer,
            ROOT/'download-worker/src/public-key.mjs':'// Generated from internal/releasefeed/public.pem; contains no private key.\nexport const PUBLIC_KEY = '+json.dumps(key+'\n')+';\n'}

if __name__ == '__main__':
    parser=argparse.ArgumentParser(); parser.add_argument('--check',action='store_true');args=parser.parse_args()
    stale=[]
    for path, content in generated().items():
        if args.check:
            if not path.exists() or path.read_text()!=content or (path.name in ('install.sh','bootstrap.sh') and not path.stat().st_mode & 0o111): stale.append(str(path.relative_to(ROOT)))
        else:
            temporary = None
            try:
                with tempfile.NamedTemporaryFile(mode='w', encoding='utf-8', dir=path.parent, delete=False) as output:
                    temporary = Path(output.name)
                    output.write(content)
                os.chmod(temporary, 0o755 if path.name in ('install.sh', 'bootstrap.sh') else 0o644)
                os.replace(temporary, path)
            finally:
                if temporary and temporary.exists(): temporary.unlink()
    if stale: parser.error('Generated release files are stale: '+', '.join(stale))
