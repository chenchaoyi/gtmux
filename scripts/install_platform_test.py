"""Execute the real installer with local fixtures; no network or real HOME writes."""
import hashlib
import io
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


class InstallPlatformTests(unittest.TestCase):
    def run_installer(self, system, machine, corrupt=False):
        with tempfile.TemporaryDirectory(prefix="gtmux-install-test-") as tmp:
            base = Path(tmp)
            tools, assets, home = (base / p for p in ("tools", "assets", "home"))
            for p in (tools, assets, home):
                p.mkdir()
            platform = "linux" if system == "Linux" else "darwin"
            arch = "arm64" if machine == "aarch64" else "amd64"
            name = f"gtmux-9.9.9-{platform}-{arch}.tar.gz"
            artifact = assets / name
            with tarfile.open(artifact, "w:gz") as archive:
                content = b"#!/bin/sh\necho gtmux-fixture\n"
                info = tarfile.TarInfo("gtmux")
                info.mode = 0o755
                info.size = len(content)
                archive.addfile(info, io.BytesIO(content))
            digest = "0" * 64 if corrupt else hashlib.sha256(artifact.read_bytes()).hexdigest()
            (assets / "SHASUMS256.txt").write_text(f"{digest}  {name}\n")
            (tools / "uname").write_text(f'#!/bin/sh\ncase "$1" in -s) echo {system};; -m) echo {machine};; esac\n')
            # The only fetch operation is a local file copy. Unknown URLs fail.
            python = shutil.which("python3")
            (tools / "curl").write_text(f'''#!{python}
import os, pathlib, shutil, sys
args=sys.argv[1:]
with open(os.environ['CALL_LOG'],'a') as log: log.write(args[-1]+'\\n')
dest=args[args.index('--output')+1]
source=pathlib.Path(os.environ['TEST_ASSETS'])/args[-1].rsplit('/',1)[-1]
if not source.is_file(): sys.exit(1)
shutil.copyfile(source,dest)
''')
            for p in tools.iterdir():
                p.chmod(0o755)
            log = base / "calls"
            env = {**os.environ, "HOME": str(home), "PATH": f"{tools}:{os.environ['PATH']}",
                   "GTMUX_VERSION": "v9.9.9", "GTMUX_INSTALL_MIRROR": "github",
                   "GTMUX_BIN_DIR": str(home / "bin"), "TEST_ASSETS": str(assets), "CALL_LOG": str(log)}
            env.pop("GTMUX_NO_APP", None)
            result = subprocess.run(["bash", str(ROOT / "install.sh")], env=env,
                                    capture_output=True, text=True, timeout=20)
            installed = home / "bin" / "gtmux"
            return result, installed.exists(), log.read_text() if log.exists() else "", list(home.iterdir())

    def test_linux_archives_install_without_any_mac_application(self):
        for machine in ("x86_64", "aarch64"):
            with self.subTest(machine=machine):
                result, installed, calls, _ = self.run_installer("Linux", machine)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertTrue(installed)
                self.assertIn("linux-", calls)
                self.assertNotIn("macos.zip", calls)
                self.assertNotIn("Menu bar", result.stdout)

    def test_checksum_failure_never_installs(self):
        result, installed, _, _ = self.run_installer("Linux", "x86_64", corrupt=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("sha256 mismatch", result.stderr)
        self.assertFalse(installed)

    def test_other_platform_is_refused_before_downloading(self):
        result, installed, calls, _ = self.run_installer("FreeBSD", "x86_64")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("unsupported operating system", result.stderr)
        self.assertEqual(calls, "")
        self.assertFalse(installed)
