"""Synthetic trust, profile and recovery tests; no Apple/R2 credentials needed."""
import datetime
import importlib.util
import json
import os
import plistlib
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("delivery", Path(__file__).with_name("hosted-ios-delivery.py"))
delivery = importlib.util.module_from_spec(spec)
spec.loader.exec_module(delivery)


def der_item(tag, value):
    size = len(value)
    if size < 128:
        length = bytes([size])
    else:
        encoded = size.to_bytes((size.bit_length() + 7) // 8, "big")
        length = bytes([128 + len(encoded)]) + encoded
    return bytes([tag]) + length + value


def der_dictionary(fields):
    pairs = []
    for key, value in fields.items():
        encoded = der_item(0x01, b"\xff" if value else b"\x00") if isinstance(value, bool) else der_item(0x0C, value.encode())
        pairs.append(der_item(0x30, der_item(0x0C, key.encode()) + encoded))
    return der_item(0x70, b"\x02\x01\x01" + der_item(0xB0, b"".join(pairs)))


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

    def test_der_scalars_preserve_boolean_types_and_long_lengths(self):
        fields = {"application-identifier": "ABCDEFGHIJ.com.gtmux.app", "get-task-allow": False, "aps-environment": "production", "long": "x" * 300}
        self.assertEqual(delivery.der_entitlements(der_dictionary(fields)), fields)
        self.assertIs(delivery.der_entitlements(der_dictionary({"get-task-allow": True}))["get-task-allow"], True)
        self.assertEqual(delivery.der_entitlements(der_dictionary({"get-task-allow": "false"}))["get-task-allow"], "false")

    def test_malformed_or_unknown_der_fails_without_echoing_payload(self):
        valid = der_dictionary({"get-task-allow": False})
        pair = der_item(0x30, der_item(0x0C, b"get-task-allow") + der_item(0x01, b"\x00"))
        malformed = [b"", b"[Dict] synthetic-sensitive-payload", valid[:-1], valid + b"\x00",
                     der_item(0x70, b"\x02\x01\x01" + der_item(0xB0, pair + pair)),
                     valid.replace(b"\x01\x01\x00", b"\x01\x01\x01"),
                     valid.replace(b"\x02\x01\x01", b"\x02\x01\x02"),
                     b"\x70\x80" + valid[2:], b"\x70\x81" + valid[1:],
                     b"x" * (1024 * 1024 + 1)]
        for data in malformed:
            with self.subTest(data=data[:12]), self.assertRaises(delivery.DeliveryError) as error:
                delivery.der_entitlements(data)
            self.assertNotIn("synthetic-sensitive-payload", str(error.exception))

    def test_export_reads_der_and_rejects_debuggable_signed_extensions(self):
        for unsafe in (None, True, "false"):
            with self.subTest(unsafe=unsafe), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                for name in ("logs", "signing", "export"):
                    (root / name).mkdir()
                ipa = root / "export/app.ipa"
                ipa.write_bytes(b"synthetic IPA; extraction is mocked")
                app = root / "verify/Payload/Test.app"
                packages = [app, app / "PlugIns/Widget.appex", app / "PlugIns/Notification.appex"]
                for bundle, package in zip(delivery.BUNDLES, packages):
                    package.mkdir(parents=True)
                    (package / "Info.plist").write_bytes(plistlib.dumps({"CFBundleIdentifier": bundle, "CFBundleShortVersionString": "1.2.3", "CFBundleVersion": "4.1"}))

                def signed_output(command, stdout, stderr):
                    package = Path(command[-1]).parent if command[0] == "security" else Path(command[-1])
                    bundle = plistlib.loads((package / "Info.plist").read_bytes())["CFBundleIdentifier"]
                    if command[0] == "security":
                        profile = self.profile(bundle)
                        profile["ExpirationDate"] = datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(days=1)
                        stdout.write(plistlib.dumps(profile))
                    else:
                        fields = {"application-identifier": "ABCDEFGHIJ." + bundle, "get-task-allow": False, "aps-environment": "production"}
                        if unsafe is not None and bundle == "com.gtmux.app.widget":
                            fields["get-task-allow"] = unsafe
                        stdout.write(der_dictionary(fields) if "--der" in command else b"[Dict] human display")
                    return subprocess.CompletedProcess(command, 0)

                with patch.object(delivery, "run") as run, patch.object(delivery.subprocess, "run", side_effect=signed_output):
                    if unsafe is None:
                        self.assertEqual(delivery.verify_export(root, "1.2.3", "4.1", "ABCDEFGHIJ"), ipa)
                        strict = [call.args[0] for call in run.call_args_list if call.args[0][0] == "codesign"]
                        self.assertCountEqual(strict, [["codesign", "--verify", "--strict", str(package)] for package in packages])
                    else:
                        with self.assertRaisesRegex(delivery.DeliveryError, "signed entitlements"):
                            delivery.verify_export(root, "1.2.3", "4.1", "ABCDEFGHIJ")

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
