import json
from pathlib import Path
import tempfile
import unittest

from android_ci_package import identity


class PackageTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.folder = Path(self.temp.name)
        self.apk = self.folder / "app-qa.apk"
        self.apk.write_bytes(b"synthetic QA package")
        self.source = "a" * 40

    def test_same_package_and_failed_only_rerun(self):
        result = identity(self.folder, self.source, 100, 1, True)
        self.assertEqual(identity(self.folder, self.source, 100, 2), result)
        with self.assertRaises(ValueError):
            identity(self.folder, self.source, 100, 1, True)

    def test_wrong_source_run_future_attempt_and_corruption(self):
        identity(self.folder, self.source, 100, 2, True)
        for source, run, attempt in [("b" * 40, 100, 2), (self.source, 101, 2), (self.source, 100, 1)]:
            with self.subTest(source=source, run=run, attempt=attempt), self.assertRaises(ValueError):
                identity(self.folder, source, run, attempt)
        self.apk.write_bytes(b"different QA package")
        with self.assertRaises(ValueError):
            identity(self.folder, self.source, 100, 2)

    def test_missing_malformed_and_symlink(self):
        for text in ["{}", "[]", '{"attempt":true}', "not json"]:
            (self.folder / "CI-PACKAGE.json").write_text(text)
            with self.subTest(text=text), self.assertRaises(ValueError):
                identity(self.folder, self.source, 100, 1)
        self.apk.unlink()
        self.apk.symlink_to(self.folder / "CI-PACKAGE.json")
        with self.assertRaises(ValueError):
            identity(self.folder, self.source, 100, 1)
