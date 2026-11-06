#!/usr/bin/env python3
"""Bounded scene migration acceptance: exact sources, isolated evidence, fail closed.

This module never interprets a successful build or a zero Jest exit as complete
acceptance. Execution is allowed only on the reviewed hosted CI surface; the
command-line verifier can independently check already captured evidence.
"""
from pathlib import Path
import argparse
import datetime
import difflib
import os
import plistlib
import shutil
import signal
import time
import hashlib
import json
import re
import subprocess

SHA = re.compile(r'^[0-9a-f]{40}$')
CASES = {
    'servers': (
        'selects a server after a clean launch',
        'switches between two saved servers',
    ),
    'coldstart': (
        'a cold-start push is handled once A: cold start with an active Mac, switches, background, a new warm tap',
        'a cold-start push is handled once C: a launch with no notification switches where it is pointed',
    ),
    'actions': ('delivers a cold notification action exactly once', 'delivers a warm notification action exactly once'),
    'links': ('opens a cold usage link through the scene', 'opens a warm usage link through the scene'),
    'ipad': ('keeps the iPad sidebar and selected session through rotation',),
}
STEPS = {
    'servers': ('select-after', 'switch-before', 'switch-after'),
    'coldstart': ('A0-home-connected', 'A1-after-cold-tap', 'A2-switch-office', 'A3-switch-home',
                  'A4-switch-office-again', 'A5-after-background', 'A6-after-warm-tap-office13',
                  'A7-switch-home-after-warm', 'C1-home', 'C2-office'),
    'actions': ('cold-native-headless', 'cold-fake-once', 'same-host-attached', 'fresh-root-before',
                'reader-selected', 'trace-used', 'diff-used', 'draft-entered', 'fresh-root-rotated',
                'draft-keyboard-retained', 'returned-to-list', 'warm-background', 'warm-fake-once'),
    'links': ('cold-not-running', 'cold-native-new-scene-url', 'cold-usage-visible',
              'warm-native-new-scene-url', 'warm-usage-visible'),
    'ipad': ('sidebar-reader-landscape', 'reader-selected', 'trace-used', 'diff-used', 'draft-entered',
             'sidebar-reader-portrait', 'draft-keyboard-retained'),
}
TOOLS = {
    '26': {'xcode': '26.3', 'sdk': '26.2', 'runtime': '26.2', 'driver': '11.14.0', 'wda': '15.0.0'},
    '27': {'xcode': '27.1', 'sdk': '27.1', 'runtime': '27.0', 'driver': '12.20.0', 'wda': '16.17.1'},
}


def revision(value):
    if not isinstance(value, str) or not SHA.fullmatch(value) or set(value) == {'0'}:
        raise ValueError('Missing or invalid exact revision')
    return value


def sources(event_name, event, github_sha):
    """Use real event identities; never replace an unreadable base with HEAD."""
    revision(github_sha)  # For PR this may be the test merge; record, never use as P.
    if event_name == 'pull_request':
        pr = event['pull_request']
        return {'baseline': revision(pr['base']['sha']), 'candidate': revision(pr['head']['sha'])}
    if event_name == 'push' and event.get('ref') == 'refs/heads/main':
        actual = revision(event['after'])
        if actual != github_sha:
            raise ValueError('actualmain does not match event.after')
        return {'candidate': actual}
    raise ValueError('Unsupported migration event')


def units(identities, toolchain):
    if toolchain not in TOOLS or 'candidate' not in identities or set(identities) - {'candidate', 'baseline'}:
        raise ValueError('Invalid migration matrix')
    for sha in identities.values():
        revision(sha)
    result = []
    for variant in ('baseline', 'candidate'):
        if variant not in identities or (toolchain == '27' and variant == 'baseline'):
            continue
        journeys = ('servers', 'coldstart') if toolchain == '26' else ('coldstart', 'actions', 'links', 'ipad')
        for journey in journeys:
            result.append({'toolchain': toolchain, 'runtime': TOOLS[toolchain]['runtime'],
                           'variant': variant, 'source': identities[variant], 'journey': journey})
    return result


def tuple_key(unit):
    if not isinstance(unit, dict) or set(unit) != {'toolchain', 'runtime', 'variant', 'source', 'journey'}:
        raise ValueError('Invalid unit fields')
    toolchain = unit['toolchain']
    if toolchain not in TOOLS or unit['runtime'] != TOOLS[toolchain]['runtime']:
        raise ValueError('Unexpected toolchain/runtime')
    if unit['variant'] not in {'baseline', 'candidate'} or (toolchain == '27' and unit['variant'] != 'candidate'):
        raise ValueError('Unexpected variant')
    allowed = ('servers', 'coldstart') if toolchain == '26' else ('coldstart', 'actions', 'links', 'ipad')
    if unit['journey'] not in allowed:
        raise ValueError('Unexpected journey')
    revision(unit['source'])
    return '/'.join((toolchain, unit['runtime'], unit['source'], unit['journey']))


