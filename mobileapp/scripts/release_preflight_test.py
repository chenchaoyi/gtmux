import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

SCRIPTS = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("preflight", SCRIPTS / "release-preflight.py")
preflight = importlib.util.module_from_spec(spec)
spec.loader.exec_module(preflight)


class PreparationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.mobile = self.root / "mobileapp"
        for script in ("set-version.sh", "gen-release-notes.sh", "appsrc-hash.sh"):
            self.put("scripts/" + script, (SCRIPTS / script).read_text())
        self.put("src/example.ts", "synthetic source")
        self.put("ios/GtmuxMobile.xcodeproj/project.pbxproj", "MARKETING_VERSION = 1.0.1;\nMARKETING_VERSION = 1.0.1;\n")
        self.put("release-notes/1.0.1.en.txt", "old note")
        self.put("release-notes/1.0.1.zh.txt", "旧说明")
        self.put("fastlane/metadata/en-US/release_notes.txt", "New synthetic note\n")
        self.put("fastlane/metadata/zh-Hans/release_notes.txt", "新的合成说明\n")
        self.git("init", "-q")
        self.git("config", "user.name", "Synthetic")
        self.git("config", "user.email", "gtmux@ccy.dev")
        self.git("add", "."); self.git("commit", "-qm", "synthetic base")
        self.git("tag", "v1.0.1"); self.git("tag", "android/v9.9.9")

    def git(self, *args):
        return subprocess.check_output(["git", "-C", str(self.root), "-c", "user.name=Synthetic", "-c", "user.email=test@example.invalid", *args], stderr=subprocess.DEVNULL).decode().strip()

    def put(self, path, content):
        target = self.mobile / path; target.parent.mkdir(parents=True, exist_ok=True); target.write_text(content)

    def stamp(self, *args):
        return subprocess.run(["bash", str(self.mobile / "scripts/set-version.sh"), *args], capture_output=True, text=True)

    def test_explicit_version_without_tag_and_independent_android_notes(self):
        tags = self.git("tag")
        result = self.stamp("--version", "1.0.2")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.strip(), "1.0.2")
        self.assertEqual(tags, self.git("tag"))
        self.assertEqual(preflight.check(self.root, "1.0.2", ["ios"])["version_and_notes"], "ready")
        self.put("release-notes/android.json", json.dumps([{"version": "1.0.1", "en": ["Old Android"], "zh": ["旧安卓"]}]))
        with self.assertRaisesRegex(ValueError, "Android"):
            preflight.check(self.root, "1.0.2", ["ios", "android"])
        self.put("release-notes/android.json", json.dumps([{"version": "1.0.2", "en": ["Android specific"], "zh": ["安卓独立说明"]}]))
        self.assertEqual(preflight.check(self.root, "1.0.2", ["ios", "android"])["platforms"], ["android", "ios"])

    def test_legacy_tag_ignores_android_namespace(self):
        result = self.stamp()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.strip(), "1.0.1")

    def test_public_commit_identity_guard_redacts_rejected_addresses(self):
        self.assertEqual(preflight.identity(self.root)["configured_addresses_printed"], False)
        for variable in ("GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"):
            with self.subTest(variable=variable), patch.dict(os.environ, {variable: "synthetic-private@example.invalid"}):
                with self.assertRaises(ValueError) as error:
                    preflight.identity(self.root)
                self.assertNotIn("synthetic-private", str(error.exception))
        with patch.dict(os.environ, {"GIT_AUTHOR_EMAIL": "123+synthetic@users.noreply.github.com",
                                     "GIT_COMMITTER_EMAIL": "123+synthetic@users.noreply.github.com"}):
            self.assertTrue(preflight.identity(self.root))
        with patch.dict(os.environ, {"GIT_AUTHOR_EMAIL": "123+synthetic[bot]@users.noreply.github.com",
                                     "GIT_COMMITTER_EMAIL": "123+synthetic[bot]@users.noreply.github.com"}):
            self.assertTrue(preflight.identity(self.root))

    def test_invalid_arguments_do_not_write(self):
        before = self.git("status", "--porcelain")
        for args in [("--version", "1.02.3"), ("--version", "1.2.3-rc1"), ("--unknown", "1.2.3"), ("--version", "1.2.3\n")]:
            self.assertNotEqual(self.stamp(*args).returncode, 0)
            self.assertEqual(before, self.git("status", "--porcelain"))

    def test_preflight_detects_drift_without_writing(self):
        self.assertEqual(self.stamp("--version", "1.0.2").returncode, 0)
        for path, body in [("src/version.ts", "wrong version"),
                           ("ios/GtmuxMobile.xcodeproj/project.pbxproj", "MARKETING_VERSION = 0.0.1;"),
                           ("src/releaseNotes.ts", "stale generated history"),
                           ("src/example.ts", "changed after notes")]:
            target = self.mobile / path; before = target.read_text(); target.write_text(body)
            status = self.git("diff")
            with self.subTest(path=path), self.assertRaises(ValueError):
                preflight.check(self.root, "1.0.2", ["ios"])
            self.assertEqual(status, self.git("diff"))
            target.write_text(before)
        self.assertEqual(preflight.check(self.root, "1.0.2", ["none"])["mobile_build"], "not selected")
