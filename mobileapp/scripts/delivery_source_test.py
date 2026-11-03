import unittest
from delivery_source import DeliveryError, validate_android_jobs


class AndroidJobsTests(unittest.TestCase):
    def rows(self, names):
        return [{"name": name, "head_sha": "a" * 40, "status": "completed", "conclusion": "success"} for name in names]

    def test_legacy_and_parallel_sources(self):
        validate_android_jobs("a" * 40, self.rows(["Android build and emulator flows"]))
        validate_android_jobs("a" * 40, self.rows(["Android QA build and native policies", "Android phone emulator flows", "Android tablet emulator flows"]))

    def test_no_missing_failed_duplicate_or_mixed_gate_can_pass(self):
        rows = self.rows(["Android QA build and native policies", "Android phone emulator flows", "Android tablet emulator flows"])
        for bad in [[], rows[:2], rows + rows[:1], rows + self.rows(["Android build and emulator flows"])]:
            with self.subTest(bad=bad), self.assertRaises(DeliveryError): validate_android_jobs("a" * 40, bad)
        for field, value in [("head_sha", "b" * 40), ("status", "queued"), ("conclusion", "failure"), ("conclusion", "skipped")]:
            bad = [dict(row) for row in rows]; bad[-1][field] = value
            with self.subTest(field=field, value=value), self.assertRaises(DeliveryError): validate_android_jobs("a" * 40, bad)