def create_job_root(runner_temp, run_id, attempt, job):
    if not all(re.fullmatch(r'[A-Za-z0-9_-]+', str(x)) for x in (run_id, attempt, job)):
        raise ValueError('Unsafe job identity')
    parent = Path(runner_temp).resolve() / 'scene-migration'
    parent.mkdir(exist_ok=True)
    root = parent / f'{run_id}-{attempt}-{job}'
    root.mkdir()  # Exclusive: an existing attempt cannot overwrite its evidence.
    return root


def allocate_unit(root, unit):
    key = tuple_key(unit)
    paths = {name: Path(root) / name / key for name in ('work', 'evidence')}
    for path in paths.values():
        path.mkdir(parents=True, exist_ok=False)
    return paths


def archive_source(repository, sha, destination):
    """Real git root handling works even when invoked from mobileapp/."""
    revision(sha)
    repo = Path(subprocess.check_output(['git', '-C', str(repository), 'rev-parse', '--show-toplevel'], text=True, timeout=15).strip())
    subprocess.run(['git', '-C', str(repo), 'cat-file', '-e', sha + '^{commit}'], check=True, timeout=15)
    target = Path(destination)
    target.mkdir(parents=True, exist_ok=False)
    archive = target.parent / (target.name + '.tar')
    # Disk-backed archive, no unbounded PIPE buffer; remove only this newly created
    # transient tar after successful extraction. No shared worktree is touched.
    with archive.open('xb') as output:
        subprocess.run(['git', '-C', str(repo), 'archive', sha], stdout=output, check=True, timeout=60)
    subprocess.run(['tar', '-xf', str(archive), '-C', str(target)], check=True, timeout=60)
    archive.unlink()
    return target


def write_once(path, value):
    with Path(path).open('x') as stream:
        json.dump(value, stream, indent=2)
        stream.write('\n')


def product_errors(unit, evidence):
    errors = []
    for key, value in TOOLS[unit['toolchain']].items():
        if evidence.get(key) != value:
            errors.append(f'{key} missing or mismatched')
    for key in ('runtimeID', 'runtimeBuild', 'udid', 'deviceType', 'sdkroot', 'linkSysroot',
                'developerDir', 'wdaDeployment', 'wdaTargetDeployment', 'wdaBinarySHA', 'sessionID', 'installedAt', 'launchedAt'):
        if not evidence.get(key):
            errors.append(f'{key} missing')
    if type(evidence.get('pid')) is not int or evidence.get('pid', 0) <= 0:
        errors.append('PID missing')
    if evidence.get('source') != unit['source'] or evidence.get('signature') != 'adhoc':
        errors.append('source/signature mismatch')
    if not re.fullmatch(r'[0-9a-f]{64}', evidence.get('wdaBinarySHA', '')):
        errors.append('WDA binary hash invalid')
    for key in ('sdkroot', 'linkSysroot'):
        if ('iphonesimulator' + TOOLS[unit['toolchain']]['sdk']) not in str(evidence.get(key, '')).lower():
            errors.append(key + ' does not identify the required SDK')
    products = evidence.get('products', [])
    if len(products) != 3 or {p.get('kind') for p in products} != {'app', 'liveActivity', 'notificationExtension'}:
        errors.append('app and two extensions missing')
    for product in products:
        if (product.get('architectures') != ['arm64'] or product.get('signature') != 'adhoc'
                or not re.fullmatch(r'[0-9a-f]{64}', product.get('sha256', ''))
                or not product.get('uuid') or not product.get('deployment')
                or product.get('sdk') != 'iphonesimulator' + TOOLS[unit['toolchain']]['sdk']
                or 'entitlements' not in product):
            errors.append('product identity incomplete')
    app = next((p for p in products if p.get('kind') == 'app'), {})
    if app.get('deployment') != '15.1':
        errors.append('app deployment changed')
    return errors


