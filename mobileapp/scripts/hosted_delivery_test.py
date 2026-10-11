"""Synthetic trust, profile and recovery tests; no Apple/R2 credentials needed."""
import datetime
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("delivery", Path(__file__).with_name("hosted-ios-delivery.py"))
delivery = importlib.util.module_from_spec(spec)
spec.loader.exec_module(delivery)


class HostedDeliveryTests(unittest.TestCase):
    def setUp(self):
        self.now = datetime.datetime(2026, 1, 1, tzinfo=datetime.timezone.utc)
        self.commit = "a" * 40
        self.release = {"tag_name": "v1.2.3", "draft": False, "prerelease": False}
        self.runs = [{"head_sha": self.commit, "head_branch": "main", "event": "push", "run_number": 1, "run_attempt": 1, "status": "completed", "conclusion": "success"}]

    def profile(self, bundle="com.gtmux.app"):
        return {"UUID": "00000000-0000-0000-0000-000000000001", "TeamIdentifier": ["ABCDEFGHIJ"], "ExpirationDate": self.now + datetime.timedelta(days=1), "ProvisionedDevices": ["synthetic-device"], "Entitlements": {"application-identifier": "ABCDEFGHIJ." + bundle, "com.apple.developer.team-identifier": "ABCDEFGHIJ", "get-task-allow": False, "aps-environment": "production"}}

    def test_published_release_exact_main_ci(self):
        delivery.validate_release("v1.2.3", self.release, self.commit, self.runs)

    def test_ui_preview_needs_green_exact_commit_but_no_release(self):
        delivery.validate_ci(self.commit, self.runs)
        for commit in ("main", "v1.2.3", "shortsha", "a" * 39):
            with self.subTest(commit=commit), self.assertRaises(delivery.DeliveryError):
                delivery.validate_ci(commit, self.runs)

    def test_ui_preview_cannot_bypass_missing_ci(self):
        with self.assertRaises(delivery.DeliveryError):
            delivery.validate_ci(self.commit, [])

    def test_arbitrary_ref_and_unpublished_release_refused(self):
        for tag, release in [("main", self.release), ("v1.2.3; echo bad", self.release), ("v1.2.3", {**self.release, "draft": True}), ("v1.2.3", {**self.release, "prerelease": True}), ("v1.2.3", {**self.release, "tag_name": "v1.2.4"})]:
            with self.subTest(tag=tag, release=release), self.assertRaises(delivery.DeliveryError):
                delivery.validate_release(tag, release, self.commit, self.runs)

    def test_ci_from_other_commit_fork_or_pr_refused(self):
        for field, value in [("head_sha", "b" * 40), ("head_branch", "fix/branch"), ("event", "pull_request"), ("status", "in_progress"), ("conclusion", "failure")]:
            with self.subTest(field=field), self.assertRaises(delivery.DeliveryError):
                delivery.validate_release("v1.2.3", self.release, self.commit, [{**self.runs[0], field: value}])

    def test_latest_failed_rerun_cannot_reuse_old_pass(self):
        newer = {**self.runs[0], "run_attempt": 2, "conclusion": "failure"}
        with self.assertRaises(delivery.DeliveryError):
            delivery.validate_release("v1.2.3", self.release, self.commit, [*self.runs, newer])

    def test_all_three_ad_hoc_profiles_supported(self):
        for bundle in delivery.BUNDLES:
            devices, uuid = delivery.profile_check(self.profile(bundle), bundle, "ABCDEFGHIJ", self.now)
            self.assertEqual(devices, {"synthetic-device"})
            self.assertEqual(uuid, self.profile()["UUID"])

    def test_wrong_team_bundle_or_expired_profile_rejected(self):
        for change in [{"TeamIdentifier": ["WRONGTEAM1"]}, {"ExpirationDate": self.now}, {"ExpirationDate": None}, {"UUID": "invalid"}]:
            with self.subTest(change=change), self.assertRaises(delivery.DeliveryError):
                delivery.profile_check({**self.profile(), **change}, "com.gtmux.app", "ABCDEFGHIJ", self.now)
        for key, value in [("application-identifier", "ABCDEFGHIJ.other"), ("com.apple.developer.team-identifier", "WRONGTEAM1"), ("get-task-allow", True), ("aps-environment", "development")]:
            profile = self.profile()
            profile["Entitlements"][key] = value
            with self.subTest(key=key), self.assertRaises(delivery.DeliveryError):
                delivery.profile_check(profile, "com.gtmux.app", "ABCDEFGHIJ", self.now)

    def test_store_enterprise_or_empty_device_profiles_rejected(self):
        for changes in [{"ProvisionedDevices": []}, {"ProvisionedDevices": None}, {"ProvisionsAllDevices": True}, {"ProvisionedDevices": [None]}]:
            with self.subTest(changes=changes), self.assertRaises(delivery.DeliveryError):
                delivery.profile_check({**self.profile(), **changes}, "com.gtmux.app", "ABCDEFGHIJ", self.now)

    def test_preflight_missing_fields_has_names_not_values(self):
        with tempfile.TemporaryDirectory() as tmp:
            result = subprocess.run(["python3", str(Path(delivery.__file__)), "preflight", "--root", tmp], capture_output=True, text=True, env={"PATH": os.environ["PATH"], "IOS_CERT_PASSWORD": "synthetic-sensitive-password"})
            self.assertEqual(result.returncode, 1)
            self.assertIn("IOS_CERT_P12", result.stderr)
            self.assertNotIn("synthetic-sensitive-password", result.stdout + result.stderr)

    def test_preflight_requires_dedicated_uploader_key(self):
        with tempfile.TemporaryDirectory() as tmp:
            env = {name: "synthetic" for name in delivery.SIGNING_FIELDS}
            env.update(IOS_TEAM_ID="ABCDEFGHIJ", IOS_RESULT_RECIPIENT="age1" + "a" * 58,
                       IOS_PUBLISHER_READ_TOKEN="synthetic-personal-token")
            with self.assertRaisesRegex(delivery.DeliveryError, "IOS_PUBLISHER_SSH_KEY"):
                delivery.preflight(Path(tmp), env)
            env["IOS_PUBLISHER_SSH_KEY"] = "synthetic-dedicated-key"
            with patch.object(delivery, "run") as run:
                delivery.preflight(Path(tmp), env)
            run.assert_called_once()

    def test_recovery_allowlist_excludes_credentials_profiles_and_symlinks(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            names = ["logs/archive.log", "export/app.ipa", "receipt.json", "publication.json", "publication.log", "signing/keychain-password", "signing/certificate.p12", "signing/profiles.json", "signing/profile.mobileprovision", "export/profile.mobileprovision", "source/secret.py", "recovery.tar.gz"]
            for name in names:
                path = root / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("synthetic")
            (root / "logs/link.log").symlink_to(root / "signing/keychain-password")
            self.assertEqual({str(p.relative_to(root)) for p in delivery.recovery_files(root)}, {"logs/archive.log", "export/app.ipa", "receipt.json", "publication.json", "publication.log"})

    def test_failed_child_output_never_echoed(self):
        with tempfile.TemporaryDirectory() as tmp:
            log = Path(tmp) / "log"
            with self.assertRaises(delivery.DeliveryError):
                delivery.run(["python3", "-c", "import sys;print('synthetic-private-link');sys.exit(7)"], log)
            self.assertIn("synthetic-private-link", log.read_text())

    def test_sealing_uses_recipient_and_excludes_secret_files(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "logs").mkdir()
            (root / "publication.json").write_text(json.dumps({"installation": "synthetic-private-url"}))
            (root / "signing").mkdir()
            (root / "signing/secret").write_text("secret")
            output = root / "result.age"
            def encrypt(command, log):
                import tarfile
                with tarfile.open(command[-1]) as archive:
                    self.assertEqual(archive.getnames(), ["publication.json"])
                self.assertEqual(command[:3], ["age", "-r", "public-recipient"])
                output.write_text("synthetic-ciphertext")
            with patch.object(delivery, "run", side_effect=encrypt):
                delivery.seal(root, output, "public-recipient")
            self.assertTrue(output.exists())
            self.assertFalse((root / "recovery.tar.gz").exists())


if __name__ == "__main__":
    unittest.main()
