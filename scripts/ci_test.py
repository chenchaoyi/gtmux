"""Exercise scope boundaries, real git rename semantics and false-green guards."""
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import ci

ROOT = Path(__file__).resolve().parents[1]


class ScopeTests(unittest.TestCase):
    def test_docs_only_and_mixed_changes(self):
        self.assertEqual(ci.classify(["docs/phone.zh.md", "README.md"])["selected"], [])
        plan = ci.classify(["docs/phone.md", "macapp/Sources/GtmuxBar/MenuView.swift"])
        self.assertEqual(plan["selected"], ["menubar"])
        self.assertEqual(set(ci.classify(["macapp/Tests/GtmuxBarTests/MenuTests.swift",
                                         "internal/server/web/style.css"])["selected"]),
                         {"menubar", "web", "test"})

    def test_shared_or_unknown_impact_is_full(self):
        for path in ["internal/radar/agents.go", "api/contract.md", "go.mod", "go.sum",
                     ".github/workflows/ci.yml", "scripts/ci.py", "new-component/code.ts",
                     "mobileapp/package-lock.json", "macapp/Package.swift", "mobileapp/.ruby-version",
                     "mobileapp/ios/Podfile", "mobileapp/ios/GtmuxMobile.xcodeproj/project.pbxproj",
                     "mobileapp/src/api/client.ts", "tunnel-worker/src/index.ts",
                     "macapp/unclassified/foo.bin", "../docs/foo.md", "/docs/foo.md"]:
            with self.subTest(path=path):
                self.assertEqual(set(ci.classify([path])["selected"]), set(ci.JOBS))
        self.assertTrue(ci.classify([])["full"])

    def test_recognized_modules_include_consumers(self):
        self.assertEqual(ci.classify(["internal/radar/agents_test.go"])["selected"], ["test"])
        self.assertEqual(set(ci.classify(["mobileapp/src/components/Chat.tsx"])["selected"]),
                         {"mobile", "web", "ios-native", "ios-duo", "android-native", "android-emulator"})
        self.assertEqual(set(ci.classify(["mobileapp/ios/GtmuxWidget/Widget.swift"])["selected"]),
                         {"mobile", "ios-native", "ios-duo"})
        self.assertEqual(set(ci.classify(["relay-worker/src/index.ts"])["selected"]),
                         {"relay-worker", "test", "mobile", "android-native", "android-emulator"})

    def test_nul_parser_preserves_names_and_old_rename_path(self):
        names = ci.parse_diff(b"R100\0internal/radar/a.go\0docs/a.md\0D\0docs/deleted.md\0M\0docs/\xe4\xb8\xad\xe6\x96\x87\n$(oops).md\0")
        self.assertEqual(names[-1], "docs/中文\n$(oops).md")
        self.assertTrue(ci.classify(names)["full"])
        for value in [b"M\0foo", b"R100\0foo\0", b"U\0foo\0", b"M\0\0", b"M\0\xff\0"]:
            with self.subTest(value=value), self.assertRaises((ValueError, UnicodeError)):
                ci.parse_diff(value)

    def test_actual_git_rename_deletion_and_symlink(self):
        with tempfile.TemporaryDirectory(prefix="gtmux-ci-test-") as temp:
            def git(*args):
                return subprocess.check_output(["git", "-C", temp, *args], stderr=subprocess.DEVNULL).decode().strip()
            git("init"); git("config", "user.email", "ci@example.invalid"); git("config", "user.name", "CI")
            source = Path(temp, "internal/radar"); source.mkdir(parents=True)
            Path(source, "a.go").write_text("package radar\n")
            git("add", "."); git("commit", "-m", "base"); base = git("rev-parse", "HEAD")
            Path(temp, "docs").mkdir()
            Path(source, "a.go").rename(Path(temp, "docs/a.md"))
            git("add", "."); git("commit", "-m", "rename"); head = git("rev-parse", "HEAD")
            real_run = subprocess.run
            with patch("ci.subprocess.run", wraps=real_run) as run:
                # Resolve refs in the isolated repo without changing the process cwd.
                run.side_effect = lambda *args, **kwargs: real_run(*args, cwd=temp, **kwargs)
                paths = ci.changed_files(base, head, "pull_request")
                self.assertEqual(set(paths), {"internal/radar/a.go", "docs/a.md"})
                self.assertTrue(ci.classify(paths)["full"])
                Path(temp, "docs/a.md").unlink()
                git("add", "."); git("commit", "-m", "delete"); deleted = git("rev-parse", "HEAD")
                self.assertEqual(ci.changed_files(head, deleted, "push"), ["docs/a.md"])
                Path(temp, "docs/link.md").symlink_to("../../outside")
                git("add", "."); git("commit", "-m", "symlink")
                with self.assertRaisesRegex(ValueError, "Symbolic-link"):
                    ci.changed_files(deleted, git("rev-parse", "HEAD"), "push")

    def test_missing_refs_force_full_cli_outputs(self):
        with tempfile.TemporaryDirectory() as temp:
            output, summary = Path(temp, "outputs"), Path(temp, "summary")
            env = {**os.environ, "GITHUB_OUTPUT": str(output), "GITHUB_STEP_SUMMARY": str(summary)}
            result = subprocess.run(["python3", str(ROOT / "scripts/ci.py"), "plan", "--event", "pull_request",
                                     "--base", "missing", "--head", "0" * 40], env=env, capture_output=True, text=True, check=True)
            self.assertTrue(json.loads(result.stdout)["full"])
            for flag in ci.JOBS.values():
                self.assertIn(flag + "=true\n", output.read_text())
            self.assertIn("Cannot confirm diff", summary.read_text())

    def test_workflow_jobs_cannot_grow_outside_aggregation(self):
        text = (ROOT / ".github/workflows/ci.yml").read_text().split("jobs:\n", 1)[1]
        jobs = set(re.findall(r"^  ([\w-]+):$", text, re.M))
        self.assertEqual(jobs, set(ci.JOBS) | {"scope", "quality", "ci-required"})
        gate = text.split("  ci-required:\n", 1)[1]
        self.assertIn("if: always()", gate)
        for job in set(ci.JOBS) | {"scope", "quality"}:
            self.assertIn(job, gate.split("steps:", 1)[0])
        for job, flag in ci.JOBS.items():
            block = re.split(r"\n  \S", text.split("  " + job + ":\n", 1)[1], maxsplit=1)[0]
            self.assertIn("needs.scope.outputs." + flag + " == 'true'", block)

    def test_unsigned_ios_preserves_arm64_target_and_verifies_products(self):
        text = (ROOT / ".github/workflows/ci.yml").read_text()
        block = text.split("  ios-native:\n", 1)[1].split("  relay-worker:\n", 1)[0]
        self.assertIn("runs-on: macos-15\n", block)
        self.assertIn("DEVELOPER_DIR: /Applications/Xcode_26.3.app/Contents/Developer", block)
        self.assertIn('test "$(uname -m)" = arm64', block)
        duo = text.split("  ios-duo:\n", 1)[1].split("  relay-worker:\n", 1)[0]
        self.assertIn("runs-on: xcode-27", duo)
        self.assertIn("DEVELOPER_DIR: /Applications/Xcode_27.1.app/Contents/Developer", duo)
        self.assertIn("Probe iPhone Duo runtime", duo)
        self.assertIn("ARCHS=arm64 ONLY_ACTIVE_ARCH=NO CODE_SIGNING_ALLOWED=NO", duo)
        self.assertIn("Verify actual ARM64 app and extension products", block)
        self.assertIn("ARCHS=arm64 ONLY_ACTIVE_ARCH=NO CODE_SIGNING_ALLOWED=NO", block)
        self.assertNotIn("ARCHS=\"$(uname -m)\"", block)
        for target in ("GtmuxMobile", "usage_widget_targets_test.rb"):
            self.assertIn(target, block)

    def test_android_package_is_built_once_and_both_devices_use_its_output(self):
        text = (ROOT / ".github/workflows/ci.yml").read_text()
        build = text.split("  android-native:\n", 1)[1].split("  android-emulator:\n", 1)[0]
        emulators = text.split("  android-emulator:\n", 1)[1].split("  web:\n", 1)[0]
        self.assertEqual(text.count(":app:assembleQa"), 1)
        self.assertIn("fail-fast: false", emulators)
        for device in ("phone", "tablet"):
            self.assertIn("- device: " + device, emulators)
        self.assertIn("steps.package.outputs.name", build)
        self.assertIn("needs.android-native.outputs.package", emulators)
        self.assertIn("android_ci_package.py verify", emulators)
        self.assertNotIn("GTMUX_ANDROID_E2E_ONLY", emulators)
        self.assertIn("github.run_attempt", emulators)