def unit_errors(unit, result, artifact_root):
    errors = []
    if result.get('unit') != unit:
        errors.append('unit identity mismatch')
    if type(result.get('testExit')) is not int or result.get('testExit') != 0 or result.get('state') != 'completed':
        errors.append('test did not complete successfully')
    errors.extend(product_errors(unit, result.get('identity', {})))
    if result.get('problems'): errors.append('execution evidence or cleanup failed')
    expected = set(CASES[unit['journey']])
    cases = result.get('cases', [])
    if len(cases) != len(expected) or {c.get('name') for c in cases} != expected:
        errors.append('missing or duplicate registered case')
    if any(c.get('status') != 'passed' or not c.get('startedAt') or not c.get('completedAt') for c in cases):
        errors.append('case skipped, failed, cancelled or incomplete')
    if (len(result.get('steps', [])) != len(STEPS[unit['journey']])
            or set(result.get('steps', [])) != set(STEPS[unit['journey']])):
        errors.append('required behavior step missing or unexpected')
    artifacts = result.get('artifacts', [])
    required = {'jest.json', 'harness.json', 'session/session.json', 'session/behavior-steps.jsonl'}
    names = {a.get('path') for a in artifacts if a.get('bytes', 0) > 0}
    if not required <= names or not any(str(n).endswith('.png') for n in names) or not any(str(n).endswith('.xml') for n in names):
        errors.append('required execution, harness or UI evidence missing')
    if not any(str(n).endswith('-fake.json') or str(n).startswith('session/requests-') for n in names):
        errors.append('fake request evidence missing')
    if unit['journey'] in {'actions', 'links', 'ipad'} and not any(str(n).endswith('-native.jsonl') for n in names):
        errors.append('native journey evidence missing')
    if unit['journey'] in {'actions', 'ipad'} and not any(str(n).endswith('-geometry.json') for n in names):
        errors.append('host geometry evidence missing')
    root = Path(artifact_root).resolve()
    for artifact in artifacts:
        try:
            relative = Path(artifact['path'])
            if relative.is_absolute() or '..' in relative.parts:
                raise ValueError('unsafe evidence path')
            unresolved = root / relative
            if unresolved.is_symlink():
                raise ValueError('linked evidence')
            path = unresolved.resolve(strict=True)
            path.relative_to(root)
            if not path.is_file():
                raise ValueError('nonfile evidence')
            data = path.read_bytes()
            if len(data) != artifact['bytes'] or hashlib.sha256(data).hexdigest() != artifact['sha256']:
                raise ValueError('artifact changed')
        except (OSError, ValueError, KeyError, TypeError):
            errors.append('artifact identity invalid')
    return errors


def verify_all(expected_units, results, root):
    errors = []
    if len(results) != len(expected_units):
        errors.append('matrix execution incomplete')
    by_key = {}
    for result in results:
        unit = result.get('unit', {})
        try:
            key = tuple_key(unit)
        except (KeyError, ValueError, TypeError):
            errors.append('invalid result unit')
            continue
        if key in by_key:
            errors.append('duplicate execution result')
        by_key[key] = result
    for unit in expected_units:
        key = tuple_key(unit)
        if key not in by_key:
            errors.append('missing ' + key)
        else:
            errors.extend(key + ': ' + e for e in unit_errors(unit, by_key[key], Path(root) / key))
    if set(by_key) - {tuple_key(u) for u in expected_units}:
        errors.append('unexpected execution unit')
    return errors


# Execution is confined to an ephemeral hosted runner. Local controls exercise
# these functions with small synthetic commands and files, never a simulator.
class Runner:
    def __init__(self, root, env):
        self.root, self.env = Path(root), dict(env)
        self.sequence = 0

    def command(self, args, *, cwd, label, timeout, check=True, limit=64 * 1024 * 1024):
        self.sequence += 1
        stem = self.root / f'{self.sequence:03d}-{label}'
        log = stem.with_suffix('.log')
        started = now()
        state, code = 'completed', None
        with log.open('xb') as output:
            child = subprocess.Popen([str(x) for x in args], cwd=cwd, env=self.env,
                                     stdout=output, stderr=subprocess.STDOUT, start_new_session=True)
            deadline = time.monotonic() + timeout
            try:
                while child.poll() is None:
                    if time.monotonic() >= deadline or log.stat().st_size > limit:
                        state = 'timeout' if time.monotonic() >= deadline else 'output-limit'
                        os.killpg(child.pid, signal.SIGKILL)
                        break
                    time.sleep(.1)
                code = child.wait(timeout=10)
                if state == 'completed' and log.stat().st_size > limit: state = 'output-limit'
                if state == 'completed' and time.monotonic() > deadline: state = 'timeout'
            finally:
                if child.poll() is None:
                    os.killpg(child.pid, signal.SIGKILL)
                    child.wait(timeout=10)
        write_once(stem.with_suffix('.json'), {'command': [str(x) for x in args], 'cwd': str(cwd),
                   'startedAt': started, 'completedAt': now(), 'state': state, 'exit': code, 'log': str(log)})
        if state != 'completed' or (check and code != 0):
            raise RuntimeError(f'{label}: {state}, exit={code}; see {log}')
        return code, log

    def text(self, args, *, cwd, label, timeout=30, check=True):
        code, log = self.command(args, cwd=cwd, label=label, timeout=timeout, check=check, limit=4 * 1024 * 1024)
        return code, log.read_text(errors='replace')


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def file_record(path, relative_to=None):
    path = Path(path)
    digest = hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            digest.update(block)
    return {'path': str(path.relative_to(relative_to) if relative_to else path),
            'bytes': path.stat().st_size, 'sha256': digest.hexdigest()}


