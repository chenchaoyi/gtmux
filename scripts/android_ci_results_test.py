import copy
import json
from pathlib import Path
import tempfile
import unittest

import android_ci_results as gate


SOURCE = "a" * 40
PR_HEAD = "b" * 40
RUN = 45020309125


def job(part, attempt=1, conclusion="success"):
    return {"name": f"Android phone emulator shard {part}", "run_id": RUN,
            "head_sha": PR_HEAD, "run_attempt": attempt, "status": "completed", "conclusion": conclusion,
            "runner_id": part * 10 + attempt, "created_at": f"2026-11-22T12:{attempt*10:02d}:00Z",
            "started_at": f"2026-11-22T12:{attempt*10:02d}:01Z",
            "completed_at": f"2026-11-22T12:{attempt*10+5:02d}:00Z"}


class AndroidAttemptsTests(unittest.TestCase):
    def test_failed_only_rerun_binds_the_actual_execution_attempt(self):
        first = job(1)
        copied = {**first, "created_at": "2026-11-22T12:20:00Z", "run_attempt": 2}
        rows = [first, copied, job(2, conclusion="failure"), job(2, attempt=2)]
        self.assertEqual(gate.shard_attempts(rows, RUN, 2, PR_HEAD, "phone"), {1: 1, 2: 2})
        with self.assertRaisesRegex(ValueError, "no unique original execution"):
            gate.shard_attempts(rows[1:], RUN, 2, PR_HEAD, "phone")
        ambiguous = {**job(2, attempt=2), "created_at": "2026-11-22T12:10:00Z", "run_attempt": 1}
        with self.assertRaisesRegex(ValueError, "no unique original execution"):
            gate.shard_attempts([first, copied, ambiguous, job(2, attempt=2)], RUN, 2, PR_HEAD, "phone")

    def test_current_failure_skip_cancel_incomplete_or_missing_never_uses_an_older_pass(self):
        baseline = [job(1), job(2), job(2, attempt=2)]
        for status in ("failure", "skipped", "cancelled", None):
            with self.subTest(conclusion=status), self.assertRaisesRegex(ValueError, "current attempt did not succeed"):
                gate.shard_attempts([*baseline, job(1, attempt=2, conclusion=status)], RUN, 2, PR_HEAD, "phone")
        with self.assertRaisesRegex(ValueError, "current attempt did not succeed"):
            gate.shard_attempts(baseline, RUN, 2, PR_HEAD, "phone")

    def test_duplicate_or_foreign_job_history_is_rejected(self):
        current = job(1)
        rows = [job(2), current]
        for other in (current, {**current, "run_id": RUN+1}, {**current, "head_sha": SOURCE},
                      {**current, "run_attempt": 2}):
            with self.subTest(job=other), self.assertRaisesRegex(ValueError, "inconsistent run, head or attempt"):
                gate.shard_attempts([*rows, other], RUN, 1, PR_HEAD, "phone")


class AndroidCoverageTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.inventory = gate.suite()

    def setUp(self):
        work = tempfile.TemporaryDirectory()
        self.addCleanup(work.cleanup)
        self.root = Path(work.name)
        self.attempts = {1: 1, 2: 2}
        self.package = {"source": SOURCE, "run": RUN, "attempt": 1, "sha256": "c"*64, "bytes": 123}

    def receipts(self, device):
        cases, selections = self.inventory
        for part, selected in selections.items():
            folder = self.root / str(part)
            folder.mkdir(exist_ok=True)
            rows = []
            for case_id, case in cases.items():
                row = {"name": f"{case_id:02d}-synthetic", "group": case["group"], "capability": case["capability"]}
                if case_id in selected:
                    row.update(status="passed")
                else:
                    row.update(status="not-run", reason="Explicit subset")
                rows.append(row)
            (folder / "RESULTS.json").write_text(json.dumps({"results": rows, "selection": selected,
                "serial": "emulator-5554", "schema": 2, "package": "com.gtmux.app.qa", "first_automation_failure": None}))
            (folder / "LANE.json").write_text(json.dumps(gate.lane(SOURCE, RUN, self.attempts[part], device, part)))
            (folder / "APK.json").write_text(json.dumps({"file": "app-qa.apk", "bytes": 123, "sha256": "c"*64}))

    def verify(self, device):
        return gate.verify_device(self.root, self.inventory, self.package, SOURCE, RUN, self.attempts, device)

    def poison(self, filename, change):
        path = self.root / "2" / filename
        value = json.loads(path.read_text())
        change(value)
        path.write_text(json.dumps(value))

    def test_both_device_suites_require_each_case_once_on_the_original_qa_package(self):
        self.assertEqual(self.inventory[1], {1: list(range(1, 18)), 2: list(range(18, 32))})
        for device in gate.DEVICES:
            with self.subTest(device=device):
                self.receipts(device)
                self.assertEqual(self.verify(device), 31)
                self.poison("APK.json", lambda value: value.update(sha256="d"*64))
                with self.assertRaisesRegex(ValueError, "QA APK differs"):
                    self.verify(device)
        self.receipts("tablet")
        self.package["attempt"] = 2
        with self.assertRaisesRegex(ValueError, "wrong product, device, or attempt"):
            self.verify("tablet")

    def test_missing_or_invalid_case_and_lane_receipts_fail(self):
        for name, change in (
                ("missing", lambda x: x["results"].pop()),
                ("duplicate", lambda x: x["results"].__setitem__(30, x["results"][29])),
                ("failure", lambda x: x["results"][17].update(status="failed")),
                ("blocked", lambda x: x["results"][17].update(status="blocked")),
                ("skipped", lambda x: x["results"][17].update(status="not-run", reason="Explicit subset")),
                ("extra passed", lambda x: x["results"][0].update(status="passed")),
                ("automation lost", lambda x: x.update(first_automation_failure="18-synthetic")),
                ("wrong selection", lambda x: x.update(selection=gate.suite()[1][1])),
        ):
            with self.subTest(case=name):
                self.receipts("phone")
                self.poison("RESULTS.json", change)
                with self.assertRaises(ValueError):
                    self.verify("phone")
        for wrong in ({"device": "tablet"}, {"profile": "pixel_tablet"}, {"source": PR_HEAD}, {"attempt": 1}):
            with self.subTest(lane=wrong):
                self.receipts("phone")
                self.poison("LANE.json", lambda value: value.update(wrong))
                with self.assertRaisesRegex(ValueError, "wrong product, device, or attempt"):
                    self.verify("phone")
        self.receipts("phone")
        (self.root / "2" / "RESULTS.json").unlink()
        with self.assertRaisesRegex(ValueError, "Missing or unsafe RESULTS.json"):
            self.verify("phone")

    def test_partition_rejects_crossing_setup_groups_or_prerequisites(self):
        cases = list(copy.deepcopy(self.inventory[0]).values())
        cases[17]["requires"] = [17]
        with self.assertRaisesRegex(ValueError, "prerequisite crosses"):
            gate.partition(cases)
        cases[17]["requires"] = []
        cases[17]["group"] = cases[16]["group"]
        with self.assertRaisesRegex(ValueError, "setup group crosses"):
            gate.partition(cases)


if __name__ == "__main__":
    unittest.main()
