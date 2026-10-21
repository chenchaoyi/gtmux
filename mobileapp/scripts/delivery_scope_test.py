"""Build-scope regressions use synthetic Git trees; no signing or network."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).with_name("delivery-scope.py")
spec = importlib.util.spec_from_file_location("scope", SCRIPT)
scope = importlib.util.module_from_spec(spec)
spec.loader.exec_module(scope)


class DeliveryScopeTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.git("init", "-q")
        self.put("mobileapp/src/version.ts", "version = '1.0.142'")
        self.put("mobileapp/App.tsx", "original app")
        self.put("mobileapp/src/releaseNotes.ts", "bundled history")
        self.put("mobileapp/ios/App.swift", "original native app")
        self.put("mobileapp/ios/Project.xcodeproj/project.pbxproj", "MARKETING_VERSION = 1.0.142;\nOTHER_SETTING = YES;\n")
        self.commit("v1.0.142")
        self.releases = [{"tag_name": "v1.0.142", "draft": False, "prerelease": False}]

    def git(self, *args):
        return subprocess.check_output(["git", "-C", str(self.root), "-c", "user.name=Synthetic Test", "-c", "user.email=test@example.invalid", *args], stderr=subprocess.DEVNULL).decode().strip()

    def put(self, path, body):
        p = self.root / path; p.parent.mkdir(parents=True, exist_ok=True); p.write_text(body)

    def commit(self, tag):
        self.git("add", "."); self.git("commit", "-qm", "synthetic input")
        self.git("tag", tag)
        return self.git("rev-parse", "HEAD")

    def test_version_only_release_skips_even_with_mac_docs_and_test_changes(self):
        self.put("mobileapp/src/version.ts", "version = '1.0.143'")
        self.put("mobileapp/ios/Project.xcodeproj/project.pbxproj", "MARKETING_VERSION = 1.0.143;\nOTHER_SETTING = YES;\n")
        for path in ["macapp/Sources/Example.swift", "mobileapp/DISTRIBUTION.md", "mobileapp/e2e/example.js", "mobileapp/src/ui/example.test.tsx", "mobileapp/scripts/delivery-scope.py"]:
            self.put(path, "non-shipping change")
        head = self.commit("v1.0.143")
        result = scope.decide(self.root, "release-test", "v1.0.143", head, self.releases)
        self.assertFalse(result["build_required"])
        self.assertEqual(result["baseline"], "v1.0.142")

    def test_js_native_dependencies_notes_and_unknown_inputs_require_package(self):
        for path in ["mobileapp/App.tsx", "mobileapp/src/releaseNotes.ts", "mobileapp/ios/App.swift", "mobileapp/package-lock.json", "mobileapp/ios/Podfile.lock", "mobileapp/new-runtime-input.dat"]:
            with self.subTest(path=path):
                self.put(path, "changed input")
                head = self.commit("v2.0." + str(len(self.git("tag").splitlines())))
                self.assertIn(path, scope.build_inputs_changed(self.root, "v1.0.142", head))

    def test_native_setting_change_is_not_hidden_by_version_normalization(self):
        path = "mobileapp/ios/Project.xcodeproj/project.pbxproj"
        self.put(path, "MARKETING_VERSION = 1.0.143;\nOTHER_SETTING = NO;\n")
        self.assertEqual(scope.build_inputs_changed(self.root, "v1.0.142", self.commit("v1.0.143")), [path])

    def test_deletion_and_rename_out_of_shipping_tree_require_package(self):
        (self.root / "mobileapp/App.tsx").rename(self.root / "mobileapp/DISTRIBUTION.md")
        head = self.commit("v1.0.143")
        self.assertIn("mobileapp/App.tsx", scope.build_inputs_changed(self.root, "v1.0.142", head))

    def test_baseline_is_numeric_published_stable_not_draft_future_or_preview(self):
        releases = [{"tag_name": tag} for tag in ["v1.0.9", "v1.0.10", "v1.0.12", "v1.0.11-rc.1"]]
        releases += [{"tag_name": "v1.0.11", "draft": True}, {"tag_name": "v1.0.11", "prerelease": True}]
        self.assertEqual(scope.previous_release("v1.0.12", releases), "v1.0.10")

    def test_missing_or_unrelated_baseline_and_mismatched_tag_fail_closed(self):
        head = self.git("rev-parse", "HEAD")
        for tag in ["v0.9.0", "v1.0.143"]:
            with self.assertRaises(subprocess.CalledProcessError):
                scope.build_inputs_changed(self.root, tag, head)
        self.put("mobileapp/App.tsx", "new app")
        newer = self.commit("v1.0.143")
        with self.assertRaises(ValueError):
            scope.decide(self.root, "release-test", "v1.0.142", newer, self.releases)
        with self.assertRaises(subprocess.CalledProcessError):
            scope.build_inputs_changed(self.root, "v1.0.143", head)

    def test_first_release_and_explicit_preview_can_build_without_diff(self):
        head = self.git("rev-parse", "HEAD")
        self.assertTrue(scope.decide(self.root, "release-test", "v1.0.142", head, [])["build_required"])
        self.assertTrue(scope.decide(self.root, "ui-preview", "", head, [])["build_required"])

    def test_workflow_checks_scope_before_protected_native_job(self):
        workflow = SCRIPT.parents[2] / ".github/workflows/ios-test-delivery.yml"
        body = workflow.read_text()
        self.assertIn("if: needs.trusted-source.outputs.build_required == 'true'", body)
        self.assertLess(body.index("id: scope"), body.index("environment: ios-test-delivery"))
        output = self.root / "github-output"
        releases = self.root / "releases.json"; releases.write_text(json.dumps(self.releases))
        self.put("mobileapp/src/version.ts", "version = '1.0.143'")
        head = self.commit("v1.0.143")
        subprocess.check_output(["python3", str(SCRIPT), "--root", str(self.root), "--purpose", "release-test", "--tag", "v1.0.143", "--commit", head, "--releases", str(releases)], env=dict(os.environ, GITHUB_OUTPUT=str(output)))
        self.assertEqual(output.read_text(), "build_required=false\n")


if __name__ == "__main__":
    unittest.main()