def exact_device(devices, runtimes, runtime_version, name):
    runtime = [r for r in runtimes['runtimes'] if r.get('version') == runtime_version and r.get('isAvailable')]
    if len(runtime) != 1:
        raise ValueError('Required available runtime is not unique')
    selected = [d for d in devices['devices'].get(runtime[0]['identifier'], []) if d.get('name') == name and d.get('isAvailable')]
    if len(selected) != 1 or not selected[0].get('deviceTypeIdentifier'):
        raise ValueError('Required device and hardware identity are not unique')
    return {**selected[0], 'runtimeID': runtime[0]['identifier'], 'runtimeBuild': runtime[0]['buildversion'],
            'runtime': runtime[0]['version']}


def preflight(runner, toolchain, cwd):
    expected = TOOLS[toolchain]
    _, version = runner.text(['xcodebuild', '-version'], cwd=cwd, label='xcode-version')
    _, sdk = runner.text(['xcrun', '--sdk', 'iphonesimulator', '--show-sdk-version'], cwd=cwd, label='sdk-version')
    _, sdkroot = runner.text(['xcrun', '--sdk', 'iphonesimulator', '--show-sdk-path'], cwd=cwd, label='sdk-root')
    if not re.search(r'^Xcode ' + re.escape(expected['xcode']) + r'$', version, re.M) or sdk.strip() != expected['sdk']:
        raise ValueError('Actual Xcode/SDK differs from reviewed matrix')
    _, developer = runner.text(['xcode-select', '-p'], cwd=cwd, label='developer-directory')
    developer = runner.env.get('DEVELOPER_DIR') or developer.strip()
    if not Path(developer).is_dir():
        raise ValueError('Missing actual developer directory')
    _, raw_runtimes = runner.text(['xcrun', 'simctl', 'list', '-j', 'runtimes'], cwd=cwd, label='runtimes')
    _, raw_devices = runner.text(['xcrun', 'simctl', 'list', '-j', 'devices', 'available'], cwd=cwd, label='devices')
    devices = {'phone': exact_device(json.loads(raw_devices), json.loads(raw_runtimes), expected['runtime'], 'iPhone 17')}
    if toolchain == '27':
        devices['ipad'] = exact_device(json.loads(raw_devices), json.loads(raw_runtimes), expected['runtime'], 'iPad Pro 13-inch (M5)')
    package = Path(runner.env['APPIUM_HOME']) / 'node_modules/appium-xcuitest-driver/package.json'
    script = "const{createRequire}=require('module');const p=require(process.argv[1]);const r=createRequire(process.argv[1]);const w=r.resolve('appium-webdriveragent/package.json');console.log(JSON.stringify({driver:p.version,wda:r('appium-webdriveragent/package.json').version,wdaPackage:w}));"
    _, packages = runner.text(['node', '-e', script, str(package)], cwd=cwd, label='driver-packages')
    packages = json.loads(packages)
    if any(packages[k] != expected[k] for k in ('driver', 'wda')):
        raise ValueError('Actual isolated driver/WDA differs from reviewed matrix')
    return {**expected, 'sdkroot': sdkroot.strip(), 'developerDir': developer, 'devices': devices,
            'wdaPackage': packages['wdaPackage'], 'driverPackage': file_record(package),
            'wdaPackageRecord': file_record(packages['wdaPackage'])}


