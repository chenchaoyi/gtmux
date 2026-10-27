import importlib.util
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("widget_signing", ROOT / "macapp/Widget/prepare_signing.py")
signing = importlib.util.module_from_spec(spec)
spec.loader.exec_module(signing)


class MacWidgetTests(unittest.TestCase):
    def test_only_parent_retains_automation_and_only_extension_is_sandboxed(self):
        parent = {"com.apple.security.automation.apple-events": True}
        app, widget = signing.entitlements("SYNTH12345", parent)
        self.assertEqual(parent, {"com.apple.security.automation.apple-events": True})
        self.assertNotIn("com.apple.security.app-sandbox", app)
        self.assertEqual(widget, {"com.apple.security.app-sandbox": True,
                                 "com.apple.security.application-groups": ["SYNTH12345.com.gtmux.menubar.usage"]})
        self.assertEqual(app["com.apple.security.application-groups"], widget["com.apple.security.application-groups"])
        self.assertNotIn("com.apple.security.network.client", widget)

    def test_unsigned_build_cannot_claim_shared_access_and_invalid_team_is_rejected(self):
        app, widget = signing.entitlements("", {})
        self.assertEqual(app, {})
        self.assertEqual(widget, {"com.apple.security.app-sandbox": True})
        for team in ["../invalid", "group.com.gtmux.app.usage", "BAD", "SYNTH12345\n"]:
            with self.assertRaises(ValueError):
                signing.entitlements(team, {})

    def test_bundle_builder_signs_extension_before_parent_and_cli_has_no_group(self):
        text = (ROOT / "macapp/build.sh").read_text()
        self.assertNotIn("codesign --force --deep", text)
        self.assertIn('"$ENT" --timestamp --sign "$SIGN_ID" "$BUNDLE/Contents/MacOS/gtmux"', text)
        extension = text.index('"$WIDGET_ENT" --timestamp')
        parent = text.index('"$PARENT_ENT" --timestamp')
        self.assertLess(extension, parent)
        self.assertIn('Contents/PlugIns/GtmuxUsageWidget.appex', text)
        self.assertIn('Contents/Resources/GtmuxUsageWidgets_GtmuxUsageWidgets.bundle', text)

    def test_extension_has_no_cli_or_credentials_path(self):
        for path in (ROOT / "macapp/Widget").glob("*.swift"):
            text = path.read_text()
            for forbidden in ["GtmuxCLI", "URLSession", "Process(", "remotes.json", "serve-token", "UserDefaults.standard"]:
                self.assertNotIn(forbidden, text)
