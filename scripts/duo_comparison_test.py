"""Run the hosted comparison wiring against owned command stubs, never a device."""
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
CONTEXT = '      await saveServers(saved);\n      storeRef.current = saved;'
PICK = '  const onPick = async (url: string) => {\n    if (url === activeUrl) {'
PUSH = 'async function request() {\n      await PushNotificationIOS.requestPermissions();\n}\n'

STUB = r'''
import json, os, pathlib, shutil, sys
name = pathlib.Path(sys.argv[0]).name
args = sys.argv[1:]
workspace = pathlib.Path(os.environ['GITHUB_WORKSPACE'])
variant = 'baseline' if 'duo-baseline-source' in str(pathlib.Path.cwd()) else 'candidate'
with open(os.environ['FIXTURE_CALLS'], 'a') as out:
    out.write(json.dumps({'command': name, 'args': args, 'variant': variant,
        'manual': os.environ.get('GTMUX_COMPARE_MANUAL_LAUNCH'),
        'os': os.environ.get('GTMUX_E2E_OS'), 'device': os.environ.get('GTMUX_E2E_DEVICE'),
        'udid': os.environ.get('GTMUX_E2E_UDID')}) + '\n')
if name == 'git':
    if args[0] == 'archive':
        sys.stdout.buffer.write((workspace / 'baseline.tar').read_bytes())
    elif 'show' in args:
        sys.stdout.buffer.write((workspace / args[-1].split(':', 1)[1]).read_bytes())
    else:
        sys.exit('unexpected git command')
elif name == 'rsync':
    shutil.copytree(args[-2], args[-1], dirs_exist_ok=True)
elif name == 'codesign':
    print('Signature=' + os.environ.get('FIXTURE_SIGNATURE', 'adhoc'), file=sys.stderr)
elif name == 'xcrun':
    if args[:3] == ['simctl', 'list', '-j']:
        print(json.dumps({'devices': {'com.apple.CoreSimulator.SimRuntime.iOS-' + os.environ['GTMUX_COMPARE_OS'].replace('.', '-'): [
            {'udid': 'owned-fixture-only', 'name': 'iPhone 17', 'isAvailable': True}]}}))
elif name == 'npm':
    sys.exit(int(os.environ.get('FIXTURE_' + variant.upper() + '_STATUS', '0')))
elif name not in ('bundle', 'xcodebuild'):
    sys.exit('unexpected command ' + name)
'''