def prepare_source(repository, source, harness_source, root, dependency_root, runner):
    destination = Path(root) / 'source' / source
    archive_source(repository, source, destination)
    mobile, harness = destination / 'mobileapp', Path(harness_source) / 'mobileapp'
    for name in ('package.json', 'package-lock.json', 'Gemfile.lock', 'ios/Podfile.lock', '.bundle/config'):
        if (mobile / name).read_bytes() != (harness / name).read_bytes():
            raise ValueError(f'{source}: committed dependency input differs: {name}')
    for name in ('node_modules', 'vendor'):
        target = Path(dependency_root) / name
        if not target.is_dir():
            raise ValueError('Missing verified job dependency: ' + str(target))
        (mobile / name).symlink_to(target, target_is_directory=True)
    observed = ('src/state/AppContext.tsx', 'src/screens/ServersScreen.tsx', 'src/push/index.ts', 'metro.config.js')
    before = {name: (mobile / name).read_text() for name in observed}
    runner.command(['python3', str(harness / 'e2e/instrument-baseline.py'), str(destination)],
                   cwd=destination, label='instrument-source', timeout=30)
    with (mobile / 'metro.config.js').open('a') as stream:
        stream.write("\nmodule.exports.watchFolders = [...(module.exports.watchFolders || []), process.env.GTMUX_COMPARE_NODE_MODULES];\nmodule.exports.resolver.nodeModulesPaths = [...(module.exports.resolver.nodeModulesPaths || []), process.env.GTMUX_COMPARE_NODE_MODULES];\n")
    edits = []
    for name, original in before.items():
        final = (mobile / name).read_text()
        edits.append({'path': name, 'beforeSHA': hashlib.sha256(original.encode()).hexdigest(),
                      'afterSHA': hashlib.sha256(final.encode()).hexdigest(),
                      'patch': ''.join(difflib.unified_diff(original.splitlines(True), final.splitlines(True), fromfile=name, tofile=name))})
    write_once(destination / 'instrumentation.json', {'source': source, 'edits': edits})
    return mobile


def prepare_work(harness_source, destination, dependencies):
    """One immutable harness source, independent writable state per journey."""
    source = Path(harness_source) / 'mobileapp'
    target = Path(destination) / 'mobileapp'
    ignore = shutil.ignore_patterns('node_modules', 'vendor', 'ios', 'android', '.e2e-artifacts', '.git', '.gradle', 'build', 'coverage')
    shutil.copytree(source, target, ignore=ignore, symlinks=True)
    manifest = []
    for path in sorted(target.rglob('*')):
        if path.is_symlink():
            raise ValueError('Unexpected link in immutable test harness')
        if path.is_file():
            relative = path.relative_to(target)
            if path.read_bytes() != (source / relative).read_bytes():
                raise ValueError('Harness bytes changed in copy')
            manifest.append(file_record(path, target))
    (target / 'node_modules').symlink_to(Path(dependencies) / 'node_modules', target_is_directory=True)
    write_once(Path(destination) / 'harness.json', manifest)
    return target


def build_product(runner, mobile, source, root):
    derived = Path(root) / 'build' / source / 'adhoc'
    runner.command(['bundle', 'exec', 'pod', 'install'], cwd=mobile / 'ios', label='pods-' + source[:8], timeout=600)
    args = ['xcodebuild', '-workspace', 'ios/GtmuxMobile.xcworkspace', '-scheme', 'GtmuxMobile',
            '-configuration', 'Release', '-sdk', 'iphonesimulator', '-destination', 'generic/platform=iOS Simulator',
            '-jobs', '2', '-derivedDataPath', str(derived), 'ARCHS=arm64', 'ONLY_ACTIVE_ARCH=NO',
            'CODE_SIGNING_ALLOWED=YES', 'CODE_SIGN_IDENTITY=-', 'build']
    _, log = runner.command(args, cwd=mobile, label='build-' + source[:8], timeout=1200)
    app = derived / 'Build/Products/Release-iphonesimulator/gtmux.app'
    bundles = [('app', app)] + [('liveActivity' if p.name == 'GtmuxWidget.appex' else 'notificationExtension' if p.name == 'GtmuxNotificationService.appex' else 'unknown', p) for p in sorted((app / 'PlugIns').glob('*.appex'))]
    products = []
    for kind, bundle in bundles:
        info = plistlib.loads((bundle / 'Info.plist').read_bytes()); binary = bundle / info['CFBundleExecutable']
        _, arch = runner.text(['lipo', '-archs', str(binary)], cwd=mobile, label=kind + '-arch')
        _, uuid = runner.text(['dwarfdump', '--uuid', str(binary)], cwd=mobile, label=kind + '-uuid')
        _, signature = runner.text(['codesign', '-dv', str(bundle)], cwd=mobile, label=kind + '-signature')
        _, entitlements = runner.text(['codesign', '-d', '--entitlements', ':-', str(bundle)], cwd=mobile, label=kind + '-entitlements')
        products.append({**file_record(binary), 'kind': kind, 'architectures': arch.strip().split(),
                         'uuid': uuid.strip(), 'deployment': info.get('MinimumOSVersion'),
                         'bundleID': info.get('CFBundleIdentifier'), 'sdk': info.get('DTSDKName'),
                         'signature': 'adhoc' if 'Signature=adhoc' in signature else 'unexpected',
                         'entitlements': entitlements, 'info': file_record(bundle / 'Info.plist')})
    build_text = log.read_text(errors='replace')
    sysroots = re.findall(r'-isysroot\s+(\S+)', build_text)
    if not sysroots:
        raise ValueError('No actual compiler sysroot in build log')
    return {'app': str(app), 'products': products, 'linkSysroot': sysroots[0], 'buildLog': file_record(log),
            'resolvedPods': file_record(mobile / 'ios/Podfile.lock')}


