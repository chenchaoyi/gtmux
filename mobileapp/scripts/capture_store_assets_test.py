import importlib.util
from pathlib import Path
import subprocess
import unittest

spec = importlib.util.spec_from_file_location("capture", Path(__file__).with_name("capture-store-assets.py"))
capture = importlib.util.module_from_spec(spec)
spec.loader.exec_module(capture)


class CaptureTests(unittest.TestCase):
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
