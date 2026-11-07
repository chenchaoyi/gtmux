"""Exercise scope boundaries, real git rename semantics and false-green guards."""
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import sys
import textwrap
import unittest
from unittest.mock import patch

import ci

ROOT = Path(__file__).resolve().parents[1]


class ScopeTests(unittest.TestCase):
    def test_unit_test_scope_requires_immutable_product_exclusion(self):
        path = "mobileapp/src/screens/HQ.test.tsx"
        self.assertIn("android-emulator", ci.classify([path])["selected"])
        self.assertEqual(ci.classify([path], unit_test_isolation=True)["selected"], ["mobile", "web"])
        for extra in ["mobileapp/e2e/fake-serve/server.ts", "mobileapp/src/ui/Composer.tsx"]:
            self.assertIn("android-emulator", ci.classify([path, extra], unit_test_isolation=True)["selected"])
        for extra in ["tests/contracts/http-contract.json", "mobileapp/metro.config.js", "mobileapp/package-lock.json"]:
            self.assertTrue(ci.classify([path, extra], unit_test_isolation=True)["full"])
        self.assertFalse(ci.confirmed_unit_test_isolation("bad-ref"))
        with patch("ci.subprocess.run", side_effect=OSError("unavailable")):
            self.assertFalse(ci.confirmed_unit_test_isolation("a" * 40))
        with patch("ci.subprocess.run") as read:
            read.return_value.stdout = (ROOT / "mobileapp/metro.config.js").read_bytes()
            self.assertTrue(ci.confirmed_unit_test_isolation("a" * 40))
            self.assertEqual(read.call_args.args[0], ["git", "show", "a" * 40 + ":mobileapp/metro.config.js"])
            read.return_value.stdout = b"resolver proof was changed"
            self.assertFalse(ci.confirmed_unit_test_isolation("a" * 40))

    def test_docs_only_and_mixed_changes(self):
        self.assertEqual(ci.classify(["docs/phone.zh.md", "README.md"])["selected"], [])
        plan = ci.classify(["docs/phone.md", "macapp/Sources/GtmuxBar/MenuView.swift"])
        self.assertEqual(plan["selected"], ["menubar"])
        self.assertEqual(set(ci.classify(["macapp/Tests/GtmuxBarTests/MenuTests.swift",
                                         "internal/server/web/style.css"])["selected"]),
                         {"menubar", "web", "test"})

    def test_reviewed_test_records_are_policy_only_without_hiding_test_changes(self):
        records = ["docs/test/catalogue.json", "docs/test/implementations.json", "docs/test/CASES.csv",
                   "docs/test/cases/L1.csv", "docs/test/cases/M.csv", "docs/test/designs/example-20261105.json"]
        self.assertEqual(ci.classify(records)["selected"], [])
        self.assertEqual(ci.classify(records + ["internal/radar/agents_test.go"])["selected"], ["test"])
        for path in ["docs/test/designs/script.py", "docs/test/catalogue.js", "docs/test/cases/unknown.csv"]:
            self.assertTrue(ci.classify([path])["full"])

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
                         {"mobile", "web", "ios-native", "ios-scene", "ios-scene-baseline-26", "android-native", "android-emulator"})
        self.assertEqual(set(ci.classify(["mobileapp/ios/GtmuxWidget/Widget.swift"])["selected"]),
                         {"mobile", "ios-native", "ios-scene", "ios-scene-baseline-26"})
        self.assertEqual(set(ci.classify(["relay-worker/src/index.ts"])["selected"]),
                         {"relay-worker", "test", "mobile", "android-native", "android-emulator"})

    def test_scene_migration_selection_is_not_branch_dependent_and_keeps_old_gates(self):
        for path in ('mobileapp/App.tsx', 'mobileapp/src/ui/windowMetrics.tsx', 'mobileapp/src/push/index.ts',
                     'mobileapp/src/usage/UsageWidgetBridge.tsx', 'mobileapp/e2e/setup/sceneJourneys.ts',
                     'mobileapp/ios/GtmuxMobile/AppDelegate.swift', 'mobileapp/ios/GtmuxMobile/Info.plist'):
            with self.subTest(path=path):
                selected = ci.classify([path])["selected"]
                self.assertIn('ios-scene', selected); self.assertIn('ios-native', selected)
        text = (ROOT / '.github/workflows/ci.yml').read_text()
        scene = re.split(r"\n  \S", text.split('  ios-scene:\n', 1)[1], maxsplit=1)[0]
        native = re.split(r"\n  \S", text.split('  ios-native:\n', 1)[1], maxsplit=1)[0]
        self.assertIn('runs-on: xcode-27', scene); self.assertIn('timeout-minutes: 60', scene)
        self.assertIn('timeout-minutes: 60', native)
        self.assertIn('appium-xcuitest-driver@12.20.0', scene)
        self.assertIn('npx appium driver install --source=npm appium-xcuitest-driver@11.14.0', native)
        self.assertIn('--execute 27 --repository "$GITHUB_WORKSPACE"', scene)
        self.assertIn('--execute 26 --repository "$GITHUB_WORKSPACE"', native)
        for block in (scene, native):
            self.assertIn('fetch-depth: 0', block)
            self.assertNotIn('head_ref', block); self.assertNotIn('continue-on-error', block)
            self.assertIn('if: always()', block); self.assertIn('if-no-files-found: error', block)
        self.assertEqual(text.count('  ios-scene:\n'), 1)
        labels = (ROOT / '.github/actionlint.yaml').read_text()
        self.assertIn('    - xcode-27\n', labels)

    def test_scene_driver_setup_preserves_pins_errors_and_existing_isolation(self):
        text = (ROOT / '.github/workflows/ci.yml').read_text()
        step = text.split('      - name: Install isolated migration driver (26)\n', 1)[1].split('\n      - name:', 1)[0]
        self.assertIn('APPIUM_HOME: ${{ runner.temp }}/scene-migration-driver-26', step)
        run = step.split('        run: ', 1)[1]
        command = textwrap.dedent(run.split('\n', 1)[1]) if run.startswith('|') else run.splitlines()[0]
        for status, collision in ((0, False), (7, False), (0, True)):
            with self.subTest(status=status, collision=collision), tempfile.TemporaryDirectory(prefix='scene-driver-setup-') as temp:
                root = Path(temp); home = root / 'isolated'; home.mkdir()
                package = home / 'package.json'; capture = root / 'npx.json'
                if collision: package.write_text('{"private":true,"sentinel":"keep"}')
                original = package.read_bytes() if collision else None
                shim = root / 'npx'
                shim.write_text('#!' + sys.executable + '\nimport json,os,sys\nfrom pathlib import Path\n'
                                'Path(os.environ["CAPTURE"]).write_text(json.dumps(sys.argv[1:]))\n'
                                'raise SystemExit(int(os.environ["INSTALL_EXIT"]))\n')
                shim.chmod(0o755)
                env = {**os.environ, 'PATH': str(root) + os.pathsep + os.environ['PATH'],
                       'APPIUM_HOME': str(home), 'CAPTURE': str(capture), 'INSTALL_EXIT': str(status)}
                completed = subprocess.run(['bash', '-e', '-c', command], cwd=root, env=env,
                                           capture_output=True, text=True, timeout=10)
                if collision:
                    self.assertNotEqual(completed.returncode, 0)
                    self.assertEqual(package.read_bytes(), original)
                    self.assertFalse(capture.exists())
                else:
                    self.assertEqual(completed.returncode, status, completed.stderr)
                    self.assertEqual(json.loads(capture.read_text()),
                                     ['appium', 'driver', 'install', '--source=npm', 'appium-xcuitest-driver@11.14.0'])
                    self.assertEqual(json.loads(package.read_text()), {'private': True, 'overrides': {
                        'appium-xcuitest-driver': {'appium-webdriveragent': '15.0.0'}}})

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
        self.assertEqual(ci.verdict(plan, self.results(ci.JOBS), scene_union="success", scene_download="success"), [])
        for name in ("scope", "quality", *ci.JOBS):
            for result in (None, "skipped", "failure", "cancelled"):
                with self.subTest(name=name, result=result):
                    needs = self.results(ci.JOBS); needs[name] = {"result": result}
                    self.assertTrue(ci.verdict(plan, needs, scene_union="success", scene_download="success"))

    def test_bad_plans_or_unselected_failure_are_rejected(self):
        for plan in (None, {}, {"selected": ["unknown"], "full": False},
                     {"selected": [], "full": True}, {"selected": [{}], "full": False},
                     {"selected": [], "full": "false"}, {"selected": ["test", "test"], "full": False}):
            self.assertTrue(ci.verdict(plan, self.results([])))
        needs = self.results([]); needs["web"]["result"] = "failure"
        self.assertTrue(ci.verdict(ci.classify(["README.md"]), needs))

    def test_scene_roles_follow_event_and_require_complete_union_plus_old_gates(self):
        for event in ('pull_request', 'push', 'workflow_dispatch'):
            plan = ci.classify(['mobileapp/App.tsx'], event=event)
            chosen = set(plan['selected'])
            self.assertEqual('ios-scene-baseline-26' in chosen, event == 'pull_request')
            needs = self.results(chosen)
            env = {**os.environ, 'CI_PLAN': json.dumps(plan), 'CI_RESULTS': json.dumps(needs), 'CI_EVENT': event,
                   'CI_SCENE_UNION': 'success', 'CI_SCENE_DOWNLOAD': 'success'}
            def run(overrides):
                return subprocess.run([sys.executable, str(ROOT/'scripts/ci.py'), 'verify'],
                    env={**env, **overrides}, capture_output=True, text=True, timeout=10)
            self.assertEqual(run({}).returncode, 0)
            for field in ('CI_SCENE_UNION', 'CI_SCENE_DOWNLOAD'):
                for bad in ('', 'skipped', 'failure', 'cancelled'):
                    self.assertNotEqual(run({field: bad}).returncode, 0, (event, field, bad))
            for job in chosen | {'quality', 'scope'}:
                for status in ('skipped', 'failure', 'cancelled', None):
                    changed = {k: dict(v) for k,v in needs.items()}; changed[job]['result'] = status
                    self.assertTrue(ci.verdict(plan, changed, event, 'success', 'success'), (event, job, status))
            wrong = dict(plan); wrong['event'] = 'push' if event == 'pull_request' else 'pull_request'
            self.assertNotEqual(run({'CI_PLAN': json.dumps(wrong)}).returncode, 0)
            wrong = dict(plan); wrong['selected'] = sorted(chosen ^ {'ios-scene-baseline-26'})
            self.assertNotEqual(run({'CI_PLAN': json.dumps(wrong)}).returncode, 0)
            docs = ci.classify(['README.md'], event=event)
            self.assertEqual(ci.verdict(docs, self.results([]), event), [])
        with self.assertRaises(ValueError): ci.classify(['mobileapp/App.tsx'], event='unknown')

    def test_scene_workflow_sequential_role_and_full_union_wiring(self):
        text = (ROOT/'.github/workflows/ci.yml').read_text()
        def block(job): return re.split(r'\n  \S', text.split('  '+job+':\n',1)[1],maxsplit=1)[0]
        baseline, native, scene = (block(n) for n in ('ios-scene-baseline-26', 'ios-native', 'ios-scene'))
        self.assertEqual(text.count('  ios-scene-baseline-26:\n'), 1)
        self.assertIn('needs: [scope, ios-native]', baseline)
        self.assertIn("if: always() && !cancelled() && github.event_name == 'pull_request' && needs.scope.result == 'success' && needs.scope.outputs.ios == 'true'", baseline)
        for role in (native, baseline):
            self.assertIn('runs-on: macos-15\n', role); self.assertIn('timeout-minutes: 60\n', role)
            self.assertEqual(role.count('--execute 26'), 1)
            self.assertNotIn('matrix:', role); self.assertNotIn('continue-on-error:', role)
        self.assertNotIn('CODE_SIGNING_ALLOWED=NO', baseline)
        self.assertIn('CODE_SIGNING_ALLOWED=NO', native)
        self.assertEqual(scene.count('--execute 27'), 1)
        for role in (native, baseline, scene):
            self.assertIn('name: scene-migration-${{ github.job }}-${{ github.run_id }}-${{ github.run_attempt }}', role)
            self.assertIn('if-no-files-found: error', role)
        gate = block('ci-required')
        self.assertIn('ios-scene-baseline-26', gate.split('steps:',1)[0])
        download = gate.split('- name: Download exact scene role artifacts\n',1)[1].split('      - name:',1)[0]
        self.assertIn("if: always() && needs.scope.outputs.scene == 'true'", download)
        self.assertIn('pattern: scene-migration-*-${{ github.run_id }}-${{ github.run_attempt }}', download)
        self.assertIn('merge-multiple: false', download)
        self.assertIn('path: ${{ runner.temp }}/scene-role-artifacts', download)
        union = gate.split('- name: Verify complete current-event scene union\n',1)[1].split('      - name:',1)[0]
        self.assertIn("if: always() && needs.scope.outputs.scene == 'true'", union)
        self.assertIn('run: python3 mobileapp/e2e/ci-scene-migration.py --verify-union "$RUNNER_TEMP/scene-role-artifacts" --repository "$GITHUB_WORKSPACE"', union)
        final = gate.split('- name: Require successful selected checks\n',1)[1]
        for text in ('if: always()', 'CI_EVENT: ${{ github.event_name }}',
                     'CI_SCENE_DOWNLOAD: ${{ steps.scene-download.outcome }}',
                     'CI_SCENE_UNION: ${{ steps.scene-union.outcome }}', 'run: python3 scripts/ci.py verify'):
            self.assertIn(text, final)


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