def parse_jest(path):
    data = json.loads(Path(path).read_text())
    cases = []
    for suite in data.get('testResults', []):
        for item in suite.get('assertionResults', []):
            cases.append({'name': item['fullName'], 'status': item['status'],
                          'startedAt': suite.get('startTime'), 'completedAt': suite.get('endTime'),
                          'failureMessages': item.get('failureMessages', [])})
    return cases


def collect_identity(runner, unit, evidence, device, product, preflight_data, installed_at, launched_at, derived):
    identity = {k: preflight_data[k] for k in ('xcode', 'sdk', 'runtime', 'driver', 'wda', 'sdkroot', 'developerDir')}
    identity.update({k: device[k] for k in ('runtimeID', 'runtimeBuild', 'udid')})
    identity.update({'source': unit['source'], 'deviceType': device['deviceTypeIdentifier'],
                     'signature': 'adhoc', 'products': product['products'], 'linkSysroot': product['linkSysroot'],
                     'installedAt': installed_at, 'journeyStartedAt': launched_at})
    session = json.loads((Path(evidence) / 'session/session.json').read_text())
    identity['sessionID'] = session['sessionID']
    capabilities = session['capabilities']
    if capabilities.get('udid', capabilities.get('appium:udid')) != device['udid']:
        raise ValueError('Actual WDA session is not attached to the selected device')
    _, launchctl = runner.text(['xcrun', 'simctl', 'spawn', device['udid'], 'launchctl', 'list'], cwd=evidence, label='process-identity')
    pids = [int(line.split()[0]) for line in launchctl.splitlines() if 'UIKitApplication:com.gtmux.app[' in line and line.split()[0].isdigit()]
    if len(pids) != 1:
        raise ValueError('Installed application PID is missing or ambiguous')
    identity['pid'] = pids[0]
    _, launched = runner.text(['ps', '-p', str(pids[0]), '-o', 'lstart='], cwd=evidence, label='actual-process-launch')
    if not launched.strip(): raise ValueError('Actual process launch time missing')
    identity['launchedAt'] = launched.strip()
    wdas = list(Path(derived).glob('Build/Products/*/WebDriverAgentRunner-Runner.app'))
    if len(wdas) != 1:
        raise ValueError('Actual built WDA runner is missing or ambiguous')
    info = plistlib.loads((wdas[0] / 'Info.plist').read_bytes())
    bundle = wdas[0] / 'PlugIns/WebDriverAgentRunner.xctest'
    test_info = plistlib.loads((bundle / 'Info.plist').read_bytes())
    binary = bundle / test_info['CFBundleExecutable']
    identity['wdaBinarySHA'] = file_record(binary)['sha256']
    identity['wdaDeployment'] = info.get('MinimumOSVersion')
    project = Path(preflight_data['wdaPackage']).parent / 'WebDriverAgent.xcodeproj'
    _, settings = runner.text(['xcodebuild', '-project', str(project), '-scheme', 'WebDriverAgentRunner', '-sdk', 'iphonesimulator',
                    '-showBuildSettings', '-json'], cwd=evidence, label='wda-build-settings', timeout=60)
    targets = json.loads(settings)
    actual = [t['buildSettings']['IPHONEOS_DEPLOYMENT_TARGET'] for t in targets if t.get('target') == 'WebDriverAgentRunner']
    if len(actual) != 1 or not actual[0]: raise ValueError('Actual WDA target deployment is missing')
    identity['wdaTargetDeployment'] = actual[0]
    return identity


TEST_FILES = {'servers': 'server-switch-compare.test.ts', 'coldstart': 'coldstart-push.test.ts',
              'actions': 'scene-migration-actions.test.ts', 'links': 'scene-migration-links.test.ts', 'ipad': 'ipad-adaptive.test.ts'}


