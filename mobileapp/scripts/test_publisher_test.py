"""Portable publisher adapter tests: synthetic secrets, no network or real IPA."""
import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import stat
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("publisher", Path(__file__).with_name("test-publisher.py"))
publisher = importlib.util.module_from_spec(spec)
spec.loader.exec_module(publisher)


class PublisherTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.path = self.root / "config.json"
        self.data = {k: "synthetic-secret-" + k for k in publisher.FIELDS}
        self.data["origin"] = "https://builds.example.test"

    def save(self, data=None, mode=0o600):
        self.path.write_text(json.dumps(self.data if data is None else data))
        self.path.chmod(mode)

    def test_another_home_and_xdg_and_explicit_location(self):
        with patch.dict(os.environ, {"HOME": str(self.root)}, clear=True):
            self.assertEqual(publisher.config_path({}), self.root / ".config/ccy-builds/gtmux-publisher.json")
            self.assertEqual(publisher.config_path({"XDG_CONFIG_HOME": str(self.root / "xdg")}), self.root / "xdg/ccy-builds/gtmux-publisher.json")
            self.assertEqual(publisher.config_path({"BUILDS_PUBLISHER_CONFIG": str(self.path)}), self.path)

    def test_missing_configuration_is_actionable_without_values(self):
        with self.assertRaises(publisher.ConfigError) as error:
            publisher.settings(self.path, {})
        self.assertIn("Import an existing authorized configuration", str(error.exception))
        self.assertIn("BUILDS_UPLOAD_KEY", str(error.exception))

    def test_complete_environment_does_not_require_file(self):
        env = {k: self.data[k] for k in publisher.FIELDS}
        env["BUILDS_ORIGIN"] = self.data["origin"]
        resolved, origin = publisher.settings(self.path, env)
        self.assertEqual(resolved, env)
        self.assertEqual(origin, self.data["origin"])

    def test_environment_override_and_partial_file(self):
        self.save({"BUILDS_UPLOAD_KEY": "from-file", "origin": self.data["origin"]})
        env = {k: self.data[k] for k in publisher.FIELDS if k != "BUILDS_UPLOAD_KEY"}
        resolved, origin = publisher.settings(self.path, env)
        self.assertEqual(resolved["BUILDS_UPLOAD_KEY"], "from-file")
        self.assertEqual(origin, self.data["origin"])
        env["BUILDS_UPLOAD_KEY"] = "override"
        self.assertEqual(publisher.settings(self.path, env)[0]["BUILDS_UPLOAD_KEY"], "override")

    def test_unsafe_file_permissions_and_symlink_refused(self):
        self.save(mode=0o644)
        with self.assertRaises(publisher.ConfigError):
            publisher.read_config(self.path)
        self.path.chmod(0o600)
        link = self.root / "link.json"
        link.symlink_to(self.path)
        with self.assertRaises(publisher.ConfigError):
            publisher.read_config(link)

    def test_wrong_owner_and_nonregular_file_refused(self):
        self.save()
        with patch.object(publisher.os, "geteuid", return_value=os.geteuid() + 1):
            with self.assertRaises(publisher.ConfigError):
                publisher.read_config(self.path)
        fifo = self.root / "fifo"
        os.mkfifo(fifo, 0o600)
        with self.assertRaises(publisher.ConfigError):
            publisher.read_config(fifo)

    def test_malformed_json_does_not_echo_secret(self):
        self.path.write_text('{"secret": "never-echo-this')
        self.path.chmod(0o600)
        with self.assertRaises(publisher.ConfigError) as error:
            publisher.read_config(self.path)
        self.assertNotIn("never-echo-this", str(error.exception))

    def test_import_private_and_non_overwriting(self):
        publisher.import_config(self.path, io.BytesIO(json.dumps(self.data).encode()))
        self.assertEqual(stat.S_IMODE(self.path.stat().st_mode), 0o600)
        self.assertEqual(publisher.read_config(self.path), self.data)
        with self.assertRaises(publisher.ConfigError):
            publisher.import_config(self.path, io.BytesIO(json.dumps(self.data).encode()))
        self.assertEqual(publisher.read_config(self.path), self.data)

    def test_empty_partial_and_oversized_import_creates_nothing(self):
        for raw in (b"", b"{}", b"x" * (publisher.LIMIT + 1)):
            with self.subTest(length=len(raw)), self.assertRaises(publisher.ConfigError):
                publisher.import_config(self.path, io.BytesIO(raw))
            self.assertFalse(self.path.exists())

    def test_no_arbitrary_environment_fields_or_credential_urls(self):
        for data in ({**self.data, "PATH": "/evil"}, {**self.data, "origin": "https://user:secret@example.test"}, {**self.data, "origin": "https://example.test/?secret=private"}, {**self.data, "origin": "https://[invalid"}):
            with self.assertRaises(publisher.ConfigError) as error:
                publisher.validate(data)
            self.assertNotIn("private", str(error.exception))
            self.assertNotIn("synthetic-secret", str(error.exception))

    def invoke(self, command):
        out, err = io.StringIO(), io.StringIO()
        with patch.dict(os.environ, {}, clear=True), contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            code = publisher.main(["--config", str(self.path), *command])
        return code, out.getvalue() + err.getvalue()

    def test_check_reports_readiness_without_secret_or_network(self):
        self.save()
        code, text = self.invoke(["check"])
        self.assertEqual(code, 0)
        self.assertIn("remote access and signing have not been tested", text)
        self.assertNotIn("synthetic-secret", text)

    def test_missing_uploader_is_reported_before_execution(self):
        self.save()
        code, text = self.invoke(["check", "--uploader", str(self.root / "absent.py")])
        self.assertEqual(code, 1)
        self.assertIn("Uploader not found", text)

    def dummy_run(self, code=0, existing=False):
        self.save()
        uploader = self.root / "builds.py"
        uploader.write_text('import os,sys,json\nprint(json.dumps({"key":os.environ["BUILDS_UPLOAD_KEY"],"args":sys.argv[1:],"link":"https://example.test/private-bearer"}))\nprint("synthetic-private-log",file=sys.stderr)\nsys.exit(' + str(code) + ')\n')
        output, log = self.root / "result.json", self.root / "result.log"
        if existing:
            output.write_text("preserve-me")
        result = self.invoke(["run", "--uploader", str(uploader), "--output", str(output), "--log", str(log), "--", "link", "--build", "synthetic-id"])
        return result, output, log

    def test_secrets_stay_in_environment_and_private_files(self):
        (code, text), output, log = self.dummy_run()
        self.assertEqual(code, 0)
        self.assertNotIn("synthetic-secret", text)
        self.assertNotIn("private-bearer", text)
        self.assertNotIn("synthetic-private-log", text)
        value = json.loads(output.read_text())
        self.assertEqual(value["key"], self.data["BUILDS_UPLOAD_KEY"])
        self.assertNotIn(value["key"], value["args"])
        self.assertEqual(value["args"][:4], ["--server", self.data["origin"], "--app", "gtmux"])
        for p in (output, log):
            self.assertEqual(stat.S_IMODE(p.stat().st_mode), 0o600)

    def test_failed_uploader_preserves_receipt_results_and_exit_code(self):
        (code, text), output, log = self.dummy_run(code=7)
        self.assertEqual(code, 7)
        self.assertIn("preserve its IPA and receipt", text)
        self.assertTrue(output.exists())
        self.assertTrue(log.exists())
        self.assertNotIn("synthetic-secret", text)

    def test_existing_result_is_not_overwritten(self):
        (code, text), output, log = self.dummy_run(existing=True)
        self.assertEqual(code, 1)
        self.assertEqual(output.read_text(), "preserve-me")
        self.assertFalse(log.exists())


if __name__ == "__main__":
    unittest.main()