class ResultTests(unittest.TestCase):
    def results(self, selected):
        return {name: {"result": "success" if name in selected or name in {"scope", "quality"} else "skipped"}
                for name in ("scope", "quality", *ci.JOBS)}

    def test_docs_can_skip_heavy_jobs_but_require_quality(self):
        plan = ci.classify(["README.md"]); needs = self.results([])
        self.assertEqual(ci.verdict(plan, needs), [])
        needs["quality"]["result"] = "skipped"
        self.assertTrue(ci.verdict(plan, needs))

    def test_missing_skipped_failed_cancelled_required_job_never_green(self):
        plan = ci.classify(["internal/radar/agents.go"])
        self.assertEqual(ci.verdict(plan, self.results(ci.JOBS)), [])
        for name in ("scope", "quality", *ci.JOBS):
            for result in (None, "skipped", "failure", "cancelled"):
                with self.subTest(name=name, result=result):
                    needs = self.results(ci.JOBS); needs[name] = {"result": result}
                    self.assertTrue(ci.verdict(plan, needs))

    def test_bad_plans_or_unselected_failure_are_rejected(self):
        for plan in (None, {}, {"selected": ["unknown"], "full": False},
                     {"selected": [], "full": True}, {"selected": [{}], "full": False},
                     {"selected": [], "full": "false"}, {"selected": ["test", "test"], "full": False}):
            self.assertTrue(ci.verdict(plan, self.results([])))
        needs = self.results([]); needs["web"]["result"] = "failure"
        self.assertTrue(ci.verdict(ci.classify(["README.md"]), needs))




