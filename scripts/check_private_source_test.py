import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import check_private_source as guard


class PrivateSourceTests(unittest.TestCase):
    def test_all_text_including_checker_and_fixtures_is_checked_without_echoing_values(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            for name in ["check.py", "fixture_test.go", "notes.md"]:
                (root / name).write_text("safe\nPrivateExampleName in context\n")
            output = io.StringIO()
            self.assertEqual(guard.check_paths(root, ["check.py", "fixture_test.go", "notes.md"],
                                              ["privateexamplename"], output), 3)
            self.assertIn('"check.py":2:', output.getvalue())
            self.assertIn('"fixture_test.go":2:', output.getvalue())
            self.assertNotIn("PrivateExampleName", output.getvalue())
            self.assertNotIn("privateexamplename", output.getvalue())
            self.assertNotIn("in context", output.getvalue())

    def test_private_file_configuration_preserves_case_insensitive_substring_policy(self):
        with tempfile.TemporaryDirectory() as folder:
            policy = Path(folder) / "policy.json"
            policy.write_text(json.dumps(["PrivateExampleName", "privateexamplename"]))
            self.assertEqual(guard.load_terms({"GTMUX_PRIVATE_TERMS_FILE": str(policy)}),
                             ["privateexamplename"])

    def test_missing_optional_policy_is_explicit_but_required_or_malformed_policy_fails(self):
        self.assertIsNone(guard.load_terms({}))
        for env in [{"GTMUX_PRIVATE_POLICY_REQUIRED": "true"},
                    {"GTMUX_PRIVATE_TERMS": "not-json"},
                    {"GTMUX_PRIVATE_TERMS": "[]"},
                    {"GTMUX_PRIVATE_TERMS": '["ok"]'},
                    {"GTMUX_PRIVATE_TERMS": '{"unexpected":true}'}]:
            with self.assertRaises(ValueError):
                guard.load_terms(env)

    def test_env_policy_wins_over_local_file_without_reading_that_file(self):
        self.assertEqual(guard.load_terms({"GTMUX_PRIVATE_TERMS": '["example-private"]',
                                           "GTMUX_PRIVATE_TERMS_FILE": "/missing/private-policy"}),
                         ["example-private"])

    def test_binary_and_external_symlink_are_not_read_as_text(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            (root / "asset").write_bytes(b"\0example-private")
            (root / "link").symlink_to("/missing/private-file")
            self.assertEqual(guard.check_paths(root, ["asset", "link"], ["example-private"], io.StringIO()), 0)

    def test_private_filenames_fail_without_repeating_the_name_in_diagnostics(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            (root / "example-private.png").write_bytes(b"\0binary")
            output = io.StringIO()
            self.assertEqual(guard.check_paths(root, ["example-private.png"], ["example-private"], output), 1)
            self.assertNotIn("example-private", output.getvalue())

    def test_unreadable_source_fails_and_exception_text_never_reaches_output(self):
        output = io.StringIO()
        with patch.dict(guard.os.environ, {"GTMUX_PRIVATE_TERMS": '["example-private"]'}, clear=True), \
                patch.object(guard.subprocess, "check_output", side_effect=OSError("private-value")), \
                patch.object(guard.sys, "stderr", output):
            self.assertEqual(guard.main(), 2)
        self.assertNotIn("private-value", output.getvalue())

    def test_wrapper_clears_private_environment_before_build_or_spec_helpers(self):
        wrapper = Path(__file__).with_name("check-design.sh").read_text()
        checked = wrapper.index("python3 scripts/check_private_source.py")
        cleared = wrapper.index("unset GTMUX_PRIVATE_TERMS ")
        self.assertLess(checked, cleared)
        self.assertLess(cleared, wrapper.index("CGO_ENABLED=0 go build"))
        self.assertLess(cleared, wrapper.index("python3 scripts/openspec_check.py"))


if __name__ == "__main__":
    unittest.main()