def cleanup_unit(runner, evidence, derived, udid):
    """End only processes bound to this unit's PID file/derived-data path."""
    problems = []
    try:
        _, processes = runner.text(['ps', '-axo', 'pid=,command='], cwd=evidence, label='owned-processes')
        rows = {}
        for row in processes.splitlines():
            parts = row.strip().split(None, 1)
            if len(parts) == 2 and parts[0].isdigit(): rows[int(parts[0])] = parts[1]
        pid_file = Path(evidence) / 'session/server.pid'
        if pid_file.exists():
            pid = int(pid_file.read_text())
            command = rows.get(pid, '')
            if command:
                if 'appium' not in command or not re.search(r'--port(?:=|\s+)4723(?:\s|$)', command):
                    raise ValueError('Saved server PID no longer identifies this unit Appium')
                try: os.killpg(pid, signal.SIGKILL)
                except ProcessLookupError: pass
        marker = '-derivedDataPath ' + str(derived)
        for pid, command in rows.items():
            if marker + ' ' in command or command.endswith(marker):
                if 'xcodebuild' not in command: raise ValueError('Unexpected command using unit WDA path')
                try: os.kill(pid, signal.SIGKILL)
                except ProcessLookupError: pass
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError) as error:
        problems.append('process cleanup: ' + str(error))
    try: runner.command(['xcrun', 'simctl', 'shutdown', udid], cwd=evidence, label='shutdown-after', timeout=30, check=False)
    except (OSError, RuntimeError, subprocess.SubprocessError) as error: problems.append('device cleanup: ' + str(error))
    return problems


def run_unit(unit, paths, harness_source, dependencies, product, preflight_data, env):
    evidence = paths['evidence']; command_root = evidence / 'commands'; command_root.mkdir()
    local_env = dict(env)
    device = preflight_data['devices']['ipad' if unit['journey'] == 'ipad' else 'phone']
    derived = paths['work'] / 'wda'
    local_env.update({'GTMUX_E2E_UDID': device['udid'], 'GTMUX_E2E_OS': unit['runtime'],
                     'GTMUX_E2E_DEVICE': device['name'], 'GTMUX_E2E_SESSION_CI': '1',
                     'GTMUX_E2E_HEADLESS': '1', 'GTMUX_E2E_SOFT_KEYBOARD': '1', 'GTMUX_E2E_ACCEPT_ALERTS': '1',
                     'GTMUX_E2E_WDA_DERIVED': str(derived), 'GTMUX_E2E_APPIUM_PORT': '4723',
                     'GTMUX_SCENE_EVIDENCE_ROOT': str(evidence), 'GTMUX_E2E_ARTIFACTS_DIR': str(evidence / 'session')})
    if unit['journey'] == 'servers':
        local_env['GTMUX_COMPARE_MANUAL_LAUNCH'] = '1'
    else:
        local_env.pop('GTMUX_COMPARE_MANUAL_LAUNCH', None)
    runner = Runner(command_root, local_env)
    result = {'unit': unit, 'state': 'not-started', 'testExit': None, 'cases': [], 'steps': [], 'identity': {}, 'artifacts': []}
    problems = []
    try:
        work = prepare_work(harness_source, paths['work'], dependencies)
        write_once(evidence / 'harness.json', json.loads((paths['work'] / 'harness.json').read_text()))
        runner.command(['xcrun', 'simctl', 'shutdown', device['udid']], cwd=work, label='shutdown-before', timeout=30, check=False)
        runner.command(['xcrun', 'simctl', 'erase', device['udid']], cwd=work, label='erase', timeout=60)
        runner.command(['xcrun', 'simctl', 'bootstatus', device['udid'], '-b'], cwd=work, label='boot', timeout=180)
        runner.command(['xcrun', 'simctl', 'install', device['udid'], product['app']], cwd=work, label='install', timeout=60)
        installed = now(); launched = now()
        status, _ = runner.command(['npm', 'run', 'test:e2e', '--', '--runInBand', TEST_FILES[unit['journey']],
                                    '--json', '--outputFile', str(evidence / 'jest.json')],
                                   cwd=work, label='journey', timeout=2100, check=False)
        result['testExit'] = status; result['state'] = 'completed'
        # Post-test evidence is isolated from the original test status. Any absent
        # evidence still makes acceptance red; never replace a test's exit7 with0.
        for label, collect in [
            ('cases', lambda: parse_jest(evidence / 'jest.json')),
            ('steps', lambda: [json.loads(s)['step'] for s in (evidence / 'session/behavior-steps.jsonl').read_text().splitlines()]),
            ('identity', lambda: collect_identity(runner, unit, evidence, device, product, preflight_data, installed, launched, derived)),
        ]:
            try: result[label] = collect()
            except (OSError, ValueError, KeyError, RuntimeError) as error: problems.append(f'{label}: {error}')
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError) as error:
        problems.append(str(error))
    finally:
        problems.extend(cleanup_unit(runner, evidence, derived, device['udid']))
    result['problems'] = problems
    result['artifacts'] = [file_record(p, evidence) for p in sorted(evidence.rglob('*')) if p.is_file() and not p.is_symlink()]
    write_once(evidence / 'result.json', result)
    return result