class MacHostContractTests(unittest.TestCase):
    def test_mac_host_keeps_both_architectures_and_native_checks(self):
        root = Path(__file__).resolve().parents[1]
        workflow = (root / ".github/workflows/ci.yml").read_text()
        block = workflow.split("  menubar:\n", 1)[1].split("\n  ci-required:", 1)[0]
        self.assertIn("runs-on: macos-26", block)
        self.assertIn("DEVELOPER_DIR: /Applications/Xcode_26.6.app/Contents/Developer", block)
        self.assertIn('test "$(uname -m)" = arm64', block)
        self.assertIn("swift build -c release --arch arm64 --arch x86_64", block)
        self.assertIn("GTMUX_UNIVERSAL: '1'", block)
        self.assertIn("swift test --package-path shared/UsageWidgets", block)
        self.assertIn("swift test --parallel --num-workers 1", block)
        self.assertIn("bash macapp/Widget/build-extension.sh", block)
        self.assertIn("./scripts/check-design.sh", block)
        # The pinned standard iOS image remains ARM, never the Intel Mac host.
        ios = workflow.split("  ios-native:\n", 1)[1].split("\n  android-native:", 1)[0]
        self.assertIn("runs-on: macos-15\n", ios)

    def test_ios_delivery_pins_the_same_arm_toolchain_and_keeps_protection(self):
        root = Path(__file__).resolve().parents[1]
        text = (root / ".github/workflows/ios-test-delivery.yml").read_text()
        block = text.split("  delivery:\n", 1)[1]
        self.assertIn("runs-on: macos-15\n", block)
        self.assertIn("DEVELOPER_DIR: /Applications/Xcode_26.3.app/Contents/Developer", block)
        self.assertIn('test "$(uname -m)" = arm64', block)
        self.assertIn("environment: ios-test-delivery", block)
        self.assertIn("hosted-signing.rb", block)
        self.assertIn("CODE_SIGN_IDENTITY='Apple Distribution'", block)
        self.assertNotIn("continue-on-error:", block)


if __name__ == "__main__":
    unittest.main()
