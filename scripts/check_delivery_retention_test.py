import contextlib
import copy
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from check_delivery_retention import main, measure, validate_policy


def policy():
    return {"schema": "gtmux.delivery-retention.v1", "purpose": "Unique source recovery and concise acceptance", "source": {"revision": "abcdef1234567", "channel": "engineering"}, "budget_bytes": 4096, "retain_until": "2026-12-01", "review_by": "2026-11-15", "review_owner": "maintainer", "auto_delete": False}


class DeliveryRetentionTests(unittest.TestCase):
    def test_valid_scope_and_no_payload_read_or_deletion(self):
        with tempfile.TemporaryDirectory() as tmp, tempfile.TemporaryDirectory() as outside:
            root = Path(tmp)
            (Path(outside) / "large").write_bytes(b"x" * 10000)
            (root / "external").symlink_to(outside, target_is_directory=True)
            manifest = root / "retention.json"
            manifest.write_text(json.dumps(policy()))
            with contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(main([str(manifest)]), 0)
            self.assertEqual(measure(root), manifest.stat().st_size)
            self.assertTrue((Path(outside) / "large").exists())

    def test_unknown_private_metadata_and_auto_delete_refused(self):
        for change in ({"private_url": "synthetic-only"}, {"auto_delete": True}, {"budget_bytes": True}, {"review_owner": "not-an-email@example.invalid"}, {"review_by": "2027-01-01"}):
            value = copy.deepcopy(policy())
            value.update(change)
            with self.assertRaises(ValueError):
                validate_policy(value)

    def test_budget_failure_does_not_clean_data(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            value = policy()
            value["budget_bytes"] = 1
            manifest = root / "retention.json"
            manifest.write_text(json.dumps(value))
            with contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(main([str(manifest)]), 1)
            self.assertTrue(manifest.exists())

    def test_project_archive_destination(self):
        with tempfile.TemporaryDirectory() as tmp:
            home = Path(tmp)
            project = home / "project"
            config = home / ".config/gtmux"
            state = home / ".local/share/gtmux"
            outside = home / "outside"
            for directory in (project, config, state, outside):
                directory.mkdir(parents=True)
            (project / "data-alias").symlink_to(state, target_is_directory=True)
            (project / "config-alias").symlink_to(config, target_is_directory=True)
            (config / "project-alias").symlink_to(project, target_is_directory=True)
            destination = project / "releases/v1.0.186"

            def check_destination(path, root=project):
                return main(["--project-root", str(root), "--destination", str(path)])

            with patch.dict(os.environ, {"HOME": str(home)}), contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(check_destination(destination), 0)
                self.assertFalse(destination.exists())
                self.assertFalse(destination.parent.exists())
                for path in (project, state / "old", config / "notes", outside / "store",
                             project / "../outside/release", project / "data-alias/release",
                             project / "config-alias/store", config / "project-alias/store"):
                    with self.subTest(path=path):
                        self.assertEqual(check_destination(path), 1)
                for root in (state, home / "missing-root"):
                    with self.subTest(root=root):
                        self.assertEqual(check_destination(root / "releases/v1.0.186", root), 1)

                destination.mkdir(parents=True)
                manifest = destination / "retention.json"
                manifest.write_text(json.dumps(policy()))
                self.assertEqual(main(["--project-root", str(project), str(manifest)]), 0)
                (destination / "payload").write_bytes(b"a" * 4096)
                self.assertEqual(main(["--project-root", str(project), str(manifest)]), 1)
                self.assertTrue((destination / "payload").is_file())
                legacy = state / "retention.json"
                legacy.write_text(json.dumps(policy()))
                self.assertEqual(main([str(legacy)]), 0)
                self.assertEqual(main(["--project-root", str(home), str(legacy)]), 1)