def execute(toolchain, repository, env):
    if env.get('CI') != 'true' or env.get('RUNNER_OS') != 'macOS' or env.get('RUNNER_ARCH') != 'ARM64':
        raise ValueError('Execution is restricted to the reviewed ARM64 macOS CI runner')
    event = json.loads(Path(env['GITHUB_EVENT_PATH']).read_text())
    ids = sources(env['GITHUB_EVENT_NAME'], event, env['GITHUB_SHA'])
    root = create_job_root(env['RUNNER_TEMP'], env['GITHUB_RUN_ID'], env['GITHUB_RUN_ATTEMPT'], env['GITHUB_JOB'])
    logs = root / 'commands'; logs.mkdir(); runner = Runner(logs, env)
    results, errors = [], []
    expected = units(ids, toolchain)
    try:
        data = preflight(runner, toolchain, repository)
        write_once(root / 'preflight.json', data)
        harness = archive_source(repository, ids['candidate'], root / 'harness')
        checkout_mobile = Path(repository) / 'mobileapp'
        for name in ('package-lock.json', 'package.json', 'Gemfile.lock'):
            if (checkout_mobile / name).read_bytes() != (harness / 'mobileapp' / name).read_bytes():
                raise ValueError('Runner dependencies are not from the exact harness source')
        lock = file_record(harness / 'mobileapp/package-lock.json')['sha256']
        dependencies = root / 'deps' / lock; dependencies.mkdir(parents=True)
        for name in ('node_modules', 'vendor'):
            if not (checkout_mobile / name).is_dir(): raise ValueError('Missing preinstalled job dependency ' + name)
            (dependencies / name).symlink_to((checkout_mobile / name).resolve(), target_is_directory=True)
        runner.env.update({'GTMUX_COMPARE_NODE_MODULES': str(dependencies / 'node_modules'), 'GTMUX_COMPARE_JOURNEY': 'push'})
        products = {}
        for sha in dict.fromkeys([ids['candidate'], *ids.values()]):
            mobile = prepare_source(repository, sha, harness, root, dependencies, runner)
            product = build_product(runner, mobile, sha, root)
            products[sha] = product
            write_once(root / 'source' / sha / 'product.json', product)
        if len({p['resolvedPods']['sha256'] for p in products.values()}) != 1:
            raise ValueError('Resolved Pods differ between compared sources')
        for unit in expected:
            result = run_unit(unit, allocate_unit(root, unit), harness, dependencies, products[unit['source']], data, runner.env)
            results.append(result)
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError) as error:
        errors.append(str(error))
    payload = {'eventName': env['GITHUB_EVENT_NAME'], 'event': event, 'githubSHA': env['GITHUB_SHA'],
               'toolchain': toolchain, 'results': results, 'preparationErrors': errors}
    write_once(root / 'acceptance.json', payload)
    errors.extend(verify_all(expected, results, root / 'evidence'))
    write_once(root / 'verdict.json', {'errors': errors, 'passed': not errors})
    for error in errors: print('scene acceptance failed:', error)
    print('Scene evidence:', root)
    return int(bool(errors))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--verify', type=Path)
    parser.add_argument('--execute', choices=tuple(TOOLS))
    parser.add_argument('--repository', type=Path, default=Path.cwd())
    parser.add_argument('--evidence-root', type=Path)
    args = parser.parse_args()
    if args.execute:
        if args.verify or args.evidence_root: parser.error('Execution and verification are separate')
        return execute(args.execute, args.repository.resolve(), os.environ)
    if not args.verify or not args.evidence_root: parser.error('Verification requires input and evidence root')
    try:
        data = json.loads(args.verify.read_text())
        identities = sources(data['eventName'], data['event'], data['githubSHA'])
        expected = units(identities, data['toolchain'])
        errors = list(data.get('preparationErrors', []))
        errors.extend(verify_all(expected, data['results'], args.evidence_root))
    except (OSError, ValueError, KeyError, TypeError) as error:
        errors = [str(error)]
    for error in errors:
        print('scene acceptance failed:', error)
    return int(bool(errors))


if __name__ == '__main__':
    raise SystemExit(main())
