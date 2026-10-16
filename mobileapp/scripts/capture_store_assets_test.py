import importlib.util
from pathlib import Path
import subprocess
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("capture", Path(__file__).with_name("capture-store-assets.py"))
capture = importlib.util.module_from_spec(spec)
spec.loader.exec_module(capture)


class CaptureTests(unittest.TestCase):
    def test_harness_overlay_copies_only_allowlisted_test_files(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source, harness = root / "release", root / "harness"
            marker = source / "src" / "app.ts"
            marker.parent.mkdir(parents=True)
            marker.write_text("released app")
            for name in capture.CAPTURE_HARNESS:
                path = harness / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("current capture harness: " + name)
            (harness / "production.ts").write_text("must not be copied")
            hashes = capture.copy_capture_harness(source, harness)
            self.assertEqual(set(hashes), set(capture.CAPTURE_HARNESS))
            self.assertEqual(marker.read_text(), "released app")
            self.assertFalse((source / "e2e" / "production.ts").exists())
            for name in capture.CAPTURE_HARNESS:
                self.assertEqual((source / "e2e" / name).read_text(), (harness / name).read_text())

    def test_incomplete_harness_is_rejected_before_any_overlay(self):
        with tempfile.TemporaryDirectory() as tmp:
            source, harness = Path(tmp) / "release", Path(tmp) / "harness"
            harness.mkdir()
            with self.assertRaises(ValueError):
                capture.copy_capture_harness(source, harness)
            self.assertFalse(source.exists())

    def types(self):
        return [{"identifier": "com.apple.CoreSimulator.SimDeviceType." + name, "name": name}
                for name in ["iPhone-17-Pro-Max", "iPad-Pro-13-inch-M5-12GB"]]

    def test_uses_available_ios_runtime_and_required_devices(self):
        runtimes = [{"platform": "iOS", "isAvailable": True, "version": "26.5"},
                    {"platform": "iOS", "isAvailable": False, "version": "27.1"},
                    {"platform": "iOS", "isAvailable": True, "version": "27.0"},
                    {"platform": "tvOS", "isAvailable": True, "version": "30.0"}]
        runtime, devices = capture.simulator_plan(runtimes, self.types())
        self.assertEqual(runtime["version"], "27.0")
        self.assertEqual([slot for slot, _ in devices], ["phone", "ipad"])

    def test_refuses_missing_runtime_or_required_device(self):
        with self.assertRaises(ValueError):
            capture.simulator_plan([], self.types())
        with self.assertRaises(ValueError):
            capture.simulator_plan([{"platform": "iOS", "isAvailable": True, "version": "27"}], [])

    def test_boot_failure_cleans_only_the_device_it_created(self):
        calls = []

        def run(args):
            calls.append(args)
            if "create" in args:
                return "owned-device-id"
            if "boot" in args:
                raise subprocess.CalledProcessError(1, args)
            return ""

        with self.assertRaises(subprocess.CalledProcessError):
            capture.capture_device(Path("source"), Path("app"), Path("output"), "phone",
                                   {"identifier": "type"}, {"identifier": "runtime"}, run)
        self.assertEqual(calls[-2:], [["xcrun", "simctl", "shutdown", "owned-device-id"],
                                     ["xcrun", "simctl", "delete", "owned-device-id"]])


if __name__ == "__main__":
    unittest.main()
