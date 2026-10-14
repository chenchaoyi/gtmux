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
                         {"mobile", "web", "ios-native"})
        self.assertEqual(set(ci.classify(["mobileapp/ios/GtmuxWidget/Widget.swift"])["selected"]),
                         {"mobile", "ios-native"})
        self.assertEqual(set(ci.classify(["relay-worker/src/index.ts"])["selected"]),
                         {"relay-worker", "test", "mobile"})

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


if __name__ == "__main__":
    unittest.main()
