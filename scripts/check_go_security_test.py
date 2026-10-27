import io
from pathlib import Path
import unittest
from unittest.mock import patch

import check_go_security as guard

ROOT = Path(__file__).resolve().parents[1]


class GoSecurityTests(unittest.TestCase):
    def test_only_patched_stable_toolchains_build(self):
        for version in ("go1.26.10", "go1.26.11", "go1.27.3", "go1.28.0", "go2.0.0"):
            self.assertTrue(guard.patched(version), version)
        for version in ("go1.25.99", "go1.26.9", "go1.27.1", "go1.27.2",
                        "go1.27.3rc1", "devel go1.28", "go1.27", "", "go1.27.3 evil"):
            self.assertFalse(guard.patched(version), version)

    def test_probe_failure_is_not_success(self):
        with patch.object(guard.subprocess, "check_output", side_effect=OSError()), \
                patch.object(guard.sys, "stderr", io.StringIO()):
            self.assertEqual(guard.main(), 1)

    def test_official_cli_and_bundled_cli_builds_use_the_gate(self):
        self.assertIn("python3 scripts/check_go_security.py", (ROOT / ".goreleaser.yaml").read_text())
        script = (ROOT / "macapp/build.sh").read_text()
        self.assertLess(script.index("scripts/check_go_security.py"), script.index("go build -ldflags"))
