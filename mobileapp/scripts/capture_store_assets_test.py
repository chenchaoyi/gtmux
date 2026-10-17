import importlib.util
from pathlib import Path
import subprocess
import tempfile
import unittest

from store_artifacts_test import png
from store_artifacts import GROUPS, checkpoint_group, load_checkpoint
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("capture", Path(__file__).with_name("capture-store-assets.py"))
capture = importlib.util.module_from_spec(spec)
spec.loader.exec_module(capture)


class CaptureTests(unittest.TestCase):
    def wda(self, home, nested=True):
        folder = home / "node_modules"
        if nested:
            folder = folder / "appium-xcuitest-driver/node_modules"
        folder = folder / "appium-webdriveragent"
        (folder / "WebDriverAgent.xcodeproj").mkdir(parents=True)
        (folder / "package.json").write_text('{"name":"appium-webdriveragent","version":"15.0.0"}')
        entry = folder / "build/lib/index.js"
        entry.parent.mkdir(parents=True)
        entry.touch()
        return entry, folder / "WebDriverAgent.xcodeproj"

    def test_wda_project_resolves_nested_and_hoisted_driver_dependencies(self):
        for nested in (True, False):
            with self.subTest(nested=nested), tempfile.TemporaryDirectory() as tmp:
                home = Path(tmp)
                entry, project = self.wda(home, nested)
                self.assertEqual(capture.wda_project(home, entry), (project.resolve(), "15.0.0"))
                with self.assertRaises(ValueError):
                    capture.wda_project(home / "other-run", entry)
                project.rmdir()
                with self.assertRaises(ValueError):
                    capture.wda_project(home, entry)

    def test_wda_prebuild_is_an_external_bounded_compiler_with_its_own_log(self):
        with tempfile.TemporaryDirectory() as tmp:
            home, output = Path(tmp) / "appium", Path(tmp) / "output"
            output.mkdir()
            entry, project = self.wda(home)
            resolver = []

            def run(args):
                resolver.append(args)
                return str(entry)

            with patch.object(capture.subprocess, "run") as compiler:
                capture.prebuild_wda(home, "owned-device-id", output, "phone", "26.5", run)
            args = compiler.call_args.args[0]
            self.assertEqual(args[:2], ["xcodebuild", "build-for-testing"])
            self.assertIn(str(project.resolve()), args)
            self.assertIn("id=owned-device-id", args)
            self.assertIn(str(output / "wda"), args)
            self.assertIn("IPHONEOS_DEPLOYMENT_TARGET=26.5", args)
            self.assertEqual(compiler.call_args.kwargs["timeout"], 900)
            self.assertTrue(compiler.call_args.kwargs["check"])
            self.assertEqual(resolver[0][:2], ["node", "-p"])
            self.assertTrue((output / "wda-build-phone.log").is_file())

    def test_capture_uses_explicit_bounded_hosted_startup_for_each_locale(self):
        calls = []
        launches = []
        order = []

        def run(args):
            calls.append(args)
            return "owned-device-id" if "create" in args else ""

        def launch(*args, **kwargs):
            launches.append({**kwargs, "env": dict(kwargs["env"])})
            order.append(kwargs["env"]["GTMUX_SHOTS_LANG"])

        with patch.dict(capture.os.environ, {"APPIUM_HOME": "/ci/appium"}), \
                patch.object(capture, "prebuild_wda", side_effect=lambda *args: order.append("build")) as prebuild, \
                patch.object(capture.subprocess, "run", side_effect=launch) as npm:
            capture.capture_device(Path("source"), Path("app"), Path("output"), "phone",
                                   {"identifier": "type", "name": "phone"},
                                   {"identifier": "runtime", "version": "26.5"}, run)
        self.assertEqual(npm.call_count, 2)
        self.assertEqual(order, ["build", "en", "zh"])
        prebuild.assert_called_once_with(Path("/ci/appium"), "owned-device-id", Path("output"), "phone", "26.5")
        for call, language in zip(launches, ("en", "zh")):
            self.assertEqual(call["timeout"], 1200)
            self.assertTrue(call["check"])
            env = call["env"]
            self.assertEqual(env["GTMUX_E2E_CAPTURE_CI"], "1")
            self.assertEqual(env["GTMUX_E2E_HEADLESS"], "1")
            self.assertEqual(env["GTMUX_E2E_UDID"], "owned-device-id")
            self.assertEqual(env["GTMUX_SHOTS_LANG"], language)
        self.assertEqual(calls[-2:], [["xcrun", "simctl", "shutdown", "owned-device-id"],
                                     ["xcrun", "simctl", "delete", "owned-device-id"]])

    def test_prebuild_failure_stops_before_appium_and_cleans_only_its_device(self):
        calls = []

        def run(args):
            calls.append(args)
            return "owned-device-id" if "create" in args else ""

        with patch.dict(capture.os.environ, {"APPIUM_HOME": "/ci/appium"}), \
                patch.object(capture, "prebuild_wda", side_effect=subprocess.TimeoutExpired("xcodebuild", 900)), \
                patch.object(capture.subprocess, "run") as npm:
            with self.assertRaises(subprocess.TimeoutExpired):
                capture.capture_device(Path("source"), Path("app"), Path("output"), "phone",
                                       {"identifier": "type", "name": "phone"},
                                       {"identifier": "runtime", "version": "26.5"}, run)
        npm.assert_not_called()
        self.assertEqual(calls[-2:], [["xcrun", "simctl", "shutdown", "owned-device-id"],
                                     ["xcrun", "simctl", "delete", "owned-device-id"]])

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

    def test_resume_skips_completed_locale_and_failure_keeps_its_checkpoint(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            output, raw = root / "output", root / "raw"
            context = {"release_tag": "v1.2.3"}
            for name in GROUPS["phone-en"]:
                png(raw / (name + ".png"), [1320, 2868], name)
            checkpoint_group(output, context, "phone-en", raw)
            commands = []
            def run(args):
                return "owned-device-id" if "create" in args else ""
            def launch(*args, **kwargs):
                commands.append(kwargs["env"]["GTMUX_SHOTS_LANG"])
                raise subprocess.CalledProcessError(1, args)
            with patch.dict(capture.os.environ, {"APPIUM_HOME": "/ci/appium"}), \
                    patch.object(capture, "prebuild_wda"), \
                    patch.object(capture.subprocess, "run", side_effect=launch):
                with self.assertRaises(subprocess.CalledProcessError):
                    capture.capture_device(root, root / "app", output, "phone", {"identifier": "type", "name": "phone"},
                                           {"identifier": "runtime", "version": "26.5"}, run, context)
            self.assertEqual(commands, ["zh"])
            self.assertEqual(set(load_checkpoint(output, context)["groups"]), {"phone-en"})

    def test_complete_device_does_not_create_or_boot_a_simulator(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            output = root / "output"
            context = {"release_tag": "v1.2.3"}
            for lang in ("en", "zh"):
                raw = root / lang
                for name in GROUPS["phone-" + lang]:
                    png(raw / (name + ".png"), [1320, 2868], name)
                checkpoint_group(output, context, "phone-" + lang, raw)
            with patch.object(capture, "command") as run, patch.object(capture, "prebuild_wda") as compiler:
                capture.capture_device(root, root / "app", output, "phone", {}, {}, run=run, context=context)
            run.assert_not_called()
            compiler.assert_not_called()

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