class DuoComparisonTests(unittest.TestCase):
    def fixture(self, root):
        workspace = root / 'source'
        temp = root / 'runner'
        binary = root / 'bin'
        for directory in (workspace, temp, binary):
            directory.mkdir()
        files = {
            'mobileapp/src/state/AppContext.tsx': CONTEXT,
            'mobileapp/src/screens/ServersScreen.tsx': PICK,
            'mobileapp/src/push/index.ts': PUSH,
            'mobileapp/package-lock.json': '{}',
            'mobileapp/ios/Podfile.lock': 'owned fixture',
            'mobileapp/Gemfile.lock': 'owned fixture',
            'mobileapp/.bundle/config': 'owned fixture',
            'mobileapp/metro.config.js': 'module.exports = {};\n',
            'mobileapp/e2e/instrument-baseline.py': (ROOT / 'mobileapp/e2e/instrument-baseline.py').read_text(),
        }
        for name, text in files.items():
            target = workspace / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(text)
        for name in ('node_modules', 'vendor'):
            (workspace / 'mobileapp' / name).mkdir()
        with tarfile.open(workspace / 'baseline.tar', 'w') as archive:
            for name, text in files.items():
                data = text.encode()
                entry = tarfile.TarInfo(name)
                entry.size = len(data)
                archive.addfile(entry, io.BytesIO(data))
        stub = binary / 'fixture-command'
        stub.write_text('#!' + sys.executable + '\n' + STUB)
        stub.chmod(0o700)
        for name in ('git', 'rsync', 'codesign', 'xcrun', 'bundle', 'xcodebuild', 'npm'):
            (binary / name).symlink_to(stub)
        env = {**os.environ, 'PATH': str(binary) + os.pathsep + os.environ['PATH'],
               'RUNNER_TEMP': str(temp), 'GITHUB_WORKSPACE': str(workspace), 'GITHUB_BASE_SHA': 'b' * 40,
               'FIXTURE_CALLS': str(root / 'calls.jsonl'), 'GTMUX_COMPARE_ADHOC': '1',
               'GTMUX_COMPARE_OS': '27.0', 'GTMUX_COMPARE_DEVICE': 'iPhone 17'}
        for key in ('GTMUX_COMPARE_JOURNEY', 'GTMUX_COMPARE_REUSE_CANDIDATE', 'GTMUX_COMPARE_MANUAL_LAUNCH'):
            env.pop(key, None)
        return workspace, env

    def run_comparison(self, root, **options):
        workspace, env = self.fixture(root)
        env.update(options)
        result = subprocess.run(['bash', str(ROOT / 'mobileapp/e2e/ci-compare-servers.sh')],
                                cwd=workspace, env=env, text=True, capture_output=True, timeout=20)
        calls = [json.loads(line) for line in (root / 'calls.jsonl').read_text().splitlines()]
        return result, calls, workspace

    def test_push_comparison_reuses_candidate_with_identical_runtime_and_launch_policy(self):
        with tempfile.TemporaryDirectory(prefix='duo-comparison-') as tmp:
            result, calls, workspace = self.run_comparison(Path(tmp), GTMUX_COMPARE_JOURNEY='push',
                GTMUX_COMPARE_REUSE_CANDIDATE='1', GTMUX_COMPARE_MANUAL_LAUNCH='1')
            self.assertEqual(result.returncode, 0, result.stderr)
            builds = [call for call in calls if call['command'] == 'xcodebuild']
            self.assertEqual(len(builds), 1)
            self.assertEqual(builds[0]['variant'], 'baseline')
            self.assertIn('CODE_SIGN_IDENTITY=-', builds[0]['args'])
            tests = [call for call in calls if call['command'] == 'npm']
            self.assertEqual([call['variant'] for call in tests], ['baseline', 'candidate'])
            for call in tests:
                self.assertEqual(call['args'], ['run', 'test:e2e', '--', '--runInBand', 'coldstart-push.test.ts'])
                self.assertIsNone(call['manual'])
                self.assertEqual((call['os'], call['device'], call['udid']), ('27.0', 'iPhone 17', 'owned-fixture-only'))
            installs = [call['args'][-1] for call in calls if call['command'] == 'xcrun' and call['args'][1] == 'install']
            self.assertTrue(installs[-1].endswith('/gtmux-ios-ci/Build/Products/Release-iphonesimulator/gtmux.app'))
            self.assertEqual((workspace / 'mobileapp/.e2e-artifacts/comparison/candidate-status.txt').read_text(), '0\n')

    def test_default_server_control_preserves_its_two_builds_and_manual_launch(self):
        with tempfile.TemporaryDirectory(prefix='duo-comparison-') as tmp:
            result, calls, _ = self.run_comparison(Path(tmp), GTMUX_COMPARE_OS='26.2')
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(len([call for call in calls if call['command'] == 'xcodebuild']), 2)
            tests = [call for call in calls if call['command'] == 'npm']
            self.assertEqual(len(tests), 2)
            for call in tests:
                self.assertEqual(call['args'][-1], 'server-switch-compare.test.ts')
                self.assertEqual((call['manual'], call['os']), ('1', '26.2'))

    def test_each_original_variant_failure_stays_red_and_both_variants_run(self):
        for variant in ('baseline', 'candidate'):
            with self.subTest(variant=variant), tempfile.TemporaryDirectory(prefix='duo-comparison-') as tmp:
                result, calls, workspace = self.run_comparison(Path(tmp), GTMUX_COMPARE_JOURNEY='push',
                    GTMUX_COMPARE_REUSE_CANDIDATE='1', **{'FIXTURE_' + variant.upper() + '_STATUS': '7'})
                self.assertEqual(result.returncode, 1, result.stderr)
                self.assertEqual(len([call for call in calls if call['command'] == 'npm']), 2)
                self.assertEqual((workspace / f'mobileapp/.e2e-artifacts/comparison/{variant}-status.txt').read_text(), '7\n')

    def test_reuse_rejects_unsigned_product_before_any_build_or_device_command(self):
        with tempfile.TemporaryDirectory(prefix='duo-comparison-') as tmp:
            result, calls, _ = self.run_comparison(Path(tmp), GTMUX_COMPARE_JOURNEY='push',
                GTMUX_COMPARE_REUSE_CANDIDATE='1', FIXTURE_SIGNATURE='none')
            self.assertEqual(result.returncode, 2)
            self.assertFalse(any(call['command'] in ('npm', 'xcrun', 'xcodebuild') for call in calls))

    def test_baseline_permission_breadcrumbs_are_idempotent_and_do_not_add_a_request(self):
        with tempfile.TemporaryDirectory(prefix='duo-comparison-') as tmp:
            workspace, env = self.fixture(Path(tmp))
            env['GTMUX_COMPARE_JOURNEY'] = 'push'
            command = [sys.executable, str(ROOT / 'mobileapp/e2e/instrument-baseline.py'), str(workspace)]
            subprocess.run(command, env=env, check=True, capture_output=True, timeout=10)
            first = (workspace / 'mobileapp/src/push/index.ts').read_text()
            subprocess.run(command, env=env, check=True, capture_output=True, timeout=10)
            self.assertEqual((workspace / 'mobileapp/src/push/index.ts').read_text(), first)
            self.assertEqual(first.count('await PushNotificationIOS.requestPermissions()'), 1)
            self.assertEqual(first.count("import {Debug} from '../debug';"), 1)
            self.assertLess(first.index("event: 'push-permission-start'"), first.index('await PushNotificationIOS'))
            self.assertGreater(first.index("event: 'push-permissions'"), first.index('await PushNotificationIOS'))

    def test_unknown_baseline_permission_source_fails_closed(self):
        with tempfile.TemporaryDirectory(prefix='duo-comparison-') as tmp:
            workspace, env = self.fixture(Path(tmp))
            env['GTMUX_COMPARE_JOURNEY'] = 'push'
            (workspace / 'mobileapp/src/push/index.ts').write_text('unknown implementation')
            result = subprocess.run([sys.executable, str(ROOT / 'mobileapp/e2e/instrument-baseline.py'), str(workspace)],
                                    env=env, text=True, capture_output=True, timeout=10)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('permission instrumentation does not match', result.stderr)


if __name__ == '__main__':
    unittest.main()
