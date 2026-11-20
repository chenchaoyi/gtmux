import os
from pathlib import Path
import sys
import tarfile
import tempfile
import unittest
from unittest.mock import patch

import capture_store_assets_test as capture_suite
import store_artifacts as store
import store_artifacts_test as store_suite


class DeliveryWriteBoundaryTests(unittest.TestCase):
    def setUp(self):
        self.root = Path(self.enterContext(tempfile.TemporaryDirectory()))
        home = self.root / "home"
        self.enterContext(patch.dict(os.environ, {"HOME": str(home)}))
        self.managed_data = home / ".local/share/gtmux"
        self.managed_config = home / ".config/gtmux"
        for managed in (self.managed_data, self.managed_config):
            managed.mkdir(parents=True)
        self.output = self.root / "project-output"
        self.output.mkdir()
        self.capture = capture_suite.capture

    def fixture(self, root, name):
        path = root / name
        path.write_bytes(b"existing archive or record")
        return path

    def pack(self, output=None):
        dest = output if output is not None else self.output
        app = self.root / "gtmux.app"
        if not app.exists():
            app = store_suite.ArtifactTests().app(self.root)
        with patch.object(sys, "argv", [
            "store_artifacts.py", "pack-build", "--root", str(dest), "--app", str(app),
            "--tag", "v1.2.3", "--commit", "a" * 40, "--xcode", "example", "--arch", "arm64",
        ]):
            store.main()

    def test_pack_refuses_linked_leaf_and_preserves_existing_bytes(self):
        target = self.fixture(self.managed_data, "existing")
        archive = self.output / "simulator-app.tar.gz"
        archive.symlink_to(target)
        with self.assertRaises((ValueError, OSError)):
            self.pack()
        self.assertTrue(archive.is_symlink())
        self.assertEqual(target.read_bytes(), b"existing archive or record")
        self.assertFalse((self.output / "build.json").exists())

    def test_archive_through_external_root_alias_and_failed_replacement(self):
        alias = self.root / "external-alias"
        alias.symlink_to(self.output, target_is_directory=True)
        self.pack(alias)
        archive = self.output / "simulator-app.tar.gz"
        with tarfile.open(archive) as packed:
            self.assertIn("gtmux.app/Info.plist", packed.getnames())
        previous = archive.read_bytes()
        with patch.object(tarfile.TarFile, "add", side_effect=OSError("synthetic write failure")):
            with self.assertRaises(OSError):
                self.pack()
        self.assertEqual(archive.read_bytes(), previous)
        self.assertEqual({p.name for p in self.output.iterdir()}, {"simulator-app.tar.gz", "build.json"})

    def test_checkpoint_refuses_linked_subdirectory(self):
        key = "phone-en"
        raw = store_suite.ArtifactTests().group(self.root, key)
        (self.output / "raw").mkdir()
        (self.output / "raw" / key).symlink_to(self.managed_config, target_is_directory=True)
        marker = self.fixture(self.managed_config, "00-demo-entry.png")
        with self.assertRaises((ValueError, OSError)):
            store.checkpoint_group(self.output, {}, key, raw)
        self.assertEqual(marker.read_bytes(), b"existing archive or record")
        self.assertFalse((self.output / "checkpoint.json").exists())
        self.assertEqual(list(self.managed_config.iterdir()), [marker])

    def test_progress_log_refuses_link_and_appends_to_regular_file(self):
        marker = self.fixture(self.managed_config, "progress")
        log = self.output / "progress.jsonl"
        log.symlink_to(marker)
        with self.assertRaises((ValueError, OSError)):
            self.capture.progress(self.output, "fixture")
        self.assertTrue(log.is_symlink())
        self.assertEqual(marker.read_bytes(), b"existing archive or record")

        external = self.root / "plain-capture"
        external.mkdir()
        self.capture.progress(external, "first")
        self.capture.progress(external, "second")
        lines = (external / "progress.jsonl").read_text().splitlines()
        self.assertEqual(len(lines), 2)
        self.assertIn('"first"', lines[0])
        self.assertIn('"second"', lines[1])

    def test_wda_log_refuses_link_before_compiler(self):
        appium = self.root / "appium"
        entry, _ = capture_suite.CaptureTests().wda(appium)
        marker = self.fixture(self.managed_data, "wda-log")
        log = self.output / "wda-build-phone.log"
        log.symlink_to(marker)
        with patch.object(self.capture.subprocess, "run") as compiler:
            with self.assertRaises((ValueError, OSError)):
                self.capture.prebuild_wda(appium, "fixture-device", self.output, "phone", "26.5",
                                          lambda args: str(entry))
            compiler.assert_not_called()
        self.assertTrue(log.is_symlink())
        self.assertEqual(marker.read_bytes(), b"existing archive or record")
